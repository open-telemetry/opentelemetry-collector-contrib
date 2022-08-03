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
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/go-version"
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
	pgVersion     *version.Version
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

	if p.pgVersion == nil {
		v, err := listClient.getVersion(ctx)
		if err != nil {
			p.logger.Error("Unable to get a version specification from the postgres instance. Unable to determine collection strategy.")
			return pmetric.NewMetrics(), err
		}
		p.pgVersion = v
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	var errors scrapererror.ScrapeErrors

	// instance level metrics
	p.collectCommitsAndRollbacks(ctx, now, listClient, databases, errors)
	p.collectDatabaseSize(ctx, now, listClient, databases, errors)
	p.collectBackends(ctx, now, listClient, databases, errors)
	p.collectMaxConnections(ctx, now, listClient, errors)
	p.collectBackgroundWriterStats(ctx, now, listClient, errors)
	p.collectReplicationDelay(ctx, now, listClient, errors)
	p.collectReplicationStats(ctx, now, listClient, errors)
	p.collectWalStats(ctx, now, listClient, errors)

	p.mb.RecordPostgresqlDatabaseCountDataPoint(now, int64(len(databases)))

	for _, database := range databases {
		dbClient, err := p.clientFactory.getClient(p.config, database)
		if err != nil {
			errors.Add(err)
			p.logger.Error("Failed to initialize connection to postgres", zap.String("database", database), zap.Error(err))
			continue
		}
		defer dbClient.Close()

		// database table metrics
		numTables := p.collectDatabaseTableMetrics(ctx, now, dbClient, errors)

		// database metrics
		p.mb.RecordPostgresqlTableCountDataPoint(now, numTables, database)
		p.collectBlockReads(ctx, now, dbClient, errors)

		// index metrics
		p.collectIndexStats(ctx, now, dbClient, database, errors)
	}

	// query metrics
	if p.config.CollectQueryPerformance {
		p.collectQueries(ctx, now, listClient, errors)
	}

	return p.mb.Emit(), errors.Combine()
}

func (p *postgreSQLScraper) collectBlockReads(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errors scrapererror.ScrapeErrors,
) {
	blocksReadByTableMetrics, err := client.getBlocksReadByTable(ctx)
	if err != nil {
		p.logger.Error("Errors encountered while fetching blocks read by table", zap.Error(err))
		errors.AddPartial(0, err)
	}

	// Metrics can be partially collected (non-nil) even if there were partial errors reported
	if blocksReadByTableMetrics == nil {
		return
	}
	for _, table := range blocksReadByTableMetrics {
		for sourceKey, source := range metadata.MapAttributeSource {
			value, ok := table.stats[sourceKey]
			if !ok {
				// Data isn't present, error was already logged at a lower level
				continue
			}
			i, err := p.parseInt(sourceKey, value)
			if err != nil {
				errors.AddPartial(0, err)
				continue
			}
			p.mb.RecordPostgresqlBlocksReadDataPoint(now, i, table.database, table.table, source)
		}
	}
}

func (p *postgreSQLScraper) collectDatabaseTableMetrics(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errors scrapererror.ScrapeErrors,
) (numTables int64) {
	databaseTableMetrics, err := client.getDatabaseTableMetrics(ctx)
	if err != nil {
		p.logger.Error("Errors encountered while fetching database table metrics", zap.Error(err))
		errors.AddPartial(0, err)
	}

	// Metrics can be partially collected (non-nil) even if there were partial errors reported
	if databaseTableMetrics == nil {
		return
	}
	for _, table := range databaseTableMetrics {
		p.mb.RecordPostgresqlRowsDataPoint(now, table.dead, metadata.AttributeStateDead, table.database, table.table)
		p.mb.RecordPostgresqlRowsDataPoint(now, table.live, metadata.AttributeStateLive, table.database, table.table)
		p.mb.RecordPostgresqlOperationsDataPoint(now, table.del, table.database, table.table, metadata.AttributeOperationDel)
		p.mb.RecordPostgresqlOperationsDataPoint(now, table.inserts, table.database, table.table, metadata.AttributeOperationIns)
		p.mb.RecordPostgresqlOperationsDataPoint(now, table.upd, table.database, table.table, metadata.AttributeOperationUpd)
		p.mb.RecordPostgresqlOperationsDataPoint(now, table.hotUpd, table.database, table.table, metadata.AttributeOperationHotUpd)
		p.mb.RecordPostgresqlTableVacuumCountDataPoint(now, table.vacuumCount, table.database, table.table)
		p.mb.RecordPostgresqlTableSizeDataPoint(now, table.size, table.database, table.table)
	}
	return int64(len(databaseTableMetrics))
}

