// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

var (
	unknownVersion = func() *version.Version { return version.Must(version.NewVersion("0.0")) }

	_ = featuregate.GlobalRegistry().MustRegister(
		"receiver.mongodb.removeDatabaseAttr",
		featuregate.StageStable,
		featuregate.WithRegisterDescription("Remove duplicate database name attribute"),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24972"),
		featuregate.WithRegisterFromVersion("v0.90.0"),
		featuregate.WithRegisterToVersion("v0.104.0"))
)

type mongodbScraper struct {
	logger             *zap.Logger
	config             *Config
	client             client
	secondaryClients   []client
	mongoVersion       *version.Version
	mb                 *metadata.MetricsBuilder
	prevReplTimestamp  pcommon.Timestamp
	prevReplCounts     map[string]int64
	prevTimestamp      pcommon.Timestamp
	prevFlushTimestamp pcommon.Timestamp
	prevCounts         map[string]int64
	prevFlushCount     int64
}

func newMongodbScraper(settings receiver.Settings, config *Config) *mongodbScraper {
	return &mongodbScraper{
		logger:             settings.Logger,
		config:             config,
		mb:                 metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		mongoVersion:       unknownVersion(),
		prevReplTimestamp:  pcommon.Timestamp(0),
		prevReplCounts:     make(map[string]int64),
		prevTimestamp:      pcommon.Timestamp(0),
		prevFlushTimestamp: pcommon.Timestamp(0),
		prevCounts:         make(map[string]int64),
		prevFlushCount:     0,
	}
}

func (s *mongodbScraper) start(ctx context.Context, _ component.Host) error {
	c, err := newClient(ctx, s.config, s.logger, false)
	if err != nil {
		return fmt.Errorf("create mongo client: %w", err)
	}
	s.client = c

	// Skip secondary host discovery if direct connection is enabled
	if s.config.DirectConnection {
		return nil
	}

	secondaries, err := s.findSecondaryHosts(ctx)
	if err != nil {
		s.logger.Warn("failed to find secondary hosts", zap.Error(err))
		return nil
	}

	for _, secondary := range secondaries {
		secondaryConfig := *s.config
		secondaryConfig.Hosts = []confignet.TCPAddrConfig{
			{
				Endpoint: secondary,
			},
		}

		client, err := newClient(ctx, &secondaryConfig, s.logger, true)
		if err != nil {
			s.logger.Warn("failed to connect to secondary", zap.String("host", secondary), zap.Error(err))
			continue
		}
		s.secondaryClients = append(s.secondaryClients, client)
	}

	return nil
}

