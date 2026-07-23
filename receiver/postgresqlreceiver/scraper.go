// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/priorityqueue"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

const (
	readmeURL                 = "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.88.0/receiver/postgresqlreceiver/README.md"
	defaultPostgreSQLDatabase = "postgres"

	defaultServiceName = "unknown_service:postgresql"
)

// otelNamespaceUUID is the official OTel namespace UUID for deterministic UUID v5 generation,
// as recommended by the semantic conventions for service.instance.id.
// See: https://opentelemetry.io/docs/specs/semconv/registry/attributes/service/
var otelNamespaceUUID = uuid.MustParse("4d63009a-8d0f-11ee-aad7-4c796ed8e320")

type postgreSQLScraper struct {
	logger        *zap.Logger
	config        *Config
	clientFactory postgreSQLClientFactory
	mb            *metadata.MetricsBuilder
	lb            *metadata.LogsBuilder
	excludes      map[string]struct{}
	cache         *lru.Cache[string, float64]
	// if enabled, uses a separated attribute for the schema
	separateSchemaAttr     bool
	useOTelSemconv         bool
	queryPlanCache         *expirable.LRU[string, string]
	newestQueryTimestamp   float64
	serviceInstanceID      string
	lastExecutionTimestamp time.Time
}

type errsMux struct {
	sync.RWMutex
	errs scrapererror.ScrapeErrors
}

func (e *errsMux) add(err error) {
	e.Lock()
	defer e.Unlock()
	e.errs.Add(err)
}

func (e *errsMux) addPartial(err error) {
	e.Lock()
	defer e.Unlock()
	e.errs.AddPartial(1, err)
}

func (e *errsMux) combine() error {
	e.Lock()
	defer e.Unlock()
	return e.errs.Combine()
}

func newPostgreSQLScraper(
	settings receiver.Settings,
	config *Config,
	clientFactory postgreSQLClientFactory,
	cache *lru.Cache[string, float64],
	queryPlanCache *expirable.LRU[string, string],
) (*postgreSQLScraper, error) {
	excludes := make(map[string]struct{})
	for _, db := range config.ExcludeDatabases {
		excludes[db] = struct{}{}
	}
	separateSchemaAttr := metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate.IsEnabled()
	useOTelSemconv := metadata.ReceiverPostgresqlUseOTelSemconvFeatureGate.IsEnabled()

	if separateSchemaAttr && useOTelSemconv {
		return nil, fmt.Errorf("feature gates %s and %s are mutually exclusive and cannot both be enabled",
			metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate.ID(),
			metadata.ReceiverPostgresqlUseOTelSemconvFeatureGate.ID())
	}

	if !separateSchemaAttr && !useOTelSemconv {
		settings.Logger.Warn(
			fmt.Sprintf("Feature gate %s is not enabled. Please see the README for more information: %s", metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate.ID(), readmeURL),
		)
	}
	var serviceInstanceID string
	if useOTelSemconv {
		serviceInstanceID = uuid.NewSHA1(otelNamespaceUUID, []byte(resolveServiceInstanceSeed(config, settings.Logger))).String()
	} else {
		serviceInstanceID = getInstanceID(config.Endpoint, settings.Logger)
	}
	mbConfig := metricsBuilderConfigForFeatureGate(config.MetricsBuilderConfig, useOTelSemconv)
	return &postgreSQLScraper{
		logger:             settings.Logger,
		config:             config,
		clientFactory:      clientFactory,
		mb:                 metadata.NewMetricsBuilder(mbConfig, settings),
		lb:                 metadata.NewLogsBuilder(config.LogsBuilderConfig, settings),
		excludes:           excludes,
		cache:              cache,
		queryPlanCache:     queryPlanCache,
		separateSchemaAttr: separateSchemaAttr,
		serviceInstanceID:  serviceInstanceID,
		useOTelSemconv:     useOTelSemconv,
	}, nil
}

var semconvModeMetricAttributeNames = [...]string{
	string(semconv.DBNamespaceKey),
	string(semconv.DBCollectionNameKey),
	string(metadata.PostgresqlIndexScansMetricAttributeKeyPostgresqlIndexName),
}

func metricsBuilderConfigForFeatureGate(config metadata.MetricsBuilderConfig, useOTelSemconv bool) metadata.MetricsBuilderConfig {
	if useOTelSemconv {
		return config
	}

	metrics := &config.Metrics
	metrics.PostgresqlBackends.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlBackends.EnabledAttributes)
	metrics.PostgresqlBlksHit.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlBlksHit.EnabledAttributes)
	metrics.PostgresqlBlksRead.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlBlksRead.EnabledAttributes)
	metrics.PostgresqlBlocksRead.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlBlocksRead.EnabledAttributes)
	metrics.PostgresqlCommits.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlCommits.EnabledAttributes)
	metrics.PostgresqlDbSize.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlDbSize.EnabledAttributes)
	metrics.PostgresqlDeadlocks.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlDeadlocks.EnabledAttributes)
	metrics.PostgresqlFunctionCalls.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlFunctionCalls.EnabledAttributes)
	metrics.PostgresqlIndexScans.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlIndexScans.EnabledAttributes)
	metrics.PostgresqlIndexSize.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlIndexSize.EnabledAttributes)
	metrics.PostgresqlOperations.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlOperations.EnabledAttributes)
	metrics.PostgresqlQueryConflicts.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlQueryConflicts.EnabledAttributes)
	metrics.PostgresqlRollbacks.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlRollbacks.EnabledAttributes)
	metrics.PostgresqlRows.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlRows.EnabledAttributes)
	metrics.PostgresqlSequentialScans.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlSequentialScans.EnabledAttributes)
	metrics.PostgresqlTableCount.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTableCount.EnabledAttributes)
	metrics.PostgresqlTableSize.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTableSize.EnabledAttributes)
	metrics.PostgresqlTableVacuumCount.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTableVacuumCount.EnabledAttributes)
	metrics.PostgresqlTempIo.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTempIo.EnabledAttributes)
	metrics.PostgresqlTempFiles.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTempFiles.EnabledAttributes)
	metrics.PostgresqlTupDeleted.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTupDeleted.EnabledAttributes)
	metrics.PostgresqlTupFetched.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTupFetched.EnabledAttributes)
	metrics.PostgresqlTupInserted.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTupInserted.EnabledAttributes)
	metrics.PostgresqlTupReturned.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTupReturned.EnabledAttributes)
	metrics.PostgresqlTupUpdated.EnabledAttributes = legacyMetricAttributes(metrics.PostgresqlTupUpdated.EnabledAttributes)

	return config
}

