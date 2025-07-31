// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

var (
	unknownVersion              = func() *version.Version { return version.Must(version.NewVersion("0.0")) }
	unexplainablePipelineStages = map[string]bool{
		"$collStats":               true,
		"$currentOp":               true,
		"$indexStats":              true,
		"$listSearchIndexes":       true,
		"$sample":                  true,
		"$shardedDataDistribution": true,
		"$mergeCursors":            true,
	}
	unexplainableCommands = map[string]bool{
		"getMore":         true,
		"insert":          true,
		"update":          true,
		"delete":          true,
		"explain":         true,
		"profile":         true,
		"listCollections": true,
		"listDatabases":   true,
		"dbStats":         true,
		"createIndexes":   true,
		"shardCollection": true,
	}
	_ = featuregate.GlobalRegistry().MustRegister(
		"receiver.mongodb.removeDatabaseAttr",
		featuregate.StageStable,
		featuregate.WithRegisterDescription("Remove duplicate database name attribute"),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24972"),
		featuregate.WithRegisterFromVersion("v0.90.0"),
		featuregate.WithRegisterToVersion("v0.104.0"))
)

const (
	namespaceKey       = "ns"
	commandKey         = "command"
	opKey              = "op"
	durationMicrosKey  = "microsecs_running"
	clientKey          = "client"
	applicationNameKey = "appName"
)

type mongodbScraper struct {
	logger             *zap.Logger
	config             *Config
	client             client
	secondaryClients   []client
	mongoVersion       *version.Version
	mb                 *metadata.MetricsBuilder
	lb                 *metadata.LogsBuilder
	cache              *lru.Cache[string, int64]
	prevReplTimestamp  pcommon.Timestamp
	prevReplCounts     map[string]int64
	prevTimestamp      pcommon.Timestamp
	prevFlushTimestamp pcommon.Timestamp
	prevCounts         map[string]int64
	prevFlushCount     int64
	obfuscator         *obfuscator
}

