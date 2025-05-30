// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

const (
	readmeURL            = "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.88.0/receiver/postgresqlreceiver/README.md"
	separateSchemaAttrID = "receiver.postgresql.separateSchemaAttr"

	defaultPostgreSQLDatabase = "postgres"
)

var separateSchemaAttrGate = featuregate.GlobalRegistry().MustRegister(
	separateSchemaAttrID,
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Moves Schema Names into dedicated Attribute"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/29559"),
)

type postgreSQLScraper struct {
	logger        *zap.Logger
	config        *Config
	clientFactory postgreSQLClientFactory
	mb            *metadata.MetricsBuilder
	excludes      map[string]struct{}
	cache         *lru.Cache[string, float64]
	// if enabled, uses a separated attribute for the schema
	separateSchemaAttr bool
	queryPlanCache     *expirable.LRU[string, string]
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
) *postgreSQLScraper {
	excludes := make(map[string]struct{})
	for _, db := range config.ExcludeDatabases {
		excludes[db] = struct{}{}
	}
	separateSchemaAttr := separateSchemaAttrGate.IsEnabled()

	if !separateSchemaAttr {
		settings.Logger.Warn(
			fmt.Sprintf("Feature gate %s is not enabled. Please see the README for more information: %s", separateSchemaAttrID, readmeURL),
		)
	}

	return &postgreSQLScraper{
		logger:             settings.Logger,
		config:             config,
		clientFactory:      clientFactory,
		mb:                 metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		excludes:           excludes,
		cache:              cache,
		queryPlanCache:     queryPlanCache,
		separateSchemaAttr: separateSchemaAttr,
	}
}

type dbRetrieval struct {
	sync.RWMutex
	activityMap map[databaseName]int64
	dbSizeMap   map[databaseName]int64
	dbStats     map[databaseName]databaseStats
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
		activityMap: make(map[databaseName]int64),
		dbSizeMap:   make(map[databaseName]int64),
		dbStats:     make(map[databaseName]databaseStats),
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
	}

	p.mb.RecordPostgresqlDatabaseCountDataPoint(now, int64(len(databases)))
	p.collectBGWriterStats(ctx, now, listClient, &errs)
	p.collectWalAge(ctx, now, listClient, &errs)
	p.collectReplicationStats(ctx, now, listClient, &errs)
	p.collectMaxConnections(ctx, now, listClient, &errs)
	p.collectDatabaseLocks(ctx, now, listClient, &errs)

	return p.mb.Emit(), errs.combine()
}

func (p *postgreSQLScraper) scrapeQuerySamples(ctx context.Context, maxRowsPerQuery int64) (plog.Logs, error) {
	logs := plog.NewLogs()
	resourceLog := logs.ResourceLogs().AppendEmpty()

	scopedLog := resourceLog.ScopeLogs().AppendEmpty()
	scopedLog.Scope().SetName(metadata.ScopeName)
	scopedLog.Scope().SetVersion("0.0.1")

	dbClient, err := p.clientFactory.getClient(defaultPostgreSQLDatabase)
	if err != nil {
		p.logger.Error("Failed to initialize connection to postgres", zap.Error(err))
		return logs, err
	}

	var errs errsMux

	logRecords := scopedLog.LogRecords()

	p.collectQuerySamples(ctx, dbClient, &logRecords, maxRowsPerQuery, &errs, p.logger)

	defer dbClient.Close()

	return logs, nil
}

func (p *postgreSQLScraper) scrapeTopQuery(ctx context.Context, maxRowsPerQuery int64, topNQuery int64, maxExplainEachInterval int64) (plog.Logs, error) {
	logs := plog.NewLogs()
	resourceLog := logs.ResourceLogs().AppendEmpty()

	scopedLog := resourceLog.ScopeLogs().AppendEmpty()
	scopedLog.Scope().SetName(metadata.ScopeName)
	scopedLog.Scope().SetVersion("0.0.1")

	var errs errsMux

	logRecords := scopedLog.LogRecords()

	p.collectTopQuery(ctx, p.clientFactory, &logRecords, maxRowsPerQuery, topNQuery, maxExplainEachInterval, &errs, p.logger)

	return logs, nil
}