func legacyMetricAttributes[T ~string](attributes []T) []T {
	filtered := slices.DeleteFunc(slices.Clone(attributes), func(attr T) bool {
		return slices.Contains(semconvModeMetricAttributeNames[:], string(attr))
	})
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

type dbRetrieval struct {
	sync.RWMutex
	activityMap     map[databaseName]int64
	dbSizeMap       map[databaseName]int64
	dbStats         map[databaseName]databaseStats
	dbConflictStats map[databaseName]databaseConflictStats
}

// scrape scrapes the metric stats, transforms them and attributes them into a metric slices.
func (p *postgreSQLScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	databases := p.config.Databases
	listClient, err := p.clientFactory.getClient(defaultPostgreSQLDatabase)
	if err != nil {
		p.logger.Error("Failed to initialize connection to postgres", zap.Error(err))
		return pmetric.NewMetrics(), err
	}
	defer listClient.Close()

	if len(databases) == 0 {
		dbList, dbErr := listClient.listDatabases(ctx)
		if dbErr != nil {
			p.logger.Error("Failed to request list of databases from postgres", zap.Error(dbErr))
			return pmetric.NewMetrics(), dbErr
		}
		databases = dbList
	}
	var filteredDatabases []string
	for _, db := range databases {
		if _, ok := p.excludes[db]; !ok {
			filteredDatabases = append(filteredDatabases, db)
		}
	}
	databases = filteredDatabases

	now := pcommon.NewTimestampFromTime(time.Now())

	var errs errsMux
	r := &dbRetrieval{
		activityMap:     make(map[databaseName]int64),
		dbSizeMap:       make(map[databaseName]int64),
		dbStats:         make(map[databaseName]databaseStats),
		dbConflictStats: make(map[databaseName]databaseConflictStats),
	}
	p.retrieveDBMetrics(ctx, listClient, databases, r, &errs)

	for _, database := range databases {
		dbClient, dbErr := p.clientFactory.getClient(database)
		if dbErr != nil {
			errs.add(dbErr)
			p.logger.Error("Failed to initialize connection to postgres", zap.String("database", database), zap.Error(dbErr))
			continue
		}
		defer dbClient.Close()
		numTables := p.collectTables(ctx, now, dbClient, database, &errs)

		p.recordDatabase(now, database, r, numTables)
		p.collectIndexes(ctx, now, dbClient, database, &errs)
		p.collectFunctions(ctx, now, dbClient, database, &errs)
		p.collectVectorStats(ctx, now, dbClient, database, &errs)
	}

	p.mb.RecordPostgresqlDatabaseCountDataPoint(now, int64(len(databases)))
	p.collectBGWriterStats(ctx, now, listClient, &errs)
	p.collectWalAge(ctx, now, listClient, &errs)
	p.collectReplicationStats(ctx, now, listClient, &errs)
	p.collectMaxConnections(ctx, now, listClient, &errs)
	p.collectDatabaseLocks(ctx, now, listClient, &errs)

	if p.useOTelSemconv {
		rb := p.setupSemconvResourceBuilder(p.mb.NewResourceBuilder())
		return p.mb.Emit(metadata.WithResource(rb.Emit())), errs.combine()
	}
	rb := p.setupLegacyResourceBuilder(p.mb.NewResourceBuilder(), "", "", "", "")
	return p.mb.Emit(metadata.WithResource(rb.Emit())), errs.combine()
}

func (p *postgreSQLScraper) scrapeQuerySamples(ctx context.Context, maxRowsPerQuery int64) (plog.Logs, error) {
	dbClient, err := p.clientFactory.getClient(defaultPostgreSQLDatabase)
	if err != nil {
		p.logger.Error("Failed to initialize connection to postgres", zap.Error(err))
		return plog.NewLogs(), err
	}

	var errs errsMux

	p.collectQuerySamples(ctx, dbClient, maxRowsPerQuery, &errs, p.logger)

	defer dbClient.Close()

	rb := p.setupLogsResourceBuilder(p.lb.NewResourceBuilder())
	return p.lb.Emit(metadata.WithLogsResource(rb.Emit())), nil
}

func (p *postgreSQLScraper) scrapeTopQuery(ctx context.Context, maxRowsPerQuery, topNQuery, maxExplainEachInterval int64, collectionInterval time.Duration) (plog.Logs, error) {
	var errs errsMux
	currentCollectionTime := time.Now()

	if p.isCollectionDue(currentCollectionTime, collectionInterval) {
		p.collectTopQuery(ctx, p.clientFactory, maxRowsPerQuery, topNQuery, maxExplainEachInterval, &errs, p.logger, currentCollectionTime)
		p.lastExecutionTimestamp = currentCollectionTime
	}

	rb := p.setupLogsResourceBuilder(p.lb.NewResourceBuilder())
	return p.lb.Emit(metadata.WithLogsResource(rb.Emit())), nil
}

func (p *postgreSQLScraper) isCollectionDue(collectionTime time.Time, interval time.Duration) bool {
	if p.lastExecutionTimestamp.IsZero() {
		// This is the first collection
		return true
	}

	if time.Duration(math.Ceil(collectionTime.Sub(p.lastExecutionTimestamp).Seconds()))*time.Second >= interval {
		return true
	}

	p.logger.Debug("Skipping the collection of top queries because collection interval has not yet elapsed." +
		"last collection time: " + p.lastExecutionTimestamp.String() + ", current collection time: " + collectionTime.String() +
		", collection interval: " + interval.String())
	return false
}

func attrString(atts map[string]any, key string) string {
	if v, ok := atts[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func attrInt64(atts map[string]any, key string) int64 {
	if v, ok := atts[key]; ok {
		if i, ok := v.(int64); ok {
			return i
		}
	}
	return 0
}

func attrFloat64(atts map[string]any, key string) float64 {
	if v, ok := atts[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

func (p *postgreSQLScraper) collectQuerySamples(ctx context.Context, dbClient client, limit int64, mux *errsMux, logger *zap.Logger) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	attributes, newestQueryTimestamp, err := dbClient.getQuerySamples(ctx, limit, p.newestQueryTimestamp, logger)
	p.newestQueryTimestamp = newestQueryTimestamp
	if err != nil {
		mux.addPartial(err)
		return
	}
	for _, atts := range attributes {
		// Use a background context so query-sample logs are not automatically linked to the scrape context.
		logCtx := context.Background()
		if ctxFromQuery, ok := atts[querySampleTraceContextKey]; ok {
			if ctx, ok := ctxFromQuery.(context.Context); ok {
				logCtx = ctx
			}
		}
		p.lb.RecordDbServerQuerySampleEvent(logCtx,
			timestamp,
			metadata.AttributeDbSystemNamePostgresql,
			attrString(atts, string(semconv.DBNamespaceKey)),
			attrString(atts, string(semconv.DBQueryTextKey)),
			attrString(atts, string(semconv.UserNameKey)),
			attrString(atts, dbAttributePrefix+querySampleColumnState),
			attrInt64(atts, dbAttributePrefix+querySampleColumnPID),
			attrString(atts, dbAttributePrefix+querySampleColumnApplicationName),
			attrString(atts, string(semconv.NetworkPeerAddressKey)),
			attrInt64(atts, string(semconv.NetworkPeerPortKey)),
			attrString(atts, dbAttributePrefix+querySampleColumnClientHostname),
			attrString(atts, dbAttributePrefix+querySampleColumnQueryStart),
			attrString(atts, dbAttributePrefix+querySampleColumnWaitEvent),
			attrString(atts, dbAttributePrefix+querySampleColumnWaitEventType),
			attrString(atts, dbAttributePrefix+querySampleColumnQueryID),
			attrFloat64(atts, postgresqlTotalExecTimeAttributeName),
			attrString(atts, dbAttributePrefix+querySampleColumnBlockingPIDs),
			attrString(atts, dbAttributePrefix+querySampleColumnBlockingStartTime),
			attrInt64(atts, dbAttributePrefix+querySampleColumnBlockingWaitDuration),
			attrString(atts, dbAttributePrefix+querySampleColumnBlockingLockMode),
			attrString(atts, dbAttributePrefix+querySampleColumnBlockingLockType),
			attrString(atts, dbAttributePrefix+querySampleColumnBlockingLockRelation),
			attrString(atts, dbAttributePrefix+querySampleColumnBlockingTxnStartTime),
		)
	}
}

func (p *postgreSQLScraper) collectTopQuery(ctx context.Context, clientFactory postgreSQLClientFactory, limit, topNQuery, maxExplainEachInterval int64, mux *errsMux, logger *zap.Logger, collectionTime time.Time) {
	timestamp := pcommon.NewTimestampFromTime(collectionTime)

	defaultDbClient, err := clientFactory.getClient(defaultPostgreSQLDatabase)
	if err != nil {
		logger.Error("failed to create db client for default postgresql database")
		mux.addPartial(err)
		return
	}

	defer defaultDbClient.Close()

	rows, err := defaultDbClient.getTopQuery(ctx, limit, logger)
	if err != nil {
		logger.Error("failed to get top query", zap.Error(err))
		mux.addPartial(err)
		return
	}

	type updatedOnlyInfo struct {
		finalConverter func(float64) any
	}

	convertToInt := func(f float64) any {
		return int64(f)
	}

	updatedOnly := map[string]updatedOnlyInfo{
		totalExecTimeColumnName:     {},
		totalPlanTimeColumnName:     {},
		rowsColumnName:              {finalConverter: convertToInt},
		callsColumnName:             {finalConverter: convertToInt},
		sharedBlksDirtiedColumnName: {finalConverter: convertToInt},
		sharedBlksHitColumnName:     {finalConverter: convertToInt},
		sharedBlksReadColumnName:    {finalConverter: convertToInt},
		sharedBlksWrittenColumnName: {finalConverter: convertToInt},
		tempBlksReadColumnName:      {finalConverter: convertToInt},
		tempBlksWrittenColumnName:   {finalConverter: convertToInt},
	}

	pq := make(priorityqueue.PriorityQueue[map[string]any, float64], 0)

	for i, row := range rows {
		queryID := row[dbAttributePrefix+queryidColumnName]

		if queryID == nil {
			// this should not happen, but in case
			logger.Error("queryid is nil", zap.Any("atts", row))
			mux.addPartial(errors.New("queryid is nil"))
			continue
		}

		for columnName, info := range updatedOnly {
			var valInAtts float64
			_val := row[dbAttributePrefix+columnName]
			if i, ok := _val.(int64); ok {
				valInAtts = float64(i)
			} else {
				valInAtts = _val.(float64)
			}
			valInCache, exist := p.cache.Get(queryID.(string) + columnName)
			valDelta := valInAtts
			if exist {
				valDelta = valInAtts - valInCache
			}
			finalValue := float64(0)
			if valDelta > 0 {
				p.cache.Add(queryID.(string)+columnName, valInAtts)
				finalValue = valDelta
			}
			if info.finalConverter != nil {
				row[dbAttributePrefix+columnName] = info.finalConverter(finalValue)
			} else {
				row[dbAttributePrefix+columnName] = finalValue
			}
		}
		if row[dbAttributePrefix+totalExecTimeColumnName] == 0.0 {
			continue
		}
		item := priorityqueue.QueueItem[map[string]any, float64]{
			Value:    row,
			Priority: row[dbAttributePrefix+totalExecTimeColumnName].(float64),
			Index:    i,
		}
		pq.Push(&item)
	}

	heap.Init(&pq)
	explained := int64(0)
	count := 0
	for pq.Len() > 0 && count < int(topNQuery) {
		item := heap.Pop(&pq).(*priorityqueue.QueueItem[map[string]any, float64])
		query := item.Value[string(semconv.DBQueryTextKey)].(string)
		queryID := item.Value[dbAttributePrefix+queryidColumnName].(string)
		// Use raw query (with $1, $2 placeholders) for EXPLAIN, not the obfuscated one (with ?)
		rawQuery, _ := item.Value[dbAttributePrefix+"raw_query"].(string)
		plan, ok := p.queryPlanCache.Get(queryID + "-plan")
		if !ok && explained < maxExplainEachInterval {
			database := item.Value[string(semconv.DBNamespaceKey)].(string)
			dbClient, err := clientFactory.getClient(database)
			if err == nil {
				plan, err = dbClient.explainQuery(rawQuery, queryID, logger)
				if err != nil {
					logger.Error("failed to explain query", zap.String("query", rawQuery), zap.Error(err))
				}
				// to avoid flood the error message. there are some internal queries meant to not be
				// explained. we wait for the cache to expire and report the error again.
				p.queryPlanCache.Add(queryID+"-plan", plan)
				err = dbClient.Close()
				if err != nil {
					logger.Error("failed to close", zap.Error(err))
				}
			}
			explained++
		}

		p.lb.RecordDbServerTopQueryEvent(
			context.Background(),
			timestamp,
			metadata.AttributeDbSystemNamePostgresql,
			item.Value[string(semconv.DBNamespaceKey)].(string),
			query,
			item.Value[dbAttributePrefix+callsColumnName].(int64),
			item.Value[dbAttributePrefix+rowsColumnName].(int64),
			item.Value[dbAttributePrefix+sharedBlksDirtiedColumnName].(int64),
			item.Value[dbAttributePrefix+sharedBlksHitColumnName].(int64),
			item.Value[dbAttributePrefix+sharedBlksReadColumnName].(int64),
			item.Value[dbAttributePrefix+sharedBlksWrittenColumnName].(int64),
			item.Value[dbAttributePrefix+tempBlksReadColumnName].(int64),
			item.Value[dbAttributePrefix+tempBlksWrittenColumnName].(int64),
			queryID,
			item.Value[dbAttributePrefix+"rolname"].(string),
			item.Value[dbAttributePrefix+totalExecTimeColumnName].(float64),
			item.Value[dbAttributePrefix+totalPlanTimeColumnName].(float64),
			plan,
		)
		count++
	}
}

func (p *postgreSQLScraper) shutdown(_ context.Context) error {
	if p.clientFactory != nil {
		p.clientFactory.close()
	}
	return nil
}

func (p *postgreSQLScraper) retrieveDBMetrics(
	ctx context.Context,
	listClient client,
	databases []string,
	r *dbRetrieval,
	errs *errsMux,
) {
	wg := &sync.WaitGroup{}

	wg.Add(3)
	go p.retrieveBackends(ctx, wg, listClient, databases, r, errs)
	go p.retrieveDatabaseSize(ctx, wg, listClient, databases, r, errs)
	go p.retrieveDatabaseStats(ctx, wg, listClient, databases, r, errs)

	// pg_stat_database_conflicts is queried separately and only when the metric is
	// enabled, since the counters are only populated on standby servers and would
	// otherwise add an unnecessary query on every scrape.
	if p.config.Metrics.PostgresqlQueryConflicts.Enabled {
		wg.Add(1)
		go p.retrieveDatabaseConflicts(ctx, wg, listClient, databases, r, errs)
	}

	wg.Wait()
}

func (p *postgreSQLScraper) recordDatabase(now pcommon.Timestamp, db string, r *dbRetrieval, numTables int64) {
	dbName := databaseName(db)
	p.mb.RecordPostgresqlTableCountDataPoint(now, numTables, db)
	if activeConnections, ok := r.activityMap[dbName]; ok {
		p.mb.RecordPostgresqlBackendsDataPoint(now, activeConnections, db)
	}
	if size, ok := r.dbSizeMap[dbName]; ok {
		p.mb.RecordPostgresqlDbSizeDataPoint(now, size, db)
	}
	if stats, ok := r.dbStats[dbName]; ok {
		p.mb.RecordPostgresqlCommitsDataPoint(now, stats.transactionCommitted, db)
		p.mb.RecordPostgresqlRollbacksDataPoint(now, stats.transactionRollback, db)
		p.mb.RecordPostgresqlDeadlocksDataPoint(now, stats.deadlocks, db)
		p.mb.RecordPostgresqlTempFilesDataPoint(now, stats.tempFiles, db)
		p.mb.RecordPostgresqlTempIoDataPoint(now, stats.tempIo, db)
		p.mb.RecordPostgresqlTupUpdatedDataPoint(now, stats.tupUpdated, db)
		p.mb.RecordPostgresqlTupReturnedDataPoint(now, stats.tupReturned, db)
		p.mb.RecordPostgresqlTupFetchedDataPoint(now, stats.tupFetched, db)
		p.mb.RecordPostgresqlTupInsertedDataPoint(now, stats.tupInserted, db)
		p.mb.RecordPostgresqlTupDeletedDataPoint(now, stats.tupDeleted, db)
		p.mb.RecordPostgresqlBlksHitDataPoint(now, stats.blksHit, db)
		p.mb.RecordPostgresqlBlksReadDataPoint(now, stats.blksRead, db)
	}
	if conflicts, ok := r.dbConflictStats[dbName]; ok {
		p.mb.RecordPostgresqlQueryConflictsDataPoint(now, conflicts.conflTablespace, metadata.AttributePostgresqlConflictTypeTablespace, db)
		p.mb.RecordPostgresqlQueryConflictsDataPoint(now, conflicts.conflLock, metadata.AttributePostgresqlConflictTypeLock, db)
		p.mb.RecordPostgresqlQueryConflictsDataPoint(now, conflicts.conflSnapshot, metadata.AttributePostgresqlConflictTypeSnapshot, db)
		p.mb.RecordPostgresqlQueryConflictsDataPoint(now, conflicts.conflBufferpin, metadata.AttributePostgresqlConflictTypeBufferpin, db)
		p.mb.RecordPostgresqlQueryConflictsDataPoint(now, conflicts.conflDeadlock, metadata.AttributePostgresqlConflictTypeDeadlock, db)
	}

	if !p.useOTelSemconv {
		rb := p.setupLegacyResourceBuilder(p.mb.NewResourceBuilder(), db, "", "", "")
		p.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}

func formatNamespace(database, schema string) string {
	if schema == "" {
		return database
	}
	return database + "|" + schema
}

func (p *postgreSQLScraper) collectTables(ctx context.Context, now pcommon.Timestamp, dbClient client, db string, errs *errsMux) (numTables int64) {
	blockReads, err := dbClient.getBlocksReadByTable(ctx, db)
	if err != nil {
		errs.addPartial(err)
	}

	tableMetrics, err := dbClient.getDatabaseTableMetrics(ctx, db)
	if err != nil {
		errs.addPartial(err)
	}

	for tableKey, tm := range tableMetrics {
		namespace := formatNamespace(db, tm.schema)

		p.mb.RecordPostgresqlRowsDataPoint(now, tm.dead, metadata.AttributeStateDead, namespace, tm.table)
		p.mb.RecordPostgresqlRowsDataPoint(now, tm.live, metadata.AttributeStateLive, namespace, tm.table)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.inserts, metadata.AttributeOperationIns, namespace, tm.table)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.del, metadata.AttributeOperationDel, namespace, tm.table)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.upd, metadata.AttributeOperationUpd, namespace, tm.table)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.hotUpd, metadata.AttributeOperationHotUpd, namespace, tm.table)
		p.mb.RecordPostgresqlTableSizeDataPoint(now, tm.size, namespace, tm.table)
		p.mb.RecordPostgresqlTableVacuumCountDataPoint(now, tm.vacuumCount, namespace, tm.table)
		p.mb.RecordPostgresqlSequentialScansDataPoint(now, tm.seqScans, namespace, tm.table)

		br, ok := blockReads[tableKey]
		if ok {
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.heapRead, metadata.AttributeSourceHeapRead, namespace, tm.table)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.heapHit, metadata.AttributeSourceHeapHit, namespace, tm.table)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.idxRead, metadata.AttributeSourceIdxRead, namespace, tm.table)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.idxHit, metadata.AttributeSourceIdxHit, namespace, tm.table)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.toastHit, metadata.AttributeSourceToastHit, namespace, tm.table)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.toastRead, metadata.AttributeSourceToastRead, namespace, tm.table)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.tidxRead, metadata.AttributeSourceTidxRead, namespace, tm.table)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.tidxHit, metadata.AttributeSourceTidxHit, namespace, tm.table)
		}

		if !p.useOTelSemconv {
			var schemaName string
			var tableName string
			if p.separateSchemaAttr {
				schemaName = tm.schema
				tableName = tm.table
			} else {
				tableName = fmt.Sprintf("%s.%s", tm.schema, tm.table)
			}
			rb := p.setupLegacyResourceBuilder(p.mb.NewResourceBuilder(), db, schemaName, tableName, "")
			p.mb.EmitForResource(metadata.WithResource(rb.Emit()))
		}
	}
	return int64(len(tableMetrics))
}

