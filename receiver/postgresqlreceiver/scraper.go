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
	"fmt"
	"strconv"
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

	var errors scrapererror.ScrapeErrors

	p.collectCommitsAndRollbacks(ctx, now, listClient, databases, errors)
	p.collectDatabaseSize(ctx, now, listClient, databases, errors)
	p.collectBackends(ctx, now, listClient, databases, errors)
	p.collectBackgroundWriterStats(ctx, now, listClient, databases, errors)
	p.collectReplicationStats(ctx, now, listClient, errors)

	for _, database := range databases {
		dbClient, err := p.clientFactory.getClient(p.config, database)
		if err != nil {
			errors.Add(err)
			p.logger.Error("Failed to initialize connection to postgres", zap.String("database", database), zap.Error(err))
			continue
		}
		defer dbClient.Close()

		p.collectBlockReads(ctx, now, dbClient, errors)
		p.collectDatabaseTableMetrics(ctx, now, dbClient, errors)
		p.collectIndexStats(ctx, now, dbClient, database, errors)
	}

	if p.config.collectQueryPerformance {
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
) {
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
		for stateKey, state := range metadata.MapAttributeState {
			value, ok := table.stats[stateKey]
			if !ok {
				// Data isn't present, error was already logged at a lower level
				continue
			}
			i, err := p.parseInt(stateKey, value)
			if err != nil {
				errors.AddPartial(0, err)
				continue
			}
			p.mb.RecordPostgresqlRowsDataPoint(now, i, state, table.database, table.table)
		}

		for opKey, op := range metadata.MapAttributeOperation {
			value, ok := table.stats[opKey]
			if !ok {
				// Data isn't present, error was already logged at a lower level
				continue
			}
			i, err := p.parseInt(opKey, value)
			if err != nil {
				errors.AddPartial(0, err)
				continue
			}
			p.mb.RecordPostgresqlOperationsDataPoint(now, i, table.database, table.table, op)
		}
	}
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
		p.mb.RecordPostgresqlDatabaseTableIndexSizeDataPoint(now, is.size)
		p.mb.RecordPostgresqlDatabaseTableIndexScansDataPoint(now, is.scans)
		p.mb.EmitForResource(
			metadata.WithPostgresqlDatabase(db),
			metadata.WithPostgresqlDatabaseTable(is.table),
			metadata.WithPostgresqlIndexName(is.index),
		)
	}
}

func (p *postgreSQLScraper) collectReplicationStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	errors scrapererror.ScrapeErrors,
) {
	replicationDelay, err := client.getReplicationDelay(ctx)
	if err != nil {
		errors.Add(err)
		return
	}
	p.mb.RecordPostgresqlReplicationDelayDataPoint(now, replicationDelay)
}

func (p *postgreSQLScraper) collectBackgroundWriterStats(
	ctx context.Context,
	now pcommon.Timestamp,
	client client,
	databases []string,
	errors scrapererror.ScrapeErrors,
) {
	bgStats, err := client.getBackgroundWriterStats(ctx)
	if err != nil {
		p.logger.Error("Errors encountered while fetching backends", zap.Error(err))
		errors.AddPartial(0, err)
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
	qs, err := client.getQueryStats(ctx)
	if err != nil {
		p.logger.Warn("unable to collect query stats, be sure to enable the \"pg_stat_statements\" extension to collect performance statistics.", zap.Error(err))
		errs.AddPartial(1, err)
		return
	}

	for _, qs := range qs {
		p.mb.RecordPostgresqlQueryDurationTotalDataPoint(now, qs.totalExecTimeMs)
		p.mb.RecordPostgresqlQueryDurationAverageDataPoint(now, qs.meanExecTimeMs)
		p.mb.RecordPostgresqlQueryCountDataPoint(now, qs.calls)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.sharedBlocksRead, metadata.AttributeQueryBlockOperationRead, metadata.AttributeQueryBlockTypeShared)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.sharedBlocksWritten, metadata.AttributeQueryBlockOperationWrite, metadata.AttributeQueryBlockTypeShared)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.sharedBlocksDirtied, metadata.AttributeQueryBlockOperationDirty, metadata.AttributeQueryBlockTypeShared)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.localBlocksRead, metadata.AttributeQueryBlockOperationRead, metadata.AttributeQueryBlockTypeLocal)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.localBlocksWritten, metadata.AttributeQueryBlockOperationWrite, metadata.AttributeQueryBlockTypeLocal)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.localBlocksDirtied, metadata.AttributeQueryBlockOperationDirty, metadata.AttributeQueryBlockTypeLocal)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.tempBlocksRead, metadata.AttributeQueryBlockOperationRead, metadata.AttributeQueryBlockTypeTemp)
		p.mb.RecordPostgresqlQueryBlockCountDataPoint(now, qs.tempBlocksRead, metadata.AttributeQueryBlockOperationWrite, metadata.AttributeQueryBlockTypeTemp)

		p.mb.EmitForResource(
			metadata.WithPostgresqlQuery(qs.query),
		)
	}
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