func newMongodbScraper(settings receiver.Settings, config *Config) *mongodbScraper {
	cache, _ := lru.New[string, int64](1024)
	return &mongodbScraper{
		logger:             settings.Logger,
		config:             config,
		mb:                 metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		lb:                 metadata.NewLogsBuilder(metadata.DefaultLogsBuilderConfig(), settings),
		cache:              cache,
		mongoVersion:       unknownVersion(),
		prevReplTimestamp:  pcommon.Timestamp(0),
		prevReplCounts:     make(map[string]int64),
		prevTimestamp:      pcommon.Timestamp(0),
		prevFlushTimestamp: pcommon.Timestamp(0),
		prevCounts:         make(map[string]int64),
		prevFlushCount:     0,
		obfuscator:         newObfuscator(),
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

func (s *mongodbScraper) scrapeLogs(ctx context.Context) (plog.Logs, error) {
	operations, err := s.client.CurrentOp(ctx)
	if err != nil {
		s.logger.Error("Failed to get current operations", zap.Error(err))
		return plog.NewLogs(), err
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	s.processCurrentOp(ctx, operations, now)

	serverStatus, err := s.client.ServerStatus(ctx, "admin")
	if err != nil {
		s.logger.Debug("Failed to get server status for logs", zap.Error(err))
		return s.lb.Emit(), nil
	}

	serverAddress, serverPort, err := serverAddressAndPort(serverStatus)
	if err != nil {
		s.logger.Debug("Failed to extract server address and port for logs", zap.Error(err))
		return s.lb.Emit(), nil
	}

	rb := s.lb.NewResourceBuilder()
	rb.SetServerAddress(serverAddress)
	rb.SetServerPort(serverPort)

	return s.lb.Emit(metadata.WithLogsResource(rb.Emit())), nil
}

func (s *mongodbScraper) processCurrentOp(ctx context.Context, operations []bson.M, now pcommon.Timestamp) {
	queryCount := 0

	for _, op := range operations {
		// Filter out irrelevant operations (e.g., internal operations, operations without commands)
		if !s.shouldIncludeOperation(op) {
			continue
		}

		namespace := getValue[string](op, namespaceKey)
		command := getValue[bson.D](op, commandKey)
		opType := getValue[string](op, opKey)
		commandType := command[0].Key
		durationMicros := float64(getValue[int64](op, durationMicrosKey)) / 1_000_000.0
		client := getValue[string](op, clientKey)
		applicationName := getValue[string](op, applicationNameKey)
		port := 0
		server := ""
		if client != "" {
			clientSplit := strings.Split(client, ":")
			server = clientSplit[0]
			if len(clientSplit) > 1 {
				if p, err := strconv.Atoi(clientSplit[1]); err == nil {
					port = p
				}
			}
		}
		cleanedCommand := cleanCommand(command)
		obfuscatedStatement := s.obfuscator.obfuscateSQLString(cleanedCommand.String())
		querySignature := generateQuerySignature(obfuscatedStatement)

		// Get explain plan for supported operations
		explainPlan := ""
		collectionName := s.getCollectionFromNamespace(namespace)
		if collectionName != "" && len(command) > 0 && s.shouldExplainOperation(opType, cleanedCommand) {
			plan, err := s.getExplainPlan(ctx, cleanedCommand)
			if err != nil {
				s.logger.Debug("Failed to get explain plan",
					zap.String("namespace", namespace),
					zap.String("op", opType),
					zap.String("appName", applicationName),
					zap.Error(err))
			} else {
				explainPlan = plan
			}
		}

		s.logger.Debug("Processing MongoDB operation",
			zap.String("namespace", namespace),
			zap.String("commandType", commandType),
			zap.Float64("duration_micros", durationMicros),
			zap.String("client", client),
			zap.String("appName", applicationName),
			zap.String("query_signature", querySignature),
			zap.Bool("has_explain_plan", explainPlan != ""))

		s.lb.RecordDbServerQuerySampleEvent(
			ctx,
			now,
			server,
			int64(port),
			metadata.AttributeDbSystemNameMongodb,
			collectionName,
			commandType,
			obfuscatedStatement,
			applicationName,
			querySignature,
			durationMicros,
			explainPlan,
		)

		queryCount++
	}

	s.logger.Debug("Processed MongoDB current operations", zap.Int("total_operations", len(operations)), zap.Int("processed_queries", queryCount))
}

// shouldExplainOperation determines if we should try to get an explain plan for this operation
func (*mongodbScraper) shouldExplainOperation(opType string, command bson.D) bool {
	if opType == "" || opType == "none" {
		return false
	}

	if opType == "insert" || opType == "update" || opType == "getmore" || opType == "killcursors" || opType == "remove" {
		return false
	}

	for _, cmd := range command {
		if unexplainableCommands[cmd.Key] {
			return false
		}
	}

	for _, v := range command {
		if v.Key == "pipeline" {
			pipelineArray, ok := v.Value.(bson.A)
			if !ok {
				continue
			}
			for _, stage := range pipelineArray {
				if stageDoc, ok := stage.(bson.M); ok {
					for stageName := range stageDoc {
						if unexplainablePipelineStages[stageName] {
							return false
						}
					}
				}
			}
		}
	}

	return true
}

func (s *mongodbScraper) shouldIncludeOperation(op bson.M) bool {
	if ns := getValue[string](op, namespaceKey); ns == "" {
		s.logger.Debug("Skipping operation without namespace", zap.Any("operation", op))
		return false
	}

	if db := s.getDBFromNamespace(getValue[string](op, namespaceKey)); db == "admin" {
		s.logger.Debug("Skipping operation for admin database", zap.Any("operation", op))
		return false
	}

	command := getValue[bson.D](op, commandKey)
	if len(command) == 0 {
		s.logger.Debug("Skipping operation without command", zap.Any("operation", op))
		return false
	}

	for _, v := range command {
		if v.Key == "hello" {
			s.logger.Debug("Skipping hello operation", zap.Any("operation", op))
			return false
		}
	}

	return true
}

// getValue extracts a value from a BSON document with type assertion
func getValue[T any](doc bson.M, key string) T {
	var zero T
	if val, ok := doc[key]; ok {
		if typedVal, ok := val.(T); ok {
			return typedVal
		}
	}
	return zero
}

func (*mongodbScraper) getDBFromNamespace(namespace string) string {
	parts := strings.SplitN(namespace, ".", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func (*mongodbScraper) getCollectionFromNamespace(namespace string) string {
	parts := strings.SplitN(namespace, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// getExplainPlan executes the explain command for a given command and database
func (s *mongodbScraper) getExplainPlan(ctx context.Context, command bson.D) (string, error) {
	explainableCommand := prepareCommandForExplain(command)
	if len(explainableCommand) == 0 {
		return "", errors.New("command cannot be explained")
	}

	explainCommand := bson.M{
		"explain": explainableCommand,
	}

	result, err := s.client.RunCommand(ctx, "admin", explainCommand)
	if err != nil {
		return "", fmt.Errorf("failed to execute explain command: %w", err)
	}

	cleanedResult := cleanExplainPlan(result)

	obfuscatedResult := obfuscateExplainPlan(cleanedResult)

	jsonBytes, err := json.Marshal(obfuscatedResult)
	if err != nil {
		return "", fmt.Errorf("failed to marshal explain plan to JSON: %w", err)
	}

	return string(jsonBytes), nil
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

func (s *mongodbScraper) collectIndexStats(ctx context.Context, now pcommon.Timestamp, databaseName, collectionName string, errs *scrapererror.ScrapeErrors) {
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

func (s *mongodbScraper) recordIndexStats(now pcommon.Timestamp, indexStats []bson.M, databaseName, collectionName string, errs *scrapererror.ScrapeErrors) {
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