func (p *postgreSQLScraper) collectIndexes(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	database string,
	errs *errsMux,
) {
	idxStats, err := client.getIndexStats(ctx, database)
	if err != nil {
		errs.addPartial(err)
		return
	}

	for _, stat := range idxStats {
		namespace := formatNamespace(database, stat.schema)

		p.mb.RecordPostgresqlIndexScansDataPoint(now, stat.scans, namespace, stat.table, stat.index)
		p.mb.RecordPostgresqlIndexSizeDataPoint(now, stat.size, namespace, stat.table, stat.index)

		if !p.useOTelSemconv {
			var schemaName string
			if p.separateSchemaAttr {
				schemaName = stat.schema
			}
			rb := p.setupLegacyResourceBuilder(p.mb.NewResourceBuilder(), database, schemaName, stat.table, stat.index)
			p.mb.EmitForResource(metadata.WithResource(rb.Emit()))
		}
	}
}

func (p *postgreSQLScraper) collectFunctions(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	database string,
	errs *errsMux,
) {
	funcStats, err := client.getFunctionStats(ctx, database)
	if err != nil {
		errs.addPartial(err)
		return
	}

	for _, stat := range funcStats {
		namespace := formatNamespace(database, stat.schema)

		p.mb.RecordPostgresqlFunctionCallsDataPoint(now, stat.calls, stat.function, namespace)

		if !p.useOTelSemconv {
			var schemaName string
			if p.separateSchemaAttr {
				schemaName = stat.schema
			}
			rb := p.setupLegacyResourceBuilder(p.mb.NewResourceBuilder(), database, schemaName, "", "")
			p.mb.EmitForResource(metadata.WithResource(rb.Emit()))
		}
	}
}