func (p *postgreSQLScraper) collectQuerySamples(ctx context.Context, dbClient client, logRecords *plog.LogRecordSlice, limit int64, mux *errsMux, logger *zap.Logger) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	attributes, err := dbClient.getQuerySamples(ctx, limit, logger)
	if err != nil {
		mux.addPartial(err)
		return
	}
	for _, atts := range attributes {
		record := logRecords.AppendEmpty()
		record.SetTimestamp(timestamp)
		record.SetEventName("query sample")
		if err := record.Attributes().FromRaw(atts); err != nil {
			mux.addPartial(err)
			logger.Error("failed to read attributes from row", zap.Error(err))
		}
		record.Body().SetStr("sample")
	}
}

func (p *postgreSQLScraper) collectTopQuery(ctx context.Context, clientFactory postgreSQLClientFactory, logRecords *plog.LogRecordSlice, limit int64, topNQuery int64, maxExplainEachInterval int64, mux *errsMux, logger *zap.Logger) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())

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

	pq := make(priorityQueue, 0)

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
				p.cache.Add(queryID.(string)+columnName, valDelta)
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
		item := item{
			row:      row,
			priority: row[dbAttributePrefix+totalExecTimeColumnName].(float64),
			index:    i,
		}
		pq.Push(&item)
	}

	heap.Init(&pq)
	explained := int64(0)
	for pq.Len() > 0 && logRecords.Len() < int(topNQuery) {
		item := heap.Pop(&pq).(*item)
		record := logRecords.AppendEmpty()
		record.SetTimestamp(timestamp)
		record.SetEventName("top query")
		if err := record.Attributes().FromRaw(item.row); err != nil {
			mux.addPartial(err)
			logger.Error("failed to read attributes from row", zap.Error(err))
		}
		query := item.row[QueryTextAttributeName].(string)
		queryID := item.row[dbAttributePrefix+queryidColumnName].(string)
		plan, ok := p.queryPlanCache.Get(queryID + "-plan")
		if ok {
			record.Attributes().PutStr(dbAttributePrefix+"query_plan", plan)
		} else {
			if explained < maxExplainEachInterval {
				database := item.row[DatabaseAttributeName].(string)
				dbClient, err := clientFactory.getClient(database)
				if err != nil {
					record.Attributes().PutStr(dbAttributePrefix+"query_plan", "")
				} else {
					plan, err := dbClient.explainQuery(query, queryID, logger)
					if err != nil {
						logger.Error("failed to explain query", zap.String("query", query), zap.Error(err))
					}
					record.Attributes().PutStr(dbAttributePrefix+"query_plan", plan)
					// to avoid flood the error message. there are some internal queries meant to not be
					// explained. we wait for the cache to expire and report the error again.
					p.queryPlanCache.Add(queryID+"-plan", plan)
					err = dbClient.Close()
					if err != nil {
						logger.Error("failed to close", zap.Error(err))
					}
				}
				explained++
			} else {
				record.Attributes().PutStr(dbAttributePrefix+"query_plan", "")
			}
		}

		record.Attributes().PutStr("db.system.name", "postgresql")
		record.Body().SetStr("top query")
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

	wg.Wait()
}

