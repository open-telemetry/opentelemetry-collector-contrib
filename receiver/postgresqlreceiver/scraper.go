// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

type postgreSQLScraper struct {
	logger        *zap.Logger
	config        *Config
	clientFactory postgreSQLClientFactory
	mb            *metadata.MetricsBuilder
}

type postgreSQLClientFactory interface {
	getClient(c *Config, database string) (client, error)
}

type defaultClientFactory struct{}

func (d *defaultClientFactory) getClient(c *Config, database string) (client, error) {
	return newPostgreSQLClient(postgreSQLConfig{
		username: c.Username,
		password: c.Password,
		database: database,
		tls:      c.TLSClientSetting,
		address:  c.NetAddr,
	})
}

func newPostgreSQLScraper(
	settings component.ReceiverCreateSettings,
	config *Config,
	clientFactory postgreSQLClientFactory,
) *postgreSQLScraper {
	return &postgreSQLScraper{
		logger:        settings.Logger,
		config:        config,
		clientFactory: clientFactory,
		mb:            metadata.NewMetricsBuilder(config.Metrics, settings.BuildInfo),
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
	listClient, err := p.clientFactory.getClient(p.config, "")
	if err != nil {
		p.logger.Error("Failed to initialize connection to postgres", zap.Error(err))
		return pmetric.NewMetrics(), err
	}
	defer listClient.Close()

	if len(databases) == 0 {
		dbList, err := listClient.listDatabases(ctx)
		if err != nil {
			p.logger.Error("Failed to request list of databases from postgres", zap.Error(err))
			return pmetric.NewMetrics(), err
		}
		databases = dbList
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	var errs scrapererror.ScrapeErrors
	r := &dbRetrieval{
		activityMap: make(map[databaseName]int64),
		dbSizeMap:   make(map[databaseName]int64),
		dbStats:     make(map[databaseName]databaseStats),
	}
	p.retrieveDBMetrics(ctx, listClient, databases, r, &errs)

	for _, database := range databases {
		dbClient, err := p.clientFactory.getClient(p.config, database)
		if err != nil {
			errs.Add(err)
			p.logger.Error("Failed to initialize connection to postgres", zap.String("database", database), zap.Error(err))
			continue
		}
		defer dbClient.Close()
		p.recordDatabase(now, database, r)
		p.collectTables(ctx, now, dbClient, database, &errs)
	}

	return p.mb.Emit(), errs.Combine()
}

func (p *postgreSQLScraper) retrieveDBMetrics(
	ctx context.Context,
	listClient client,
	databases []string,
	r *dbRetrieval,
	errs *scrapererror.ScrapeErrors,
) {
	wg := &sync.WaitGroup{}

	wg.Add(3)
	go p.retrieveBackends(ctx, wg, listClient, databases, r, errs)
	go p.retrieveDatabaseSize(ctx, wg, listClient, databases, r, errs)
	go p.retrieveDatabaseStats(ctx, wg, listClient, databases, r, errs)

	wg.Wait()
}

func (p *postgreSQLScraper) recordDatabase(now pcommon.Timestamp, db string, r *dbRetrieval) {
	if activeConnections, ok := r.activityMap[databaseName(db)]; ok {
		p.mb.RecordPostgresqlBackendsDataPoint(now, activeConnections, db)
	}
	if size, ok := r.dbSizeMap[databaseName(db)]; ok {
		p.mb.RecordPostgresqlDbSizeDataPoint(now, size, db)
	}
	if stats, ok := r.dbStats[databaseName(db)]; ok {
		p.mb.RecordPostgresqlCommitsDataPoint(now, stats.transactionCommitted, db)
		p.mb.RecordPostgresqlRollbacksDataPoint(now, stats.transactionRollback, db)
	}
}

func (p *postgreSQLScraper) collectTables(ctx context.Context, now pcommon.Timestamp, dbClient client, db string, errs *scrapererror.ScrapeErrors) {
	blockReads, err := dbClient.getBlocksReadByTable(ctx, db)
	if err != nil {
		errs.AddPartial(1, err)
	}

	tableMetrics, err := dbClient.getDatabaseTableMetrics(ctx, db)
	if err != nil {
		errs.AddPartial(1, err)
	}

	for tableKey, tm := range tableMetrics {
		p.mb.RecordPostgresqlRowsDataPoint(now, tm.dead, db, tm.table, metadata.AttributeStateDead)
		p.mb.RecordPostgresqlRowsDataPoint(now, tm.live, db, tm.table, metadata.AttributeStateLive)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.inserts, db, tm.table, metadata.AttributeOperationIns)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.del, db, tm.table, metadata.AttributeOperationDel)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.upd, db, tm.table, metadata.AttributeOperationUpd)
		p.mb.RecordPostgresqlOperationsDataPoint(now, tm.hotUpd, db, tm.table, metadata.AttributeOperationHotUpd)
		br, ok := blockReads[tableKey]
		if ok {
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.heapRead, db, br.table, metadata.AttributeSourceHeapRead)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.heapHit, db, br.table, metadata.AttributeSourceHeapHit)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.idxRead, db, br.table, metadata.AttributeSourceIdxRead)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.idxHit, db, br.table, metadata.AttributeSourceIdxHit)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.toastHit, db, br.table, metadata.AttributeSourceToastHit)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.toastRead, db, br.table, metadata.AttributeSourceToastRead)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.tidxRead, db, br.table, metadata.AttributeSourceTidxRead)
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, br.tidxHit, db, br.table, metadata.AttributeSourceTidxHit)
		}
	}
}

func (p *postgreSQLScraper) retrieveDatabaseStats(
	ctx context.Context,
	wg *sync.WaitGroup,
	client client,
	databases []string,
	r *dbRetrieval,
	errors *scrapererror.ScrapeErrors,
) {
	defer wg.Done()
	dbStats, err := client.getDatabaseStats(ctx, databases)
	if err != nil {
		p.logger.Error("Errors encountered while fetching commits and rollbacks", zap.Error(err))
		errors.AddPartial(1, err)
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
	errors *scrapererror.ScrapeErrors,
) {
	defer wg.Done()
	databaseSizeMetrics, err := client.getDatabaseSize(ctx, databases)
	if err != nil {
		p.logger.Error("Errors encountered while fetching database size", zap.Error(err))
		errors.AddPartial(1, err)
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
	errors *scrapererror.ScrapeErrors,
) {
	defer wg.Done()
	activityByDB, err := client.getBackends(ctx, databases)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	r.Lock()
	r.activityMap = activityByDB
	r.Unlock()
}