// collectVectorStats collects the pgvector search and insert metrics. Search and insert stats are
// gathered by dedicated helpers that record datapoints carrying the db.namespace attribute. In the
// legacy per-entity resource model the search and insert metrics share the same database-level
// resource, so this is the one place that builds that resource and emits them together; in OTel
// semconv mode they are flushed with the single server-level resource at the end of scrape.
func (p *postgreSQLScraper) collectVectorStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	database string,
	errs *errsMux,
) {
	searchRecorded := p.recordVectorSearchStats(ctx, now, client, database, errs)
	insertRecorded := p.recordVectorInsertStats(ctx, now, client, database, errs)

	if p.useOTelSemconv || (!searchRecorded && !insertRecorded) {
		return
	}

	rb := p.setupLegacyResourceBuilder(p.mb.NewResourceBuilder(), database, "", "", "")
	p.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

// recordVectorSearchStats records the vector search metrics into the metrics builder and reports
// whether any datapoints were recorded. It does not emit a ResourceMetrics itself; collectVectorStats
// performs the consolidated emit.
func (p *postgreSQLScraper) recordVectorSearchStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	database string,
	errs *errsMux,
) bool {
	// All metrics are opt-in and derived from the same pg_stat_statements query, so skip
	// the collection entirely unless at least one of them is enabled.
	if !p.config.Metrics.PostgresqlVectorSearchCalls.Enabled &&
		!p.config.Metrics.PostgresqlVectorSearchDuration.Enabled &&
		!p.config.Metrics.PostgresqlVectorSearchRowsReturned.Enabled {
		return false
	}

	stats, err := client.getVectorSearchStats(ctx)
	if err != nil {
		errs.addPartial(err)
		return false
	}

	var recorded bool
	for _, stat := range stats {
		distanceFunction, ok := metadata.MapAttributePostgresqlDistanceFunctionName[stat.distanceFunction]
		if !ok {
			// A statement matched the vector filter but could not be classified into a known
			// distance function; skip it rather than emitting an invalid attribute value.
			p.logger.Debug("skipping unclassified vector distance function", zap.String("postgresql.distance.function.name", stat.distanceFunction))
			continue
		}
		p.mb.RecordPostgresqlVectorSearchCallsDataPoint(now, stat.calls, distanceFunction, database)
		p.mb.RecordPostgresqlVectorSearchDurationDataPoint(now, stat.totalExecTime, distanceFunction, database)
		p.mb.RecordPostgresqlVectorSearchRowsReturnedDataPoint(now, stat.rowsReturned, distanceFunction, database)
		recorded = true
	}

	return recorded
}