func (p *postgreSQLScraper) recordDatabase(now pcommon.Timestamp, db string, r *dbRetrieval, numTables int64) {
	dbName := databaseName(db)
	p.mb.RecordPostgresqlTableCountDataPoint(now, numTables)
	if activeConnections, ok := r.activityMap[dbName]; ok {
		p.mb.RecordPostgresqlBackendsDataPoint(now, activeConnections)
	}
	if size, ok := r.dbSizeMap[dbName]; ok {
		p.mb.RecordPostgresqlDbSizeDataPoint(now, size)
	}
	if stats, ok := r.dbStats[dbName]; ok {
		p.mb.RecordPostgresqlCommitsDataPoint(now, stats.transactionCommitted)
		p.mb.RecordPostgresqlRollbacksDataPoint(now, stats.transactionRollback)
		p.mb.RecordPostgresqlDeadlocksDataPoint(now, stats.deadlocks)
		p.mb.RecordPostgresqlTempFilesDataPoint(now, stats.tempFiles)
		p.mb.RecordPostgresqlTupUpdatedDataPoint(now, stats.tupUpdated)
		p.mb.RecordPostgresqlTupReturnedDataPoint(now, stats.tupReturned)
		p.mb.RecordPostgresqlTupFetchedDataPoint(now, stats.tupFetched)
		p.mb.RecordPostgresqlTupInsertedDataPoint(now, stats.tupInserted)
		p.mb.RecordPostgresqlTupDeletedDataPoint(now, stats.tupDeleted)
		p.mb.RecordPostgresqlBlksHitDataPoint(now, stats.blksHit)
		p.mb.RecordPostgresqlBlksReadDataPoint(now, stats.blksRead)
	}
	rb := p.mb.NewResourceBuilder()
	rb.SetPostgresqlDatabaseName(db)
	p.mb.EmitForResource(metadata.WithResource(rb.Emit()))
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
		p.mb.RecordPostgresqlRowsDataPoint(now, tm.dead, metadata.AttributeStateDead)
		p.mb.RecordPostgresqlRowsDataPoint(now, tm.live, metadata.AttributeStateLive)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.inserts, metadata.AttributeOperationIns)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.del, metadata.AttributeOperationDel)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.upd, metadata.AttributeOperationUpd)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.hotUpd, metadata.AttributeOperationHotUpd)
		p.mb.RecordPostgresqlTableSizeDataPoint(now, tm.size)
		p.mb.RecordPostgresqlTableVacuumCountDataPoint(now, tm.vacuumCount)
		p.mb.RecordPostgresqlSequentialScansDataPoint(now, tm.seqScans)

		br, ok := blockReads[tableKey]
		if ok {
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.heapRead, metadata.AttributeSourceHeapRead)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.heapHit, metadata.AttributeSourceHeapHit)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.idxRead, metadata.AttributeSourceIdxRead)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.idxHit, metadata.AttributeSourceIdxHit)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.toastHit, metadata.AttributeSourceToastHit)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.toastRead, metadata.AttributeSourceToastRead)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.tidxRead, metadata.AttributeSourceTidxRead)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.tidxHit, metadata.AttributeSourceTidxHit)
		}
		rb := p.mb.NewResourceBuilder()
		rb.SetPostgresqlDatabaseName(db)
		if p.separateSchemaAttr {
			rb.SetPostgresqlSchemaName(tm.schema)
			rb.SetPostgresqlTableName(tm.table)
		} else {
			rb.SetPostgresqlTableName(fmt.Sprintf("%s.%s", tm.schema, tm.table))
		}
		p.mb.EmitForResource(metadata.WithResource(rb.Emit()))
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
		p.mb.RecordPostgresqlIndexScansDataPoint(now, stat.scans)
		p.mb.RecordPostgresqlIndexSizeDataPoint(now, stat.size)
		rb := p.mb.NewResourceBuilder()
		rb.SetPostgresqlDatabaseName(database)
		if p.separateSchemaAttr {
			rb.SetPostgresqlSchemaName(stat.schema)
			rb.SetPostgresqlTableName(stat.table)
		} else {
			rb.SetPostgresqlTableName(stat.table)
		}
		rb.SetPostgresqlIndexName(stat.index)
		p.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
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
		if preciseLagMetricsFg.IsEnabled() {
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

func (p *postgreSQLScraper) retrieveBackends(
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

// reference: https://pkg.go.dev/container/heap#example-package-priorityQueue

type item struct {
	row      map[string]any
	priority float64
	index    int
}

type priorityQueue []*item

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority > pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // don't stop the GC from reclaiming the item eventually
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
