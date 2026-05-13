// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

var (
	unknownVersion = func() *version.Version { return version.Must(version.NewVersion("0.0")) }

	// otelNamespaceUUID is the official OTel namespace UUID for deterministic UUID v5 generation.
	otelNamespaceUUID = uuid.MustParse("4d63009a-8d0f-11ee-aad7-4c796ed8e320")
)

const (
	namespaceKey                    = "ns"
	commandKey                      = "command"
	opKey                           = "op"
	activeKey                       = "active"
	durationMicrosKey               = "microsecs_running"
	clientKey                       = "client"
	applicationNameKey              = "appName"
	effectiveUsersKey               = "effectiveUsers"
	cursorKey                       = "cursor"
	cursorAwaitDataKey              = "awaitData"
	cursorIDKey                     = "cursorId"
	cursorNBatchesReturnedKey       = "nBatchesReturned"
	cursorNDocsReturnedKey          = "nDocsReturned"
	cursorNoCursorTimeoutKey        = "noCursorTimeout"
	cursorOperationUsingCursorIDKey = "operationUsingCursorId"
	cursorOriginatingCommandKey     = "originatingCommand"
	cursorTailableKey               = "tailable"
	idKey                           = "id"
	lsidKey                         = "lsid"
	operationIDKey                  = "opid"
	planSummaryKey                  = "planSummary"
	queryFrameworkKey               = "queryFramework"
	prepareReadConflictsKey         = "prepareReadConflicts"
	writeConflictsKey               = "writeConflicts"
	numYieldsKey                    = "numYields"
	waitingForLockKey               = "waitingForLock"
	locksKey                        = "locks"
	lockStatsKey                    = "lockStats"
	waitingForFlowControlKey        = "waitingForFlowControl"
	flowControlStatsKey             = "flowControlStats"
	waitingForLatchKey              = "waitingForLatch"
)

// generateInstanceID generates a deterministic UUID v5 from server address and port.
func generateInstanceID(serverAddress string, serverPort int64) string {
	name := fmt.Sprintf("%s:%d", serverAddress, serverPort)
	return uuid.NewSHA1(otelNamespaceUUID, []byte(name)).String()
}

type mongodbScraper struct {
	logger             *zap.Logger
	config             *Config
	client             client
	secondaryClients   []client
	mongoVersion       *version.Version
	mb                 *metadata.MetricsBuilder
	lb                 *metadata.LogsBuilder
	prevReplTimestamp  pcommon.Timestamp
	prevReplCounts     map[string]int64
	prevTimestamp      pcommon.Timestamp
	prevFlushTimestamp pcommon.Timestamp
	prevCounts         map[string]int64
	prevFlushCount     int64
	obfuscator         *obfuscator
}