// recordVectorInsertStats records the vector insert metrics into the metrics builder and reports
// whether any datapoints were recorded. Like recordVectorSearchStats it does not emit a
// ResourceMetrics itself; collectVectorStats performs the consolidated emit.
func (p *postgreSQLScraper) recordVectorInsertStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	database string,
	errs *errsMux,
) bool {
	// Both metrics are opt-in and derived from the same pg_stat_statements query, so skip
	// the collection entirely unless at least one of them is enabled.
	if !p.config.Metrics.PostgresqlVectorInsertRows.Enabled &&
		!p.config.Metrics.PostgresqlVectorInsertDuration.Enabled {
		return false
	}

	stats, err := client.getVectorInsertStats(ctx)
	if err != nil {
		errs.addPartial(err)
		return false
	}

	var recorded bool
	for _, stat := range stats {
		p.mb.RecordPostgresqlVectorInsertRowsDataPoint(now, stat.rows, database)
		p.mb.RecordPostgresqlVectorInsertDurationDataPoint(now, stat.totalExecTime, database)
		recorded = true
	}

	return recorded
}

func (p *postgreSQLScraper) collectBGWriterStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errs *errsMux,
) {
	bgStats, err := client.getBGWriterStats(ctx)
	if err != nil {
		errs.addPartial(err)
		return
	}

	p.mb.RecordPostgresqlBgwriterBuffersAllocatedDataPoint(now, bgStats.buffersAllocated)

	p.mb.RecordPostgresqlBgwriterBuffersWritesDataPoint(now, bgStats.bgWrites, metadata.AttributeBgBufferSourceBgwriter)
	if bgStats.bufferBackendWrites >= 0 {
		p.mb.RecordPostgresqlBgwriterBuffersWritesDataPoint(now, bgStats.bufferBackendWrites, metadata.AttributeBgBufferSourceBackend)
	}
	p.mb.RecordPostgresqlBgwriterBuffersWritesDataPoint(now, bgStats.bufferCheckpoints, metadata.AttributeBgBufferSourceCheckpoints)
	if bgStats.bufferFsyncWrites >= 0 {
		p.mb.RecordPostgresqlBgwriterBuffersWritesDataPoint(now, bgStats.bufferFsyncWrites, metadata.AttributeBgBufferSourceBackendFsync)
	}

	p.mb.RecordPostgresqlBgwriterCheckpointCountDataPoint(now, bgStats.checkpointsReq, metadata.AttributeBgCheckpointTypeRequested)
	p.mb.RecordPostgresqlBgwriterCheckpointCountDataPoint(now, bgStats.checkpointsScheduled, metadata.AttributeBgCheckpointTypeScheduled)

	p.mb.RecordPostgresqlBgwriterDurationDataPoint(now, bgStats.checkpointSyncTime, metadata.AttributeBgDurationTypeSync)
	p.mb.RecordPostgresqlBgwriterDurationDataPoint(now, bgStats.checkpointWriteTime, metadata.AttributeBgDurationTypeWrite)

	p.mb.RecordPostgresqlBgwriterMaxwrittenDataPoint(now, bgStats.maxWritten)
}