func (p *postgreSQLScraper) collectCommitsAndRollbacks(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	databases []string,
	errors scrapererror.ScrapeErrors,
) {
	xactMetrics, err := client.getCommitsAndRollbacks(ctx, databases)
	if err != nil {
		p.logger.Error("Errors encountered while fetching commits and rollbacks", zap.Error(err))
		errors.AddPartial(0, err)
	}

	// Metrics can be partially collected (non-nil) even if there were partial errors reported
	if xactMetrics == nil {
		return
	}
	for _, metric := range xactMetrics {
		commitValue := metric.stats["xact_commit"]
		if i, err := p.parseInt("xact_commit", commitValue); err != nil {
			errors.AddPartial(0, err)
			continue
		} else {
			p.mb.RecordPostgresqlCommitsDataPoint(now, i, metric.database)
		}

		rollbackValue := metric.stats["xact_rollback"]
		if i, err := p.parseInt("xact_rollback", rollbackValue); err != nil {
			errors.AddPartial(0, err)
			continue
		} else {
			p.mb.RecordPostgresqlRollbacksDataPoint(now, i, metric.database)
		}
	}
}

func (p *postgreSQLScraper) collectMaxConnections(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errors scrapererror.ScrapeErrors,
) {
	mc, err := client.getMaxConnections(ctx)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	p.mb.RecordPostgresqlConnectionsMaxDataPoint(now, mc)
}

func (p *postgreSQLScraper) collectDatabaseSize(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	databases []string,
	errors scrapererror.ScrapeErrors,
) {
	databaseSizeMetric, err := client.getDatabaseSize(ctx, databases)
	if err != nil {
		p.logger.Error("Errors encountered while fetching database size", zap.Error(err))
		errors.AddPartial(0, err)
	}

	// Metrics can be partially collected (non-nil) even if there were partial errors reported
	if databaseSizeMetric == nil {
		return
	}
	for _, metric := range databaseSizeMetric {
		for k, v := range metric.stats {
			i, err := p.parseInt(k, v)
			if err != nil {
				errors.AddPartial(0, err)
				continue
			}
			p.mb.RecordPostgresqlDbSizeDataPoint(now, i, metric.database)
		}
	}
}

func (p *postgreSQLScraper) collectBackends(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	databases []string,
	errors scrapererror.ScrapeErrors,
) {
	backendsMetric, err := client.getBackends(ctx, databases)
	if err != nil {
		p.logger.Error("Errors encountered while fetching backends", zap.Error(err))
		errors.AddPartial(0, err)
	}

	// Metrics can be partially collected (non-nil) even if there were partial errors reported
	if backendsMetric == nil {
		return
	}
	for _, metric := range backendsMetric {
		for k, v := range metric.stats {
			i, err := p.parseInt(k, v)
			if err != nil {
				errors.AddPartial(0, err)
				continue
			}
			p.mb.RecordPostgresqlBackendsDataPoint(now, i, metric.database)
		}
	}
}

func (p *postgreSQLScraper) collectIndexStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	db string,
	errors scrapererror.ScrapeErrors,
) {
	indexStat, err := client.getIndexStats(ctx, db)
	if err != nil {
		p.logger.Error(fmt.Sprintf("Errors encountered while fetching index statistics for database %s", db), zap.Error(err))
		errors.AddPartial(1, err)
		return
	}

	for _, is := range indexStat.indexStats {
		p.mb.RecordPostgresqlIndexSizeDataPoint(now, is.size, is.index, is.table)
		p.mb.RecordPostgresqlIndexScansDataPoint(now, is.scans, is.index, is.table)
		// TODO: use resoruce attributes rather than metric ones.
		// p.mb.EmitForResource(
		// 	metadata.WithPostgresqlDatabase(db),
		// 	metadata.WithPostgresqlDatabaseTable(is.table),
		// 	metadata.WithPostgresqlIndexName(is.index),
		// )
	}
}

func (p *postgreSQLScraper) collectReplicationDelay(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errors scrapererror.ScrapeErrors,
) {
	replicationDelay, err := client.getReplicationDelay(ctx)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	p.mb.RecordPostgresqlReplicationDelayDataPoint(now, replicationDelay)
}

func (p *postgreSQLScraper) collectReplicationStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errors scrapererror.ScrapeErrors,
) {
	replication, err := client.getReplicationStats(ctx)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	for _, rs := range replication {
		p.mb.RecordPostgresqlReplicationDataDelayDataPoint(now, rs.pendingBytes, rs.client)
		p.mb.RecordPostgresqlWalLagDataPoint(now, rs.replayLag, metadata.AttributeWalOperationLagReplay)
		p.mb.RecordPostgresqlWalLagDataPoint(now, rs.flushLag, metadata.AttributeWalOperationLagFlush)
		p.mb.RecordPostgresqlWalLagDataPoint(now, rs.writeLag, metadata.AttributeWalOperationLagWrite)
	}
}

