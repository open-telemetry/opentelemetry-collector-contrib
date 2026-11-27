// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"crypto/fips140"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

var (
	unknownVersion = func() *version.Version { return version.Must(version.NewVersion("0.0")) }

	// otelNamespaceUUID is the official OTel namespace UUID for deterministic UUID v5 generation.
	otelNamespaceUUID = uuid.MustParse("4d63009a-8d0f-11ee-aad7-4c796ed8e320")
)

const (
	adminDatabase               = "admin"
	replSetGetStatusCommand     = "replSetGetStatus"
	defaultServiceName          = "unknown_service:mongodb"
	namespaceKey                = "ns"
	commandKey                  = "command"
	commentKey                  = "comment"
	opKey                       = "op"
	activeKey                   = "active"
	durationMicrosKey           = "microsecs_running"
	clientKey                   = "client"
	applicationNameKey          = "appName"
	effectiveUsersKey           = "effectiveUsers"
	userKey                     = "user"
	cursorKey                   = "cursor"
	cursorAwaitDataKey          = "awaitData"
	cursorIDKey                 = "cursorId"
	cursorNBatchesReturnedKey   = "nBatchesReturned"
	cursorNDocsReturnedKey      = "nDocsReturned"
	cursorNoCursorTimeoutKey    = "noCursorTimeout"
	cursorOriginatingCommandKey = "originatingCommand"
	cursorTailableKey           = "tailable"
	idKey                       = "id"
	lsidKey                     = "lsid"
	operationIDKey              = "opid"
	planSummaryKey              = "planSummary"
	queryFrameworkKey           = "queryFramework"
	prepareReadConflictsKey     = "prepareReadConflicts"
	writeConflictsKey           = "writeConflicts"
	numYieldsKey                = "numYields"
	waitingForLockKey           = "waitingForLock"
	waitingForFlowControlKey    = "waitingForFlowControl"
	waitingForLatchKey          = "waitingForLatch"
)

// generateInstanceID generates a deterministic UUID v5 from server address and port.
func generateInstanceID(serverAddress string, serverPort int64) string {
	name := fmt.Sprintf("%s:%d", serverAddress, serverPort)
	// UUID v5 is defined over SHA-1, which FIPS 140-3 does not approve for new uses.
	// The hash only derives a stable identifier and is not a security function, so it
	// runs outside strict enforcement (GODEBUG=fips140=only) instead of panicking there.
	var id uuid.UUID
	fips140.WithoutEnforcement(func() {
		id = uuid.NewSHA1(otelNamespaceUUID, []byte(name))
	})
	return id.String()
}

type mongodbScraper struct {
	logger *zap.Logger
	config *Config
	client client
	// members are the replica set members found at start, each with a client pinned to that node.
	// When there are any, query samples come only from them, so an election cannot make the main
	// client, which follows the primary, report a node that a member client already reports.
	members []*member
	// discoverMembers is set on the scraper that collects query samples, the only one that uses
	// member clients.
	discoverMembers       bool
	mongoVersion          *version.Version
	mb                    *metadata.MetricsBuilder
	lb                    *metadata.LogsBuilder
	prevReplTimestamp     pcommon.Timestamp
	prevReplCounts        map[string]int64
	prevTimestamp         pcommon.Timestamp
	prevFlushTimestamp    pcommon.Timestamp
	prevCounts            map[string]int64
	prevFlushCount        int64
	obfuscator            *obfuscator
	planCache             *lru.LRU[string, string]
	lastScrapeTime        time.Time // upper bound of the last successful scrape window; used by both profiler and getLog paths
	lastTopQueryExecution time.Time
	// collectorHostName stands in for a loopback server address. It is empty when the host name
	// could not be determined, in which case the address is reported as the server gave it.
	collectorHostName string
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
		collectorHostName:  resolveCollectorHostName(settings.Logger),
	}
}

// resolveServerAddress reports the network location of the server described by serverStatus, with a
// loopback address replaced by the collector host's name. Both server.address and the
// service.instance.id seed are taken from this one result, so they cannot name different machines.
func (s *mongodbScraper) resolveServerAddress(serverStatus bson.M) (string, int64, error) {
	address, port, err := serverAddressAndPort(serverStatus)
	if err != nil {
		return "", 0, err
	}
	return resolveLoopbackHost(address, s.collectorHostName), port, nil
}