func (p *postgreSQLScraper) collectDatabaseLocks(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errs *errsMux,
) {
	dbLocks, err := client.getDatabaseLocks(ctx)
	if err != nil {
		p.logger.Error("Errors encountered while fetching database locks", zap.Error(err))
		errs.addPartial(err)
		return
	}
	for _, dbLock := range dbLocks {
		p.mb.RecordPostgresqlDatabaseLocksDataPoint(now, dbLock.locks, dbLock.relation, dbLock.mode, dbLock.lockType)
	}
}

func (p *postgreSQLScraper) collectMaxConnections(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errs *errsMux,
) {
	mc, err := client.getMaxConnections(ctx)
	if err != nil {
		errs.addPartial(err)
		return
	}
	p.mb.RecordPostgresqlConnectionMaxDataPoint(now, mc)
}

func (p *postgreSQLScraper) collectReplicationStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errs *errsMux,
) {
	rss, err := client.getReplicationStats(ctx)
	if err != nil {
		errs.addPartial(err)
		return
	}
	for _, rs := range rss {
		if rs.pendingBytes >= 0 {
			p.mb.RecordPostgresqlReplicationDataDelayDataPoint(now, rs.pendingBytes, rs.clientAddr)
		}
		if metadata.PostgresqlreceiverPreciselagmetricsFeatureGate.IsEnabled() {
			if rs.writeLag >= 0 {
				p.mb.RecordPostgresqlWalDelayDataPoint(now, rs.writeLag, metadata.AttributeWalOperationLagWrite, rs.clientAddr)
			}
			if rs.replayLag >= 0 {
				p.mb.RecordPostgresqlWalDelayDataPoint(now, rs.replayLag, metadata.AttributeWalOperationLagReplay, rs.clientAddr)
			}
			if rs.flushLag >= 0 {
				p.mb.RecordPostgresqlWalDelayDataPoint(now, rs.flushLag, metadata.AttributeWalOperationLagFlush, rs.clientAddr)
			}
		} else {
			if rs.writeLagInt >= 0 {
				p.mb.RecordPostgresqlWalLagDataPoint(now, rs.writeLagInt, metadata.AttributeWalOperationLagWrite, rs.clientAddr)
			}
			if rs.replayLagInt >= 0 {
				p.mb.RecordPostgresqlWalLagDataPoint(now, rs.replayLagInt, metadata.AttributeWalOperationLagReplay, rs.clientAddr)
			}
			if rs.flushLagInt >= 0 {
				p.mb.RecordPostgresqlWalLagDataPoint(now, rs.flushLagInt, metadata.AttributeWalOperationLagFlush, rs.clientAddr)
			}
		}
	}
}