func (s *mongodbScraper) shutdown(ctx context.Context) error {
	var errs []error

	if s.client != nil {
		if err := s.client.Disconnect(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	for _, client := range s.secondaryClients {
		if err := client.Disconnect(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple disconnect errors: %v", errs)
	}
	return nil
}

func (s *mongodbScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if s.client == nil {
		return pmetric.NewMetrics(), errors.New("no client was initialized before calling scrape")
	}

	if s.mongoVersion.Equal(unknownVersion()) {
		version, err := s.client.GetVersion(ctx)
		if err == nil {
			s.mongoVersion = version
		} else {
			s.logger.Warn("determine mongo version", zap.Error(err))
		}
	}

	errs := &scrapererror.ScrapeErrors{}
	s.collectMetrics(ctx, errs)
	return s.mb.Emit(), errs.Combine()
}

func (s *mongodbScraper) collectMetrics(ctx context.Context, errs *scrapererror.ScrapeErrors) {
	dbNames, err := s.client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch database names: %w", err))
		return
	}

	serverStatus, sErr := s.client.ServerStatus(ctx, "admin")
	if sErr != nil {
		errs.Add(fmt.Errorf("failed to fetch server status: %w", sErr))
		return
	}
	serverAddress, serverPort, aErr := serverAddressAndPort(serverStatus)
	if aErr != nil {
		errs.Add(fmt.Errorf("failed to fetch server address and port: %w", aErr))
		return
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	s.mb.RecordMongodbDatabaseCountDataPoint(now, int64(len(dbNames)))
	s.recordAdminStats(now, serverStatus, errs)
	s.collectTopStats(ctx, now, errs)

	rb := s.mb.NewResourceBuilder()
	rb.SetServerAddress(serverAddress)
	rb.SetServerPort(serverPort)
	s.mb.EmitForResource(metadata.WithResource(rb.Emit()))

	// Collect metrics for each database
	for _, dbName := range dbNames {
		s.collectDatabase(ctx, now, dbName, errs)
		collectionNames, err := s.client.ListCollectionNames(ctx, dbName)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf("failed to fetch collection names: %w", err))
			return
		}

		for _, collectionName := range collectionNames {
			s.collectIndexStats(ctx, now, dbName, collectionName, errs)
		}

		rb.SetServerAddress(serverAddress)
		rb.SetServerPort(serverPort)
		rb.SetDatabase(dbName)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}

func (s *mongodbScraper) collectDatabase(ctx context.Context, now pcommon.Timestamp, databaseName string, errs *scrapererror.ScrapeErrors) {
	dbStats, err := s.client.DBStats(ctx, databaseName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch database stats metrics: %w", err))
	} else {
		s.recordDBStats(now, dbStats, databaseName, errs)
	}

	serverStatus, err := s.client.ServerStatus(ctx, databaseName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch server status metrics: %w", err))
		return
	}
	s.recordNormalServerStats(now, serverStatus, databaseName, errs)
}

func (s *mongodbScraper) collectTopStats(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	topStats, err := s.client.TopStats(ctx)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch top stats metrics: %w", err))
		return
	}
	s.recordOperationTime(now, topStats, errs)
}

func (s *mongodbScraper) collectIndexStats(ctx context.Context, now pcommon.Timestamp, databaseName string, collectionName string, errs *scrapererror.ScrapeErrors) {
	if databaseName == "local" {
		return
	}
	indexStats, err := s.client.IndexStats(ctx, databaseName, collectionName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch index stats metrics: %w", err))
		return
	}
	s.recordIndexStats(now, indexStats, databaseName, collectionName, errs)
}

func (s *mongodbScraper) recordDBStats(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	s.recordCollections(now, doc, dbName, errs)
	s.recordDataSize(now, doc, dbName, errs)
	s.recordExtentCount(now, doc, dbName, errs)
	s.recordIndexSize(now, doc, dbName, errs)
	s.recordIndexCount(now, doc, dbName, errs)
	s.recordObjectCount(now, doc, dbName, errs)
	s.recordStorageSize(now, doc, dbName, errs)
}

func (s *mongodbScraper) recordNormalServerStats(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	s.recordConnections(now, doc, dbName, errs)
	s.recordDocumentOperations(now, doc, dbName, errs)
	s.recordMemoryUsage(now, doc, dbName, errs)
	s.recordLockAcquireCounts(now, doc, dbName, errs)
	s.recordLockAcquireWaitCounts(now, doc, dbName, errs)
	s.recordLockTimeAcquiringMicros(now, doc, dbName, errs)
	s.recordLockDeadlockCount(now, doc, dbName, errs)
}

func (s *mongodbScraper) recordAdminStats(now pcommon.Timestamp, document bson.M, errs *scrapererror.ScrapeErrors) {
	s.recordCacheOperations(now, document, errs)
	s.recordCursorCount(now, document, errs)
	s.recordCursorTimeoutCount(now, document, errs)
	s.recordGlobalLockTime(now, document, errs)
	s.recordNetworkCount(now, document, errs)
	s.recordOperations(now, document, errs)
	s.recordOperationsRepl(now, document, errs)
	s.recordSessionCount(now, document, errs)
	s.recordLatencyTime(now, document, errs)
	s.recordUptime(now, document, errs)
	s.recordHealth(now, document, errs)
	s.recordActiveWrites(now, document, errs)
	s.recordActiveReads(now, document, errs)
	s.recordFlushesPerSecond(now, document, errs)
	s.recordWTCacheBytes(now, document, errs)
	s.recordPageFaults(now, document, errs)
}

func (s *mongodbScraper) recordIndexStats(now pcommon.Timestamp, indexStats []bson.M, databaseName string, collectionName string, errs *scrapererror.ScrapeErrors) {
	s.recordIndexAccess(now, indexStats, databaseName, collectionName, errs)
}

func serverAddressAndPort(serverStatus bson.M) (string, int64, error) {
	host, ok := serverStatus["host"].(string)
	if !ok {
		return "", 0, errors.New("host field not found in server status")
	}
	hostParts := strings.Split(host, ":")
	switch len(hostParts) {
	case 1:
		return hostParts[0], defaultMongoDBPort, nil
	case 2:
		port, err := strconv.ParseInt(hostParts[1], 10, 64)
		if err != nil {
			return "", 0, fmt.Errorf("failed to parse port: %w", err)
		}
		return hostParts[0], port, nil
	default:
		return "", 0, fmt.Errorf("unexpected host format: %s", host)
	}
}

func (s *mongodbScraper) findSecondaryHosts(ctx context.Context) ([]string, error) {
	result, err := s.client.RunCommand(ctx, "admin", bson.M{"replSetGetStatus": 1})
	if err != nil {
		s.logger.Error("Failed to get replica set status", zap.Error(err))
		return nil, fmt.Errorf("failed to get replica set status: %w", err)
	}

	members, ok := result["members"].(bson.A)
	if !ok {
		return nil, fmt.Errorf("invalid members format: expected type primitive.A but got %T, value: %v", result["members"], result["members"])
	}

	var hosts []string
	for _, member := range members {
		m, ok := member.(bson.M)
		if !ok {
			continue
		}

		state, ok := m["stateStr"].(string)
		if !ok {
			continue
		}

		name, ok := m["name"].(string)
		if !ok {
			continue
		}

		// Only add actual secondaries, not arbiters or other states
		if state == "SECONDARY" {
			s.logger.Debug("Found secondary",
				zap.String("host", name),
				zap.String("state", state))
			hosts = append(hosts, name)
		}
	}

	return hosts, nil
}