func (p *postgreSQLScraper) collectBackgroundWriterStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errors scrapererror.ScrapeErrors,
) {
	bgStats, err := client.getBackgroundWriterStats(ctx)
	if err != nil {
		p.logger.Error("Errors encountered while fetching background writers", zap.Error(err))
		errors.AddPartial(1, err)
		return
	}

	// Metrics can be partially collected (non-nil) even if there were partial errors reported
	if bgStats == nil {
		return
	}

	for _, metric := range bgStats {
		for k, v := range metric.stats {
			i, err := p.parseInt(k, v)
			if err != nil {
				errors.AddPartial(1, err)
				continue
			}

			switch k {
			case "buffers_allocated":
				p.mb.RecordPostgresqlBgwriterBuffersAllocatedDataPoint(now, i)
			case "checkpoint_req":
				p.mb.RecordPostgresqlBgwriterCheckpointCountDataPoint(now, i, metadata.AttributeBgCheckpointTypeRequested)
			case "checkpoint_scheduled":
				p.mb.RecordPostgresqlBgwriterCheckpointCountDataPoint(now, i, metadata.AttributeBgCheckpointTypeScheduled)
			case "checkpoint_duration_write":
				p.mb.RecordPostgresqlBgwriterDurationDataPoint(now, i, metadata.AttributeBgDurationTypeWrite)
			case "checkpoint_duration_sync":
				p.mb.RecordPostgresqlBgwriterDurationDataPoint(now, i, metadata.AttributeBgDurationTypeSync)
			case "bg_writes":
				p.mb.RecordPostgresqlBgwriterBuffersWritesDataPoint(now, i, metadata.AttributeBgBufferSourceBgwriter)
			case "backend_writes":
				p.mb.RecordPostgresqlBgwriterBuffersWritesDataPoint(now, i, metadata.AttributeBgBufferSourceBackend)
			case "buffers_written_fsync":
				p.mb.RecordPostgresqlBgwriterBuffersWritesDataPoint(now, i, metadata.AttributeBgBufferSourceBackendFsync)
			case "buffers_checkpoints":
				p.mb.RecordPostgresqlBgwriterBuffersWritesDataPoint(now, i, metadata.AttributeBgBufferSourceCheckpoints)
			case "maxwritten_count":
				p.mb.RecordPostgresqlBgwriterMaxwrittenCountDataPoint(now, i)
			}
		}
	}
}

func (p *postgreSQLScraper) collectQueries(ctx context.Context, now pcommon.Timestamp, client client, errs scrapererror.ScrapeErrors) {
	// unable to contextualize the query to run so don't collect
	if p.pgVersion == nil {
		return
	}
	queryStats, err := client.getQueryStats(ctx, *p.pgVersion)
	if err != nil {
		p.logger.Warn("unable to collect query stats, be sure to enable the \"pg_stat_statements\" extension to collect performance statistics.", zap.Error(err))
		errs.AddPartial(1, err)
		return
	}

	for _, qs := range queryStats {
		p.mb.RecordPostgresqlQueryDurationTotalDataPoint(now, qs.totalExecTimeMs, qs.query)
		p.mb.RecordPostgresqlQueryDurationAverageDataPoint(now, qs.meanExecTimeMs, qs.query)
		p.mb.RecordPostgresqlQueryCountDataPoint(now, qs.calls, qs.query)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.sharedBlocksRead, metadata.AttributeQueryBlockOperationRead, metadata.AttributeQueryBlockTypeShared, qs.query)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.sharedBlocksWritten, metadata.AttributeQueryBlockOperationWrite, metadata.AttributeQueryBlockTypeShared, qs.query)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.sharedBlocksDirtied, metadata.AttributeQueryBlockOperationDirty, metadata.AttributeQueryBlockTypeShared, qs.query)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.localBlocksRead, metadata.AttributeQueryBlockOperationRead, metadata.AttributeQueryBlockTypeLocal, qs.query)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.localBlocksWritten, metadata.AttributeQueryBlockOperationWrite, metadata.AttributeQueryBlockTypeLocal, qs.query)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.localBlocksDirtied, metadata.AttributeQueryBlockOperationDirty, metadata.AttributeQueryBlockTypeLocal, qs.query)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.tempBlocksRead, metadata.AttributeQueryBlockOperationRead, metadata.AttributeQueryBlockTypeTemp, qs.query)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.tempBlocksWritten, metadata.AttributeQueryBlockOperationWrite, metadata.AttributeQueryBlockTypeTemp, qs.query)

		// TODO: emit query resource
		// p.mb.EmitForResource(
		// 	metadata.WithPostgresqlQuery(qs.query),
		// )
	}
}

func (p *postgreSQLScraper) collectWalStats(ctx context.Context, now pcommon.Timestamp, client client, errs scrapererror.ScrapeErrors) {
	ws, err := client.getWALStats(ctx)
	if err != nil {
		// return no metric because there was no difference in
		// the current instance time and the last archived time.
		// This is not indicative of a scraping error so no point in recording it
		if !errors.Is(err, errNoLastArchive) {
			errs.AddPartial(1, err)
		}
		return
	}
	p.mb.RecordPostgresqlWalAgeDataPoint(now, ws.age)
}

// parseInt converts string to int64.
func (p *postgreSQLScraper) parseInt(key, value string) (int64, error) {
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		p.logger.Info(
			"invalid value",
			zap.String("expectedType", "int"),
			zap.String("key", key),
			zap.String("value", value),
		)
		return 0, err
	}
	return i, nil
}

func parseInt(v string) (int64, error) {
	return strconv.ParseInt(v, 10, 64)
}