func (p *postgreSQLScraper) collectWalAge(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errs *errsMux,
) {
	walAge, err := client.getLatestWalAgeSeconds(ctx)
	if errors.Is(err, errNoLastArchive) {
		// return no error as there is no last archive to derive the value from
		return
	}
	if err != nil {
		errs.addPartial(fmt.Errorf("unable to determine latest WAL age: %w", err))
		return
	}
	p.mb.RecordPostgresqlWalAgeDataPoint(now, walAge)
}

func (p *postgreSQLScraper) retrieveDatabaseStats(
	ctx context.Context,
	wg *sync.WaitGroup,
	client client,
	databases []string,
	r *dbRetrieval,
	errs *errsMux,
) {
	defer wg.Done()
	dbStats, err := client.getDatabaseStats(ctx, databases)
	if err != nil {
		p.logger.Error("Errors encountered while fetching commits and rollbacks", zap.Error(err))
		errs.addPartial(err)
		return
	}
	r.Lock()
	r.dbStats = dbStats
	r.Unlock()
}

func (p *postgreSQLScraper) retrieveDatabaseConflicts(
	ctx context.Context,
	wg *sync.WaitGroup,
	client client,
	databases []string,
	r *dbRetrieval,
	errs *errsMux,
) {
	defer wg.Done()
	dbConflictStats, err := client.getDatabaseConflicts(ctx, databases)
	if err != nil {
		p.logger.Error("Errors encountered while fetching database recovery conflicts", zap.Error(err))
		errs.addPartial(err)
		return
	}
	r.Lock()
	r.dbConflictStats = dbConflictStats
	r.Unlock()
}