func (s *mongodbScraper) start(ctx context.Context, _ component.Host) error {
	c, err := newClient(ctx, s.config, s.logger, false)
	if err != nil {
		return fmt.Errorf("create mongo client: %w", err)
	}
	s.client = c

	if !s.discoverMembers {
		return nil
	}
	if reason := s.config.secondaryDiscoverySkipReason(); reason != "" {
		s.logger.Info("skipping replica set member discovery, scraping only the configured host",
			zap.String("reason", reason))
		return nil
	}

	hosts, err := s.discoverMemberHosts(ctx)
	if err != nil {
		s.logger.Warn("failed to discover replica set members, scraping only the configured host", zap.Error(err))
		return nil
	}

	for _, h := range hosts {
		memberConfig := *s.config
		memberConfig.Hosts = []confignet.TCPAddrConfig{{Endpoint: h}}

		c, err := newClient(ctx, &memberConfig, s.logger, true)
		if err != nil {
			s.logger.Warn("failed to connect to replica set member", zap.String("host", h), zap.Error(err))
			continue
		}
		s.members = append(s.members, &member{host: h, client: c})
		s.logger.Info("scraping discovered replica set member", zap.String("host", h))
	}
	return nil
}

// member is a replica set member with a client connected directly to it. The client stays on that
// node whatever its role, so an election changes nothing about what the member reports.
type member struct {
	host    string
	client  client
	failing bool
}

// recordMemberHealth logs when collection from a member starts or stops failing, at warn level when
// it fails, so the failure is visible without debug logging and without repeating the message on
// every scrape.
func (s *mongodbScraper) recordMemberHealth(m *member, err error) {
	switch {
	case err != nil && !m.failing:
		m.failing = true
		s.logger.Warn("failed to collect from replica set member, its data is missing until it recovers",
			zap.String("host", m.host), zap.Error(err))
	case err == nil && m.failing:
		m.failing = false
		s.logger.Info("collecting from replica set member again", zap.String("host", m.host))
	}
}