func newMongodbScraper(settings receiver.Settings, config *Config) *mongodbScraper {
	return &mongodbScraper{
		logger:             settings.Logger,
		config:             config,
		mb:                 metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		lb:                 metadata.NewLogsBuilder(config.LogsBuilderConfig, settings),
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
	now := pcommon.NewTimestampFromTime(time.Now())

	if err := s.scrapeLogsFromClient(ctx, s.client, now); err != nil {
		return plog.NewLogs(), err
	}

	for _, c := range s.secondaryClients {
		if err := s.scrapeLogsFromClient(ctx, c, now); err != nil {
			s.logger.Warn("Failed to scrape logs from secondary", zap.Error(err))
		}
	}

	return s.lb.Emit(), nil
}

func (s *mongodbScraper) scrapeLogsFromClient(ctx context.Context, c client, now pcommon.Timestamp) error {
	serverStatus, err := c.ServerStatus(ctx, "admin")
	if err != nil {
		s.logger.Debug("Failed to get server status for logs", zap.Error(err))
		return nil
	}

	serverAddress, serverPort, err := serverAddressAndPort(serverStatus)
	if err != nil {
		s.logger.Debug("Failed to extract server address and port for logs", zap.Error(err))
		return nil
	}

	operations, err := c.CurrentOp(ctx)
	if err != nil {
		s.logger.Error("Failed to get current operations", zap.Error(err))
		return nil
	}

	s.processCurrentOp(ctx, operations, now)

	rb := s.lb.NewResourceBuilder()
	rb.SetServerAddress(serverAddress)
	rb.SetServerPort(serverPort)
	rb.SetServiceInstanceID(generateInstanceID(serverAddress, serverPort))
	s.lb.EmitForResource(metadata.WithLogsResource(rb.Emit()))
	return nil
}

func (s *mongodbScraper) processCurrentOp(ctx context.Context, operations []bson.M, now pcommon.Timestamp) {
	emitted := uint64(0)
	for _, op := range operations {
		if emitted >= s.config.QuerySampleCollection.MaxRowsPerQuery {
			break
		}
		if !s.shouldIncludeOperation(op) {
			continue
		}
		namespace := getValue[string](op, namespaceKey)
		databaseName := getDBFromNamespace(namespace)
		command := getValue[bson.D](op, commandKey)
		opType := getValue[string](op, opKey)
		commandType, queryTruncated := getCommandDetails(command)
		prepareReadConflictCount := getInt64Value(op, prepareReadConflictsKey)
		writeConflictCount := getInt64Value(op, writeConflictsKey)
		yieldCount := getInt64Value(op, numYieldsKey)
		lsid := ""
		if lsidLookup, ok := newLookup(op[lsidKey]); ok {
			if lsidValue, ok := lsidLookup(idKey); ok {
				lsid = formatLsidID(lsidValue)
			}
		}
		cursor := getValue[bson.D](op, cursorKey)
		cursorAwaitData := getValueFromBSOND[bool](cursor, cursorAwaitDataKey)
		cursorBatchesReturnedCount := getInt64ValueFromBSOND(cursor, cursorNBatchesReturnedKey)
		cursorDocumentsReturnedCount := getInt64ValueFromBSOND(cursor, cursorNDocsReturnedKey)
		cursorID := getFormattedValueFromBSOND(cursor, cursorIDKey)
		cursorNoTimeout := getValueFromBSOND[bool](cursor, cursorNoCursorTimeoutKey)
		cursorOperationUsingCursorID := getFormattedValueFromBSOND(cursor, cursorOperationUsingCursorIDKey)
		cursorOriginatingCommand := getValueFromBSOND[bson.D](cursor, cursorOriginatingCommandKey)
		obfuscatedCursorOriginatingCommand := ""
		if len(cursorOriginatingCommand) > 0 {
			cleanedCursorOriginatingCommand := cleanCommand(cursorOriginatingCommand)
			obfuscatedCursorOriginatingCommand = s.obfuscator.obfuscateMongoDBString(cleanedCursorOriginatingCommand.String())
		}
		cursorTailable := getValueFromBSOND[bool](cursor, cursorTailableKey)
		planSummary := getValue[string](op, planSummaryKey)
		queryFramework := getValue[string](op, queryFrameworkKey)
		waitingForLock := getValue[bool](op, waitingForLockKey)
		locks := getJSONValue(op, locksKey)
		lockStats := getJSONValue(op, lockStatsKey)
		waitingForFlowControl := getValue[bool](op, waitingForFlowControlKey)
		flowControlStats := getJSONValue(op, flowControlStatsKey)
		waitingForLatch := hasNonEmptyDocumentValue(op[waitingForLatchKey])
		waitingForLatchDetails := ""
		if waitingForLatch {
			waitingForLatchDetails = getJSONValue(op, waitingForLatchKey)
		}
		operationStatus, ok := deriveOperationStatus(op, waitingForLock, waitingForFlowControl, waitingForLatch)
		if !ok {
			s.logger.Debug("Skipping operation without supported status", zap.Any("operation", op))
			continue
		}
		durationSeconds := float64(getInt64Value(op, durationMicrosKey)) / 1_000_000.0
		clientAddr := getValue[string](op, clientKey)
		applicationName := getValue[string](op, applicationNameKey)
		userName := extractEffectiveUserName(op)
		operationID := extractOperationID(op)
		clientAddress, clientPort := clientAddressAndPort(clientAddr)
		collectionName := getCollectionFromNamespace(namespace)
		cleanedCommand := cleanCommand(command)
		obfuscatedStatement := s.obfuscator.obfuscateMongoDBString(cleanedCommand.String())

		s.lb.RecordDbServerQuerySampleEvent(
			ctx,
			now,
			clientAddress,
			clientPort,
			metadata.AttributeDbSystemNameMongodb,
			databaseName,
			collectionName,
			commandType,
			obfuscatedStatement,
			queryTruncated,
			userName,
			applicationName,
			cursorAwaitData,
			cursorBatchesReturnedCount,
			cursorDocumentsReturnedCount,
			cursorID,
			cursorNoTimeout,
			cursorOperationUsingCursorID,
			obfuscatedCursorOriginatingCommand,
			cursorTailable,
			databaseName,
			lsid,
			namespace,
			operationID,
			planSummary,
			queryFramework,
			operationStatus,
			opType,
			durationSeconds,
			prepareReadConflictCount,
			writeConflictCount,
			yieldCount,
			waitingForLock,
			locks,
			lockStats,
			waitingForFlowControl,
			flowControlStats,
			waitingForLatch,
			waitingForLatchDetails,
		)
		emitted++
	}
	s.logger.Debug("Processed MongoDB current operations", zap.Int("total_operations", len(operations)))
}

func extractOperationID(op bson.M) string {
	if opIDRaw, ok := op[operationIDKey]; ok {
		return fmt.Sprintf("%v", opIDRaw)
	}
	return ""
}

func deriveOperationStatus(op bson.M, waitingForLock, waitingForFlowControl, waitingForLatch bool) (metadata.AttributeMongodbOperationStatus, bool) {
	if waitingForLock || waitingForFlowControl || waitingForLatch {
		return metadata.AttributeMongodbOperationStatusWaiting, true
	}
	if getValue[bool](op, activeKey) {
		return metadata.AttributeMongodbOperationStatusActive, true
	}
	return 0, false
}

func extractEffectiveUserName(op bson.M) string {
	effectiveUsers, ok := op[effectiveUsersKey].(bson.A)
	if !ok || len(effectiveUsers) == 0 {
		return ""
	}

	switch first := effectiveUsers[0].(type) {
	case bson.M:
		user, _ := first["user"].(string)
		return user
	case bson.D:
		for _, e := range first {
			if e.Key == "user" {
				user, _ := e.Value.(string)
				return user
			}
		}
	case map[string]any:
		user, _ := first["user"].(string)
		return user
	}

	return ""
}

func (s *mongodbScraper) shouldIncludeOperation(op bson.M) bool {
	if ns := getValue[string](op, namespaceKey); ns == "" {
		s.logger.Debug("Skipping operation without namespace", zap.Any("operation", op))
		return false
	}

	if db := getDBFromNamespace(getValue[string](op, namespaceKey)); db == "admin" || db == "local" {
		s.logger.Debug("Skipping operation for admin and local database", zap.Any("operation", op))
		return false
	}

	command := getValue[bson.D](op, commandKey)
	if len(command) == 0 {
		s.logger.Debug("Skipping operation without command", zap.Any("operation", op))
		return false
	}

	for _, v := range command {
		if v.Key == "hello" || v.Key == "ping" || v.Key == "isMaster" {
			s.logger.Debug("Skipping operation", zap.Any("operation", op))
			return false
		}
	}

	return true
}

func getDBFromNamespace(namespace string) string {
	parts := strings.SplitN(namespace, ".", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func getCollectionFromNamespace(namespace string) string {
	parts := strings.SplitN(namespace, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func getCommandDetails(command bson.D) (string, metadata.AttributeMongodbQueryTruncated) {
	if len(command) == 0 {
		return "", metadata.AttributeMongodbQueryTruncatedNotTruncated
	}

	commandType := command[0].Key
	for _, elem := range command {
		if elem.Key == "$truncated" {
			return commandType, metadata.AttributeMongodbQueryTruncatedTruncated
		}
	}
	return commandType, metadata.AttributeMongodbQueryTruncatedNotTruncated
}

type valueLookup func(string) (any, bool)

func newBSONDLookup(doc bson.D) valueLookup {
	return func(key string) (any, bool) {
		for _, elem := range doc {
			if elem.Key == key {
				return elem.Value, true
			}
		}
		return nil, false
	}
}

func newBSONMLookup(doc bson.M) valueLookup {
	return func(key string) (any, bool) {
		value, ok := doc[key]
		return value, ok
	}
}

func newLookup(doc any) (valueLookup, bool) {
	switch typedValue := doc.(type) {
	case bson.D:
		return newBSONDLookup(typedValue), true
	case bson.M:
		return newBSONMLookup(typedValue), true
	case map[string]any:
		return func(key string) (any, bool) {
			value, ok := typedValue[key]
			return value, ok
		}, true
	default:
		return nil, false
	}
}

func getLookupValue[T any](lookup valueLookup, key string) T {
	var zero T
	value, ok := lookup(key)
	if !ok {
		return zero
	}
	typedValue, ok := value.(T)
	if !ok {
		return zero
	}
	return typedValue
}

func getLookupInt64Value(lookup valueLookup, key string) int64 {
	value, ok := lookup(key)
	if !ok {
		return 0
	}
	intValue, _ := toInt64(value)
	return intValue
}

func getLookupFormattedValue(lookup valueLookup, key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

func formatLsidID(value any) string {
	if binaryValue, ok := value.(bson.Binary); ok && binaryValue.Subtype == 0x04 && len(binaryValue.Data) == 16 {
		u, err := uuid.FromBytes(binaryValue.Data)
		if err == nil {
			return u.String()
		}
	}
	return fmt.Sprintf("%v", value)
}

func getLookupJSONValue(lookup valueLookup, key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	jsonValue, err := bson.MarshalExtJSON(value, false, false)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(jsonValue)
}

func getValueFromBSOND[T any](doc bson.D, key string) T {
	return getLookupValue[T](newBSONDLookup(doc), key)
}

func getInt64ValueFromBSOND(doc bson.D, key string) int64 {
	return getLookupInt64Value(newBSONDLookup(doc), key)
}

func getFormattedValueFromBSOND(doc bson.D, key string) string {
	return getLookupFormattedValue(newBSONDLookup(doc), key)
}

func getFormattedValue(doc any, key string) string {
	lookup, ok := newLookup(doc)
	if !ok {
		return ""
	}
	return getLookupFormattedValue(lookup, key)
}

func hasNonEmptyDocumentValue(value any) bool {
	switch typedValue := value.(type) {
	case bson.D:
		return len(typedValue) > 0
	case bson.M:
		return len(typedValue) > 0
	case map[string]any:
		return len(typedValue) > 0
	default:
		return false
	}
}

// getValue extracts a value from a BSON document with type assertion
func getValue[T any](doc bson.M, key string) T {
	return getLookupValue[T](newBSONMLookup(doc), key)
}

func getInt64Value(doc bson.M, key string) int64 {
	return getLookupInt64Value(newBSONMLookup(doc), key)
}

func toInt64(val any) (int64, bool) {
	switch typedVal := val.(type) {
	case int:
		return int64(typedVal), true
	case int8:
		return int64(typedVal), true
	case int16:
		return int64(typedVal), true
	case int32:
		return int64(typedVal), true
	case int64:
		return typedVal, true
	case uint:
		if uint64(typedVal) > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(typedVal), true
	case uint8:
		return int64(typedVal), true
	case uint16:
		return int64(typedVal), true
	case uint32:
		return int64(typedVal), true
	case uint64:
		if typedVal > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(typedVal), true
	default:
		return 0, false
	}
}

func getJSONValue(doc bson.M, key string) string {
	return getLookupJSONValue(newBSONMLookup(doc), key)
}

func clientAddressAndPort(clientAddr string) (string, int64) {
	if clientAddr == "" {
		return "", 0
	}

	host, port, err := net.SplitHostPort(clientAddr)
	if err == nil {
		parsedPort, parseErr := strconv.ParseInt(port, 10, 64)
		if parseErr != nil {
			return host, 0
		}
		return host, parsedPort
	}

	if strings.Count(clientAddr, ":") == 1 {
		host, port, found := strings.Cut(clientAddr, ":")
		if !found {
			return clientAddr, 0
		}
		parsedPort, parseErr := strconv.ParseInt(port, 10, 64)
		if parseErr != nil {
			return host, 0
		}
		return host, parsedPort
	}

	return clientAddr, 0
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

	// Collect metrics for each database
	for _, dbName := range dbNames {
		s.collectDatabase(ctx, now, dbName, errs)
		collectionNames, err := s.client.ListCollectionNames(ctx, dbName)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf("failed to fetch collection names: %w", err))
			continue
		}

		for _, collectionName := range collectionNames {
			s.collectIndexStats(ctx, now, dbName, collectionName, errs)
		}
	}

	// Emit single resource for the server
	rb := s.mb.NewResourceBuilder()
	rb.SetServerAddress(serverAddress)
	rb.SetServerPort(serverPort)
	rb.SetServiceInstanceID(generateInstanceID(serverAddress, serverPort))
	s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
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
	if s.config.Metrics.MongodbCollectionCount.Enabled {
		s.recordCollections(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbDataSize.Enabled {
		s.recordDataSize(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbExtentCount.Enabled {
		s.recordExtentCount(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbIndexSize.Enabled {
		s.recordIndexSize(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbIndexCount.Enabled {
		s.recordIndexCount(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbObjectCount.Enabled {
		s.recordObjectCount(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbStorageSize.Enabled {
		s.recordStorageSize(now, doc, dbName, errs)
	}
}

func (s *mongodbScraper) recordNormalServerStats(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	if s.config.Metrics.MongodbConnectionCount.Enabled {
		s.recordConnections(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbDocumentOperationCount.Enabled {
		s.recordDocumentOperations(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbMemoryUsage.Enabled {
		s.recordMemoryUsage(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbLockAcquireCount.Enabled {
		s.recordLockAcquireCounts(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbLockAcquireWaitCount.Enabled {
		s.recordLockAcquireWaitCounts(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbLockAcquireTime.Enabled {
		s.recordLockTimeAcquiringMicros(now, doc, dbName, errs)
	}

	if s.config.Metrics.MongodbLockDeadlockCount.Enabled {
		s.recordLockDeadlockCount(now, doc, dbName, errs)
	}
}

func (s *mongodbScraper) recordAdminStats(now pcommon.Timestamp, document bson.M, errs *scrapererror.ScrapeErrors) {
	if s.config.Metrics.MongodbCacheOperations.Enabled {
		s.recordCacheOperations(now, document, errs)
	}

	if s.config.Metrics.MongodbCursorCount.Enabled {
		s.recordCursorCount(now, document, errs)
	}

	if s.config.Metrics.MongodbCursorTimeoutCount.Enabled {
		s.recordCursorTimeoutCount(now, document, errs)
	}

	if s.config.Metrics.MongodbGlobalLockTime.Enabled {
		s.recordGlobalLockTime(now, document, errs)
	}

	if s.config.Metrics.MongodbNetworkRequestCount.Enabled {
		s.recordNetworkCount(now, document, errs)
	}

	s.recordOperations(now, document, errs)

	s.recordOperationsRepl(now, document, errs)

	if s.config.Metrics.MongodbSessionCount.Enabled {
		s.recordSessionCount(now, document, errs)
	}

	if s.config.Metrics.MongodbOperationLatencyTime.Enabled {
		s.recordLatencyTime(now, document, errs)
	}

	if s.config.Metrics.MongodbUptime.Enabled {
		s.recordUptime(now, document, errs)
	}

	if s.config.Metrics.MongodbHealth.Enabled {
		s.recordHealth(now, document, errs)
	}

	if s.config.Metrics.MongodbActiveWrites.Enabled {
		s.recordActiveWrites(now, document, errs)
	}

	if s.config.Metrics.MongodbActiveReads.Enabled {
		s.recordActiveReads(now, document, errs)
	}

	if s.config.Metrics.MongodbFlushesRate.Enabled {
		s.recordFlushesPerSecond(now, document, errs)
	}

	if s.config.Metrics.MongodbWtcacheBytesRead.Enabled {
		s.recordWTCacheBytes(now, document, errs)
	}

	if s.config.Metrics.MongodbPageFaults.Enabled {
		s.recordPageFaults(now, document, errs)
	}
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