func (p *postgreSQLScraper) retrieveDatabaseSize(
	ctx context.Context,
	wg *sync.WaitGroup,
	client client,
	databases []string,
	r *dbRetrieval,
	errs *errsMux,
) {
	defer wg.Done()
	databaseSizeMetrics, err := client.getDatabaseSize(ctx, databases)
	if err != nil {
		p.logger.Error("Errors encountered while fetching database size", zap.Error(err))
		errs.addPartial(err)
		return
	}
	r.Lock()
	r.dbSizeMap = databaseSizeMetrics
	r.Unlock()
}

func (*postgreSQLScraper) retrieveBackends(
	ctx context.Context,
	wg *sync.WaitGroup,
	client client,
	databases []string,
	r *dbRetrieval,
	errs *errsMux,
) {
	defer wg.Done()
	activityByDB, err := client.getBackends(ctx, databases)
	if err != nil {
		errs.addPartial(err)
		return
	}
	r.Lock()
	r.activityMap = activityByDB
	r.Unlock()
}

// setupSemconvResourceBuilder sets service defaults, server.address, server.port, and UUID v5 service.instance.id.
func (p *postgreSQLScraper) setupSemconvResourceBuilder(rb *metadata.ResourceBuilder) *metadata.ResourceBuilder {
	rb.SetServiceName(defaultServiceName)
	rb.SetServiceNamespace("")
	if address, port, err := serverEndpointAttributes(p.config); err == nil {
		rb.SetServerAddress(address)
		rb.SetServerPort(port)
	}
	rb.SetServiceInstanceID(p.serviceInstanceID)
	return rb
}

func serverEndpointAttributes(config *Config) (string, int64, error) {
	host, portString, err := net.SplitHostPort(config.Endpoint)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.ParseInt(portString, 10, 64)
	if err != nil {
		return "", 0, err
	}
	if config.Transport == confignet.TransportTypeUnix {
		host = path.Join("/", host, ".s.PGSQL."+portString)
	}
	return host, port, nil
}

// setupLegacyResourceBuilder sets legacy per-entity resource attributes and host:port service.instance.id.
func (p *postgreSQLScraper) setupLegacyResourceBuilder(rb *metadata.ResourceBuilder, database, schema, table, index string) *metadata.ResourceBuilder {
	rb.SetServiceInstanceID(p.serviceInstanceID)
	rb.SetServiceName(defaultServiceName)
	rb.SetServiceNamespace("")
	if database != "" {
		rb.SetPostgresqlDatabaseName(database)
	}
	if schema != "" {
		rb.SetPostgresqlSchemaName(schema)
	}
	if table != "" {
		rb.SetPostgresqlTableName(table)
	}
	if index != "" {
		rb.SetPostgresqlIndexName(index)
	}
	return rb
}

// setupLogsResourceBuilder sets resource attributes for logs.
func (p *postgreSQLScraper) setupLogsResourceBuilder(rb *metadata.ResourceBuilder) *metadata.ResourceBuilder {
	if p.useOTelSemconv {
		return p.setupSemconvResourceBuilder(rb)
	}
	return p.setupLegacyResourceBuilder(rb, "", "", "", "")
}

// resolveServiceInstanceSeed returns the database endpoint used as the UUID v5 seed in semconv mode.
// Local TCP and Unix endpoints include the machine hostname so databases on different machines produce distinct IDs.
func resolveServiceInstanceSeed(config *Config, logger *zap.Logger) string {
	endpoint := config.Endpoint
	if config.Transport == confignet.TransportTypeUnix {
		address, _, err := serverEndpointAttributes(config)
		if err != nil {
			logger.Warn("Failed to parse Unix endpoint for service.instance.id; using raw endpoint in UUID seed",
				zap.String("endpoint", endpoint),
				zap.Error(err))
			address = endpoint
		}

		hostname, err := os.Hostname()
		if err != nil {
			logger.Warn("Failed to resolve hostname for service.instance.id; UUID may not be unique for identical sockets on different machines",
				zap.String("endpoint", endpoint),
				zap.Error(err))
			hostname = ""
		}
		return strings.Join([]string{"unix", hostname, address}, "\x00")
	}

	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		logger.Warn("Failed to parse endpoint for service.instance.id; using raw endpoint as UUID seed",
			zap.String("endpoint", endpoint),
			zap.Error(err))
		return endpoint
	}

	parsedIP := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (parsedIP == nil || !parsedIP.IsLoopback()) {
		return endpoint
	}

	hostname, err := os.Hostname()
	if err != nil {
		logger.Warn("Failed to resolve hostname for service.instance.id; UUID may not be unique for co-hosted receivers on different machines",
			zap.String("endpoint", endpoint),
			zap.Error(err))
		return endpoint
	}
	return net.JoinHostPort(hostname, port)
}

func getInstanceID(instanceString string, logger *zap.Logger) string {
	const fallback = "unknown:5432"
	host, port, err := net.SplitHostPort(instanceString)
	if err != nil {
		logger.Warn("Unable to determine actual instance ID for constructing service.instance.id", zap.Error(err))
		return fallback
	}

	if strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback() {
		localhost, hostNameErr := os.Hostname()
		if hostNameErr != nil {
			logger.Warn("Failed getting localhost machine name to construct service.instance.id.")
		} else {
			host = localhost
		}
	}
	return host + ":" + port
}