func (s *mongodbScraper) shutdown(ctx context.Context) error {
	var errs []error

	if s.client != nil {
		if err := s.client.Disconnect(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	for _, m := range s.members {
		if err := m.client.Disconnect(ctx); err != nil {
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

// memberScrapeTimeout bounds one member's share of a scrape. Members are scraped concurrently and
// each gets its own deadline, so a member that hangs (for example one being removed from the set)
// delays only its own data rather than every member's.
func (s *mongodbScraper) memberScrapeTimeout() time.Duration {
	timeout := s.config.ControllerConfig.CollectionInterval
	if s.config.Timeout > 0 && s.config.Timeout < timeout {
		timeout = s.config.Timeout
	}
	if timeout <= 0 {
		timeout = time.Minute
	}
	return timeout
}

// forEachMember runs fn for every member concurrently, each with its own deadline, and waits for all.
func (s *mongodbScraper) forEachMember(ctx context.Context, fn func(ctx context.Context, i int, m *member)) {
	var wg sync.WaitGroup
	timeout := s.memberScrapeTimeout()
	for i, m := range s.members {
		wg.Go(func() {
			mctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			fn(mctx, i, m)
		})
	}
	wg.Wait()
}

func (s *mongodbScraper) scrapeLogs(ctx context.Context) (plog.Logs, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	if len(s.members) == 0 {
		s.scrapeLogsFromClient(ctx, s.client, now)
		return s.lb.Emit(), nil
	}

	// Reading each member's current operations is the slow, network-bound part, so it runs
	// concurrently; turning them into log records shares the logs builder and stays sequential.
	snapshots := make([]currentOpSnapshot, len(s.members))
	s.forEachMember(ctx, func(ctx context.Context, i int, m *member) {
		snapshots[i] = fetchCurrentOp(ctx, m.client)
	})
	for i, m := range s.members {
		s.recordMemberHealth(m, s.emitCurrentOp(snapshots[i], now))
	}

	return s.lb.Emit(), nil
}

// currentOpSnapshot is one node's server status and in-progress operations, read for query samples.
type currentOpSnapshot struct {
	serverStatus bson.M
	operations   []bson.M
	// statusErr and opErr record which of the two reads failed, so each is logged at the level the
	// receiver has always used for it.
	statusErr error
	opErr     error
}

func fetchCurrentOp(ctx context.Context, c client) currentOpSnapshot {
	serverStatus, err := c.ServerStatus(ctx, "admin")
	if err != nil {
		return currentOpSnapshot{statusErr: err}
	}
	operations, err := c.CurrentOp(ctx)
	if err != nil {
		return currentOpSnapshot{opErr: err}
	}
	return currentOpSnapshot{serverStatus: serverStatus, operations: operations}
}

// scrapeLogsFromClient collects query samples from the node the main client is connected to and logs
// any failure itself: that node has no member health tracking, so a failure that is only returned
// would never be seen.
func (s *mongodbScraper) scrapeLogsFromClient(ctx context.Context, c client, now pcommon.Timestamp) {
	snap := fetchCurrentOp(ctx, c)
	switch {
	case snap.statusErr != nil:
		s.logger.Debug("Failed to get server status for logs", zap.Error(snap.statusErr))
		return
	case snap.opErr != nil:
		s.logger.Error("Failed to get current operations", zap.Error(snap.opErr))
		return
	}
	if err := s.emitCurrentOp(snap, now); err != nil {
		s.logger.Debug("Failed to extract server address and port for logs", zap.Error(err))
	}
}

// emitCurrentOp turns a snapshot into query sample log records. It returns the snapshot's error, or
// the error resolving the node's address, without logging it; callers decide how to report it.
func (s *mongodbScraper) emitCurrentOp(snap currentOpSnapshot, now pcommon.Timestamp) error {
	if snap.statusErr != nil {
		return fmt.Errorf("failed to get server status: %w", snap.statusErr)
	}
	if snap.opErr != nil {
		return fmt.Errorf("failed to get current operations: %w", snap.opErr)
	}

	serverAddress, serverPort, err := s.resolveServerAddress(snap.serverStatus)
	if err != nil {
		return fmt.Errorf("failed to extract server address and port: %w", err)
	}

	s.processCurrentOp(snap.operations, now)

	rb := s.lb.NewResourceBuilder()
	setResourceAttributes(rb, serverAddress, serverPort, s.mongoVersion)
	s.lb.EmitForResource(metadata.WithLogsResource(rb.Emit()))
	return nil
}

func (s *mongodbScraper) processCurrentOp(operations []bson.M, now pcommon.Timestamp) {
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
		recordCtx := context.Background()
		if comment, ok := lookup(command, commentKey); ok {
			recordCtx = contextWithTraceContextComment(comment)
		}
		opType := getValue[string](op, opKey)
		queryTruncated, commandComment, operationName := extractCommandMetadata(command, opType)
		preparedReadConflictCount := getInt64Value(op, prepareReadConflictsKey)
		writeConflictCount := getInt64Value(op, writeConflictsKey)
		yieldCount := getInt64Value(op, numYieldsKey)
		lsid := ""
		if v, ok := lookup(op[lsidKey], idKey); ok {
			lsid = formatLsidID(v)
		}
		cursor := getValue[bson.D](op, cursorKey)
		cursorAwaitData := getValue[bool](cursor, cursorAwaitDataKey)
		cursorID := getFormattedValue(cursor, cursorIDKey)
		cursorNoTimeout := getValue[bool](cursor, cursorNoCursorTimeoutKey)
		cursorReturnedBatches := getInt64Value(cursor, cursorNBatchesReturnedKey)
		cursorReturnedDocuments := getInt64Value(cursor, cursorNDocsReturnedKey)
		cursorOriginatingCommand := getValue[bson.D](cursor, cursorOriginatingCommandKey)
		obfuscatedCursorOriginatingCommand := ""
		if len(cursorOriginatingCommand) > 0 {
			cleanedCursorOriginatingCommand := cleanCommand(cursorOriginatingCommand)
			obfuscated, err := s.obfuscator.obfuscateCommand(cleanedCursorOriginatingCommand)
			if err != nil {
				s.logger.Debug("Failed to obfuscate originating command", zap.Error(err))
			}
			obfuscatedCursorOriginatingCommand = obfuscated
		}
		cursorTailable := getValue[bool](cursor, cursorTailableKey)
		planSummary := getValue[string](op, planSummaryKey)
		queryFramework := getValue[string](op, queryFrameworkKey)
		waitingForLock := getValue[bool](op, waitingForLockKey)
		waitingForFlowControl := getValue[bool](op, waitingForFlowControlKey)
		waitingForLatchDetails := getJSONValue(op, waitingForLatchKey)
		waitingForLatch := waitingForLatchDetails != ""
		waitTypes := buildWaitTypes(waitingForLock, waitingForFlowControl, waitingForLatch)
		operationState, ok := deriveOperationState(op, waitingForLock, waitingForFlowControl, waitingForLatch)
		if !ok {
			s.logger.Debug("Skipping operation without supported state", zap.Any("operation", op))
			continue
		}
		durationSeconds := float64(getInt64Value(op, durationMicrosKey)) / 1_000_000.0
		clientAddr := getValue[string](op, clientKey)
		clientAppName := getValue[string](op, applicationNameKey)
		userName := extractEffectiveUserName(op)
		operationID := extractOperationID(op)
		clientAddress, clientPort := clientAddressAndPort(clientAddr)
		collectionName := getCollectionFromNamespace(namespace)
		cleanedCommand := cleanCommand(command)
		obfuscatedStatement, err := s.obfuscator.obfuscateCommand(cleanedCommand)
		if err != nil {
			s.logger.Debug("Failed to obfuscate command", zap.Error(err))
		}

		s.lb.RecordDbServerQuerySampleEvent(
			recordCtx,
			now,
			clientAddress,
			clientPort,
			metadata.AttributeDbSystemNameMongodb,
			databaseName,
			collectionName,
			operationName,
			obfuscatedStatement,
			queryTruncated,
			userName,
			clientAppName,
			cursorAwaitData,
			cursorID,
			cursorNoTimeout,
			obfuscatedCursorOriginatingCommand,
			cursorReturnedBatches,
			cursorReturnedDocuments,
			cursorTailable,
			lsid,
			operationID,
			planSummary,
			queryFramework,
			operationState,
			opType,
			commandComment,
			durationSeconds,
			preparedReadConflictCount,
			writeConflictCount,
			yieldCount,
			waitTypes,
			waitingForLatchDetails,
		)
		emitted++
	}
	s.logger.Debug("Processed MongoDB current operations", zap.Int("total_operations", len(operations)))
}

// contextWithTraceContextComment extracts propagated context from the top-level
// string fields in a document-valued MongoDB comment. Query samples must not
// inherit the collector's scrape span when no application context is present.
func contextWithTraceContextComment(comment any) context.Context {
	carrier := make(propagation.MapCarrier)
	add := func(key string, value any) {
		if key == "originalComment" {
			return
		}
		if str, ok := value.(string); ok {
			carrier.Set(key, str)
		}
	}
	switch value := comment.(type) {
	case bson.D:
		for _, elem := range value {
			add(elem.Key, elem.Value)
		}
	case bson.M:
		for key, val := range value {
			add(key, val)
		}
	case map[string]any:
		for key, val := range value {
			add(key, val)
		}
	default:
		return context.Background()
	}
	ctx := (propagation.TraceContext{}).Extract(context.Background(), carrier)
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return context.Background()
	}
	return ctx
}

func extractOperationID(op bson.M) string {
	if opIDRaw, ok := op[operationIDKey]; ok {
		return fmt.Sprintf("%v", opIDRaw)
	}
	return ""
}

func deriveOperationState(op bson.M, waitingForLock, waitingForFlowControl, waitingForLatch bool) (metadata.AttributeMongodbOperationState, bool) {
	if waitingForLock || waitingForFlowControl || waitingForLatch {
		return metadata.AttributeMongodbOperationStateWaiting, true
	}
	if getValue[bool](op, activeKey) {
		return metadata.AttributeMongodbOperationStateActive, true
	}
	return 0, false
}

func buildWaitTypes(waitingForLock, waitingForFlowControl, waitingForLatch bool) []any {
	waitTypes := []any{}
	if waitingForLock {
		waitTypes = append(waitTypes, "lock")
	}
	if waitingForFlowControl {
		waitTypes = append(waitTypes, "flow_control")
	}
	if waitingForLatch {
		waitTypes = append(waitTypes, "latch")
	}
	return waitTypes
}

func extractEffectiveUserName(op bson.M) string {
	users, ok := op[effectiveUsersKey].(bson.A)
	if !ok || len(users) == 0 {
		return ""
	}
	return getValue[string](users[0], userKey)
}

func (s *mongodbScraper) shouldIncludeOperation(op bson.M) bool {
	if len(getValue[bson.D](op, commandKey)) == 0 {
		s.logger.Debug("Skipping operation with empty command", zap.Any("operation", op))
		return false
	}
	return true
}

// extractCommandMetadata extracts the truncation flag, comments, and operation
// name from a command document.
func extractCommandMetadata(command bson.D, opType string) (bool, []any, string) {
	operationName := opType
	if len(command) > 0 {
		operationName = command[0].Key
	}
	truncated := false
	comments := []any{}
	for _, elem := range command {
		switch elem.Key {
		case "$truncated":
			truncated = true
		case commentKey:
			if values, ok := elem.Value.(bson.A); ok {
				for _, value := range values {
					if comment, ok := applicationCommandComment(value); ok {
						comments = append(comments, stringifyCommandComment(comment))
					}
				}
			} else if comment, ok := applicationCommandComment(elem.Value); ok {
				comments = append(comments, stringifyCommandComment(comment))
			}
		}
	}
	return truncated, comments, operationName
}

// applicationCommandComment removes propagation fields from a document-valued
// comment when it contains a valid traceparent. In that case, only the
// application's original comment is reported, if present.
func applicationCommandComment(value any) (any, bool) {
	if !trace.SpanContextFromContext(contextWithTraceContextComment(value)).IsValid() {
		return value, true
	}
	originalComment, ok := lookup(value, "originalComment")
	return originalComment, ok && originalComment != nil
}

func stringifyCommandComment(value any) string {
	if comment, ok := value.(string); ok {
		return comment
	}
	if value == nil {
		return "null"
	}
	valueType, data, err := bson.MarshalValue(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return bson.RawValue{Type: valueType, Value: data}.String()
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

// lookup returns the value for key from a BSON document. It accepts bson.M,
// bson.D, or map[string]any so callers do not need to know which container
// the driver returned.
func lookup(doc any, key string) (any, bool) {
	switch d := doc.(type) {
	case bson.M:
		v, ok := d[key]
		return v, ok
	case bson.D:
		for _, elem := range d {
			if elem.Key == key {
				return elem.Value, true
			}
		}
		return nil, false
	case map[string]any:
		v, ok := d[key]
		return v, ok
	default:
		return nil, false
	}
}

// getValue returns the value for key cast to T, or T's zero value if the key
// is missing or the value is not of type T.
func getValue[T any](doc any, key string) T {
	var zero T
	v, ok := lookup(doc, key)
	if !ok {
		return zero
	}
	typed, _ := v.(T)
	return typed
}

// getInt64Value returns the value for key coerced to int64 via parseInt
// (shared with metrics.go), or 0 when the key is missing or the value is
// not a supported numeric type.
func getInt64Value(doc any, key string) int64 {
	v, ok := lookup(doc, key)
	if !ok {
		return 0
	}
	n, _ := parseInt(v)
	return n
}

// getFormattedValue returns fmt.Sprintf("%v", value) for the key, or "" when
// the key is missing.
func getFormattedValue(doc any, key string) string {
	v, ok := lookup(doc, key)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// getJSONValue returns the Extended JSON encoding of key's value, or "" if the
// key is missing, unmarshalable, or encodes to an empty document ("{}").
func getJSONValue(doc any, key string) string {
	v, ok := lookup(doc, key)
	if !ok {
		return ""
	}
	j, err := bson.MarshalExtJSON(v, false, false)
	if err != nil {
		return ""
	}
	if s := string(j); s != "{}" {
		return s
	}
	return ""
}

// formatLsidID renders a logical-session UUID. MongoDB returns lsid.id as a
// bson.Binary with the standard UUID subtype, which fmt.Sprintf would print
// as a raw byte slice; convert to the canonical hex form when possible.
func formatLsidID(value any) string {
	if b, ok := value.(bson.Binary); ok && b.Subtype == 0x04 && len(b.Data) == 16 {
		if u, err := uuid.FromBytes(b.Data); err == nil {
			return u.String()
		}
	}
	return fmt.Sprintf("%v", value)
}

func clientAddressAndPort(clientAddr string) (string, int64) {
	if clientAddr == "" {
		return "", 0
	}

	host, port, err := net.SplitHostPort(clientAddr)
	if err != nil {
		return clientAddr, 0
	}

	parsedPort, parseErr := strconv.ParseInt(port, 10, 64)
	if parseErr != nil {
		return host, 0
	}
	return host, parsedPort
}

func setResourceAttributes(rb *metadata.ResourceBuilder, serverAddress string, serverPort int64, mongoVersion *version.Version) {
	rb.SetServerAddress(serverAddress)
	rb.SetServerPort(serverPort)
	rb.SetServiceInstanceID(generateInstanceID(serverAddress, serverPort))
	rb.SetServiceName(defaultServiceName)
	rb.SetServiceNamespace("")
	if mongoVersion != nil && !mongoVersion.Equal(unknownVersion()) {
		rb.SetDbSystemVersion(mongoVersion.String())
	}
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
	serverAddress, serverPort, aErr := s.resolveServerAddress(serverStatus)
	if aErr != nil {
		errs.Add(fmt.Errorf("failed to fetch server address and port: %w", aErr))
		return
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	s.mb.RecordMongodbDatabaseCountDataPoint(now, int64(len(dbNames)))
	s.recordAdminStats(now, serverStatus, errs)
	s.collectTopStats(ctx, now, errs)
	s.collectReplicaSetMetrics(ctx, now, errs)

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
	setResourceAttributes(rb, serverAddress, serverPort, s.mongoVersion)
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

// bsonObjectTooLargeCode is the MongoDB error code for a reply bigger than 16 MB.
const bsonObjectTooLargeCode = 10334

func (s *mongodbScraper) collectTopStats(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	if !s.config.MetricsBuilderConfig.Metrics.MongodbOperationTime.Enabled {
		return
	}
	topStats, err := s.client.TopStats(ctx)
	if err != nil {
		var srvErr mongo.ServerError
		if errors.As(err, &srvErr) && srvErr.HasErrorCode(bsonObjectTooLargeCode) {
			s.logger.Warn("top response is larger than 16MB so mongodb.operation.time cannot be collected, disable the mongodb.operation.time metric to stop this warning", zap.Error(err))
			return
		}
		errs.AddPartial(1, fmt.Errorf("failed to fetch top stats metrics: %w", err))
		return
	}
	s.recordOperationTime(now, topStats, errs)
}

// replicationUnavailableCodes are the server error codes returned when the deployment does not
// expose replica set state: a standalone mongod, a mongos router, or a member whose replica set has
// not been initiated. Unauthorized is deliberately absent: it means the configured user lacks a
// privilege it can be granted, so it is reported rather than skipped.
var replicationUnavailableCodes = []int{
	26, // NamespaceNotFound
	59, // CommandNotFound
	76, // NoReplicationEnabled
	94, // NotYetInitialized
}

func isReplicationUnavailable(err error) bool {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true
	}
	var srvErr mongo.ServerError
	if !errors.As(err, &srvErr) {
		return false
	}
	return slices.ContainsFunc(replicationUnavailableCodes, srvErr.HasErrorCode)
}

// reportReplicationError records err as a partial scrape failure, unless it only means the
// deployment does not expose replica set state, in which case the metrics are skipped.
func (s *mongodbScraper) reportReplicationError(err error, message string, failed int, errs *scrapererror.ScrapeErrors) {
	if isReplicationUnavailable(err) {
		s.logger.Debug(message, zap.Error(err))
		return
	}
	errs.AddPartial(failed, fmt.Errorf("%s: %w", message, err))
}

// countEnabled returns how many of the given metric enabled flags are set, which is the number of
// metrics a failed server read leaves unrecorded.
func countEnabled(enabled ...bool) int {
	count := 0
	for _, e := range enabled {
		if e {
			count++
		}
	}
	return count
}

func (s *mongodbScraper) collectReplicaSetMetrics(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	metrics := s.config.MetricsBuilderConfig.Metrics

	if metrics.MongodbOplogUsage.Enabled || metrics.MongodbOplogLimit.Enabled {
		s.collectOplogStats(ctx, now, errs)
	}

	var oplogWindow float64
	var hasOplogWindow bool
	if metrics.MongodbOplogWindow.Enabled || metrics.MongodbReplicaSetHeadroom.Enabled {
		oplogWindow, hasOplogWindow = s.collectOplogWindow(ctx, now, errs)
	}

	if metrics.MongodbReplicaSetHeadroom.Enabled || metrics.MongodbReplicaSetLag.Enabled ||
		metrics.MongodbReplicaSetMemberCount.Enabled || metrics.MongodbReplicaStatus.Enabled {
		s.collectReplicaSetStatus(ctx, now, oplogWindow, hasOplogWindow, errs)
	}
}

func (s *mongodbScraper) collectOplogStats(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	metrics := s.config.MetricsBuilderConfig.Metrics
	oplogStats, err := s.client.OplogStats(ctx)
	if err != nil {
		s.reportReplicationError(err, "failed to fetch oplog stats metrics", countEnabled(metrics.MongodbOplogUsage.Enabled, metrics.MongodbOplogLimit.Enabled), errs)
		return
	}

	if metrics.MongodbOplogUsage.Enabled {
		s.recordOplogUsage(now, oplogStats, errs)
	}

	if metrics.MongodbOplogLimit.Enabled {
		s.recordOplogLimit(now, oplogStats, errs)
	}
}

// collectOplogWindow returns the time span the oplog covers, which mongodb.replica_set.headroom
// is measured against. A failure here is counted against both metrics, since neither can be
// recorded without it.
func (s *mongodbScraper) collectOplogWindow(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) (float64, bool) {
	metrics := s.config.MetricsBuilderConfig.Metrics
	oldest, newest, err := s.client.OplogBounds(ctx)
	if err != nil {
		s.reportReplicationError(err, "failed to fetch oplog window and replica set headroom metrics",
			countEnabled(metrics.MongodbOplogWindow.Enabled, metrics.MongodbReplicaSetHeadroom.Enabled), errs)
		return 0, false
	}

	if metrics.MongodbOplogWindow.Enabled {
		s.recordOplogWindow(now, oldest, newest)
	}
	return oplogWindowSeconds(oldest, newest), true
}

func (s *mongodbScraper) collectReplicaSetStatus(ctx context.Context, now pcommon.Timestamp, oplogWindow float64, hasOplogWindow bool, errs *scrapererror.ScrapeErrors) {
	metrics := s.config.MetricsBuilderConfig.Metrics
	// Headroom is left out when the oplog window is missing, as collectOplogWindow already counted it.
	failed := countEnabled(metrics.MongodbReplicaSetMemberCount.Enabled, metrics.MongodbReplicaStatus.Enabled,
		metrics.MongodbReplicaSetLag.Enabled, metrics.MongodbReplicaSetHeadroom.Enabled && hasOplogWindow)

	status, err := s.client.RunCommand(ctx, adminDatabase, bson.M{replSetGetStatusCommand: 1})
	if err != nil {
		s.reportReplicationError(err, "failed to fetch replica set status metrics", failed, errs)
		return
	}

	members, ok := status[replicaSetMembersKey].(bson.A)
	if !ok {
		errs.AddPartial(failed, fmt.Errorf("failed to fetch replica set members: expected %T, got %T", bson.A{}, status[replicaSetMembersKey]))
		return
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbReplicaSetMemberCount.Enabled {
		s.recordReplicaSetMemberCount(now, members)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbReplicaStatus.Enabled {
		s.recordReplicaStatus(now, members, errs)
	}

	// Lag and headroom are only reported by the primary. A secondary sees the other members
	// through heartbeats, so its view of their progress trails the primary's.
	self, ok := replicaSetSelf(members)
	if !ok || replicaSetMemberStates[getInt64Value(self, replicaSetMemberStateKey)] != metadata.AttributeMongodbReplicaStatePrimary {
		return
	}
	lags := replicaSetLags(members)

	if s.config.MetricsBuilderConfig.Metrics.MongodbReplicaSetLag.Enabled {
		s.recordReplicaSetLag(now, lags)
	}

	if hasOplogWindow && s.config.MetricsBuilderConfig.Metrics.MongodbReplicaSetHeadroom.Enabled {
		s.recordReplicaSetHeadroom(now, lags, oplogWindow)
	}
}

func (s *mongodbScraper) collectIndexStats(ctx context.Context, now pcommon.Timestamp, databaseName, collectionName string, errs *scrapererror.ScrapeErrors) {
	if databaseName == "local" {
		return
	}
	indexStats, err := s.client.IndexStats(ctx, databaseName, collectionName)
	if err != nil {
		var srvErr mongo.ServerError
		// ignore the error, $indexStats is not supported on views
		if errors.As(err, &srvErr) && srvErr.HasErrorCode(40602) {
			s.logger.Debug("skipping unsupported index stats for view",
				zap.String("database", databaseName), zap.String("view", collectionName))
			return
		}
		errs.AddPartial(1, fmt.Errorf("failed to fetch index stats metrics: %w", err))
		return
	}
	s.recordIndexStats(now, indexStats, databaseName, collectionName, errs)
}

func (s *mongodbScraper) recordDBStats(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	if s.config.MetricsBuilderConfig.Metrics.MongodbCollectionCount.Enabled {
		s.recordCollections(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbDataSize.Enabled {
		s.recordDataSize(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbExtentCount.Enabled {
		s.recordExtentCount(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbIndexSize.Enabled {
		s.recordIndexSize(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbIndexCount.Enabled {
		s.recordIndexCount(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbObjectCount.Enabled {
		s.recordObjectCount(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbStorageSize.Enabled {
		s.recordStorageSize(now, doc, dbName, errs)
	}
}

func (s *mongodbScraper) recordNormalServerStats(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	if s.config.MetricsBuilderConfig.Metrics.MongodbConnectionCount.Enabled {
		s.recordConnections(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbDocumentOperationCount.Enabled {
		s.recordDocumentOperations(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbMemoryUsage.Enabled {
		s.recordMemoryUsage(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbLockAcquireCount.Enabled {
		s.recordLockAcquireCounts(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbLockAcquireWaitCount.Enabled {
		s.recordLockAcquireWaitCounts(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbLockAcquireTime.Enabled {
		s.recordLockTimeAcquiringMicros(now, doc, dbName, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbLockDeadlockCount.Enabled {
		s.recordLockDeadlockCount(now, doc, dbName, errs)
	}
}

func (s *mongodbScraper) recordAdminStats(now pcommon.Timestamp, document bson.M, errs *scrapererror.ScrapeErrors) {
	if s.config.MetricsBuilderConfig.Metrics.MongodbCacheOperations.Enabled {
		s.recordCacheOperations(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbCursorCount.Enabled {
		s.recordCursorCount(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbCursorTimeoutCount.Enabled {
		s.recordCursorTimeoutCount(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbGlobalLockTime.Enabled {
		s.recordGlobalLockTime(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbAssertCount.Enabled {
		s.recordAsserts(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbGlobalLockQueueCount.Enabled {
		s.recordGlobalLockQueue(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbWriteConcernWaitTime.Enabled {
		s.recordWriteConcernWaitTime(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbNetworkRequestCount.Enabled {
		s.recordNetworkCount(now, document, errs)
	}

	s.recordOperations(now, document, errs)

	s.recordOperationsRepl(now, document, errs)

	if s.config.MetricsBuilderConfig.Metrics.MongodbSessionCount.Enabled {
		s.recordSessionCount(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbOperationLatencyTime.Enabled {
		s.recordLatencyTime(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbOperationLatencyCount.Enabled {
		s.recordLatencyCount(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbUptime.Enabled {
		s.recordUptime(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbHealth.Enabled {
		s.recordHealth(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbActiveWrites.Enabled {
		s.recordActiveWrites(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbActiveReads.Enabled {
		s.recordActiveReads(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbFlushesRate.Enabled {
		s.recordFlushesPerSecond(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbWtcacheBytesRead.Enabled {
		s.recordWTCacheBytes(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbWtLogWrite.Enabled {
		s.recordWTLogWrite(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbWtLogOperationCount.Enabled {
		s.recordWTLogOperations(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbWtLogSyncTime.Enabled {
		s.recordWTLogSyncTime(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbWtFsyncCount.Enabled {
		s.recordWTFsyncCount(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbWtConcurrentTransactionTicketInUse.Enabled {
		s.recordWTConcurrentTransactionsOut(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbQueryExecutorIndexKeyScannedCount.Enabled {
		s.recordQueryExecutorIndexKeysScanned(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbQueryExecutorDocumentScannedCount.Enabled {
		s.recordQueryExecutorDocumentsScanned(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbQueryExecutorCollectionScanCount.Enabled {
		s.recordQueryExecutorCollectionScans(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbQueryExecutorCollectionScanNonTailableCount.Enabled {
		s.recordQueryExecutorNonTailableCollectionScans(now, document, errs)
	}

	if s.config.MetricsBuilderConfig.Metrics.MongodbPageFaults.Enabled {
		s.recordPageFaults(now, document, errs)
	}
}

func (s *mongodbScraper) recordIndexStats(now pcommon.Timestamp, indexStats []bson.M, databaseName, collectionName string, errs *scrapererror.ScrapeErrors) {
	s.recordIndexAccess(now, indexStats, databaseName, collectionName, errs)
}

// discoverMemberHosts runs findMemberHosts within the configured timeout, so a deployment with no
// reachable primary delays start by at most timeout rather than the driver's server selection default.
func (s *mongodbScraper) discoverMemberHosts(ctx context.Context) ([]string, error) {
	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}
	return s.findMemberHosts(ctx)
}

// errNoReplicaSetConfig is returned when a node reports a replica set name but no usable member
// list, which MongoDB does for a member removed from the set or not yet initiated.
var errNoReplicaSetConfig = errors.New("node reports a replica set but no usable member list")

// findMemberHosts returns every data-bearing member of the replica set, including the node the
// main client is connected to, as hello reports them. hello is used rather than replSetGetStatus
// because it reports members under the replica set horizon that matches the name the client
// connected with, which is how a deployment behind a private endpoint or other address translation
// makes its members reachable; replSetGetStatus always reports the internal names. Arbiters hold no
// data and hidden members are not advertised, so neither is returned. A standalone server or a
// mongos reports no set name and has no members to discover.
func (s *mongodbScraper) findMemberHosts(ctx context.Context) ([]string, error) {
	result, err := s.client.RunCommand(ctx, adminDatabase, bson.M{"hello": 1})
	if err != nil {
		return nil, fmt.Errorf("failed to run hello: %w", err)
	}
	if _, ok := result["setName"]; !ok {
		return nil, nil
	}

	self, _ := result["me"].(string)
	electable, ok := result["hosts"].(bson.A)
	if self == "" || !ok || len(electable) == 0 {
		return nil, errNoReplicaSetConfig
	}

	var hosts []string
	seen := map[string]struct{}{}
	add := func(host string) {
		key := strings.ToLower(host)
		if _, dup := seen[key]; host == "" || dup {
			return
		}
		seen[key] = struct{}{}
		s.logger.Debug("Found replica set member", zap.String("host", host))
		hosts = append(hosts, host)
	}
	add(self)
	for _, key := range []string{"hosts", "passives"} {
		list, _ := result[key].(bson.A)
		for _, v := range list {
			host, _ := v.(string)
			add(host)
		}
	}
	return hosts, nil
}
