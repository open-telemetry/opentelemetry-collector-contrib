// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

const (
	readmeURL            = "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/mongodbreceiver/README.md"
	removeDatabaseAttrID = "receiver.mongodb.removeDatabaseAttr"
)

var (
	unknownVersion = func() *version.Version { return version.Must(version.NewVersion("0.0")) }

	removeDatabaseAttrFeatureGate = featuregate.GlobalRegistry().MustRegister(
		removeDatabaseAttrID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("Remove duplicate database name attribute"),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24972"),
		featuregate.WithRegisterFromVersion("v0.90.0"))
)

type mongodbScraper struct {
	logger       *zap.Logger
	config       *Config
	client       client
	mongoVersion *version.Version
	mb           *metadata.MetricsBuilder

	// removeDatabaseAttr if enabled, will remove database attribute on database metrics
	removeDatabaseAttr bool
}

func newMongodbScraper(settings receiver.CreateSettings, config *Config) *mongodbScraper {
	ms := &mongodbScraper{
		logger:             settings.Logger,
		config:             config,
		mb:                 metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		mongoVersion:       unknownVersion(),
		removeDatabaseAttr: removeDatabaseAttrFeatureGate.IsEnabled(),
	}

	if !ms.removeDatabaseAttr {
		settings.Logger.Warn(
			fmt.Sprintf("Feature gate %s is not enabled. Please see the README for more information: %s", removeDatabaseAttrID, readmeURL),
		)
	}

	return ms
}

func (s *mongodbScraper) start(ctx context.Context, _ component.Host) error {
	c, err := newClient(ctx, s.config, s.logger)
	if err != nil {
		return fmt.Errorf("create mongo client: %w", err)
	}
	s.client = c
	return nil
}

func (s *mongodbScraper) shutdown(ctx context.Context) error {
	if s.client != nil {
		return s.client.Disconnect(ctx)
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

	now := pcommon.NewTimestampFromTime(time.Now())

	s.mb.RecordMongodbDatabaseCountDataPoint(now, int64(len(dbNames)))
	s.collectAdminDatabase(ctx, now, errs)
	s.collectTopStats(ctx, now, errs)

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

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(databaseName)
	s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (s *mongodbScraper) collectAdminDatabase(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	serverStatus, err := s.client.ServerStatus(ctx, "admin")
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch admin server status metrics: %w", err))
		return
	}
	s.recordAdminStats(now, serverStatus, errs)
	s.mb.EmitForResource()
}

func (s *mongodbScraper) collectTopStats(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	topStats, err := s.client.TopStats(ctx)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch top stats metrics: %w", err))
		return
	}
	s.recordOperationTime(now, topStats, errs)
	s.mb.EmitForResource()
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

	if s.removeDatabaseAttr {
		rb := s.mb.NewResourceBuilder()
		rb.SetDatabase(databaseName)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	} else {
		s.mb.EmitForResource()
	}
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
}

func (s *mongodbScraper) recordIndexStats(now pcommon.Timestamp, indexStats []bson.M, databaseName string, collectionName string, errs *scrapererror.ScrapeErrors) {
	s.recordIndexAccess(now, indexStats, databaseName, collectionName, errs)
}
