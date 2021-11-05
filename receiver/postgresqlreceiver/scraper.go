package postgresqlreceiver

import (
	"context"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

type postgreSQLScraper struct {
	logger *zap.Logger
	config *Config
}

func newPostgreSQLScraper(
	logger *zap.Logger,
	config *Config,
) *postgreSQLScraper {
	return &postgreSQLScraper{
		logger: logger,
		config: config,
	}
}

// start starts the scraper
func (p *postgreSQLScraper) start(_ context.Context, host component.Host) error {
	return nil
}

var initializeClient = func(p *postgreSQLScraper, database string) (client, error) {
	return newPostgreSQLClient(postgreSQLConfig{
		username:  p.config.Username,
		password:  p.config.Password,
		database:  database,
		host:      p.config.Host,
		port:      p.config.Port,
		sslConfig: p.config.SSLConfig,
	})
}

func (p *postgreSQLScraper) shutdown(context.Context) error {
	return nil
}

// initMetric initializes a metric with a metadata attribute.
func initMetric(ms pdata.MetricSlice, mi metadata.MetricIntf) pdata.Metric {
	m := ms.AppendEmpty()
	mi.Init(m)
	return m
}

// addToIntMetric adds and attributes a int sum datapoint to metricslice.
func addToIntMetric(metric pdata.NumberDataPointSlice, attributes pdata.AttributeMap, value int64, ts pdata.Timestamp) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(ts)
	dataPoint.SetIntVal(value)
	if attributes.Len() > 0 {
		attributes.CopyTo(dataPoint.Attributes())
	}
}

// scrape scrapes the metric stats, transforms them and attributes them into a metric slices.
func (p *postgreSQLScraper) scrape(context.Context) (pdata.Metrics, error) {
	// metric initialization
	md := pdata.NewMetrics()
	ilm := md.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otel/postgresql")
	now := pdata.NewTimestampFromTime(time.Now())

	blocksRead := initMetric(ilm.Metrics(), metadata.M.PostgresqlBlocksRead).Sum().DataPoints()
	commits := initMetric(ilm.Metrics(), metadata.M.PostgresqlCommits).Sum().DataPoints()
	databaseSize := initMetric(ilm.Metrics(), metadata.M.PostgresqlDbSize).Gauge().DataPoints()
	backends := initMetric(ilm.Metrics(), metadata.M.PostgresqlBackends).Gauge().DataPoints()
	databaseRows := initMetric(ilm.Metrics(), metadata.M.PostgresqlRows).Gauge().DataPoints()
	operations := initMetric(ilm.Metrics(), metadata.M.PostgresqlOperations).Sum().DataPoints()
	rollbacks := initMetric(ilm.Metrics(), metadata.M.PostgresqlRollbacks).Sum().DataPoints()

	databaseAgnosticMetricsCollected := false
	databases := p.config.Databases
	if len(databases) == 0 {
		client, err := initializeClient(p, "")
		if err != nil {
			p.logger.Error("Failed to initialize connection to postgres", zap.Error(err))
			return md, err
		}
		defer client.Close()

		dbList, err := client.listDatabases()
		if err != nil {
			p.logger.Error("Failed to request list of databases from postgres", zap.Error(err))
			return md, err
		}

		databases = dbList
		p.databaseAgnosticMetricCollection(
			now,
			client,
			databases,
			commits,
			rollbacks,
			databaseSize,
			backends,
		)
		databaseAgnosticMetricsCollected = true
	}

	for _, database := range databases {
		client, err := initializeClient(p, database)
		if err != nil {
			// TODO - do string interpolation better
			p.logger.Error("Failed to initialize connection to postgres", zap.String("database", database), zap.Error(err))
			continue
		}

		defer client.Close()

		if !databaseAgnosticMetricsCollected {
			p.databaseAgnosticMetricCollection(
				now,
				client,
				databases,
				commits,
				rollbacks,
				databaseSize,
				backends,
			)
			databaseAgnosticMetricsCollected = true
		}

		p.databaseSpecificMetricCollection(
			now,
			client,
			blocksRead,
			databaseRows,
			operations,
		)
	}

	return md, nil
}

func (p *postgreSQLScraper) databaseSpecificMetricCollection(
	now pdata.Timestamp,
	client client,
	blocksRead pdata.NumberDataPointSlice,
	databaseRows pdata.NumberDataPointSlice,
	operations pdata.NumberDataPointSlice,
) {
	// blocks read by table
	blocksReadByTableMetrics, err := client.getBlocksReadByTable()
	if err != nil {
		p.logger.Error("Failed to fetch blocks read by table", zap.Error(err))
	} else {
		for _, table := range blocksReadByTableMetrics {
			for k, v := range table.stats {
				if i, ok := p.parseInt(k, v); ok {
					attributes := pdata.NewAttributeMap()
					attributes.Insert(metadata.L.Database, pdata.NewAttributeValueString(table.database))
					attributes.Insert(metadata.L.Table, pdata.NewAttributeValueString(table.table))
					attributes.Insert(metadata.L.Source, pdata.NewAttributeValueString(k))
					addToIntMetric(blocksRead, attributes, i, now)
				}
			}
		}
	}

	// database rows & operations by table
	databaseTableMetrics, err := client.getDatabaseTableMetrics()
	if err != nil {
		p.logger.Error("Failed to fetch database table metrics", zap.Error(err))
	} else {
		for _, table := range databaseTableMetrics {
			for _, key := range []string{"live", "dead"} {
				value := table.stats[key]
				if i, ok := p.parseInt(key, value); ok {
					attributes := pdata.NewAttributeMap()
					attributes.Insert(metadata.L.Database, pdata.NewAttributeValueString(table.database))
					attributes.Insert(metadata.L.Table, pdata.NewAttributeValueString(table.table))
					attributes.Insert(metadata.L.State, pdata.NewAttributeValueString(key))
					addToIntMetric(databaseRows, attributes, i, now)
				}
			}

			for _, key := range []string{"ins", "upd", "del", "hot_upd"} {
				value := table.stats[key]
				if i, ok := p.parseInt(key, value); ok {
					attributes := pdata.NewAttributeMap()
					attributes.Insert(metadata.L.Database, pdata.NewAttributeValueString(table.database))
					attributes.Insert(metadata.L.Table, pdata.NewAttributeValueString(table.table))
					attributes.Insert(metadata.L.Operation, pdata.NewAttributeValueString(key))
					addToIntMetric(operations, attributes, i, now)
				}
			}
		}
	}
}

func (p *postgreSQLScraper) databaseAgnosticMetricCollection(
	now pdata.Timestamp,
	client client,
	databases []string,
	commits pdata.NumberDataPointSlice,
	rollbacks pdata.NumberDataPointSlice,
	databaseSize pdata.NumberDataPointSlice,
	backends pdata.NumberDataPointSlice,
) {
	// commits & rollbacks
	xactMetrics, err := client.getCommitsAndRollbacks(databases)
	if err != nil {
		p.logger.Error("Failed to fetch commits and rollbacks", zap.Error(err))
	} else {
		for _, metric := range xactMetrics {
			commitValue := metric.stats["xact_commit"]
			if i, ok := p.parseInt("xact_commit", commitValue); ok {
				attributes := pdata.NewAttributeMap()
				attributes.Insert(metadata.L.Database, pdata.NewAttributeValueString(metric.database))
				addToIntMetric(commits, attributes, i, now)
			}

			rollbackValue := metric.stats["xact_rollback"]
			if i, ok := p.parseInt("xact_rollback", rollbackValue); ok {
				attributes := pdata.NewAttributeMap()
				attributes.Insert(metadata.L.Database, pdata.NewAttributeValueString(metric.database))
				addToIntMetric(rollbacks, attributes, i, now)
			}
		}
	}

	// database size
	databaseSizeMetric, err := client.getDatabaseSize(databases)
	if err != nil {
		p.logger.Error("Failed to fetch database size", zap.Error(err))
	} else {
		for _, metric := range databaseSizeMetric {
			for k, v := range metric.stats {
				if f, ok := p.parseInt(k, v); ok {
					attributes := pdata.NewAttributeMap()
					attributes.Insert(metadata.L.Database, pdata.NewAttributeValueString(metric.database))
					addToIntMetric(databaseSize, attributes, f, now)
				}
			}
		}
	}

	// backends
	backendsMetric, err := client.getBackends(databases)
	if err != nil {
		p.logger.Error("Failed to fetch backends", zap.Error(err))
	} else {
		for _, metric := range backendsMetric {
			for k, v := range metric.stats {
				if f, ok := p.parseInt(k, v); ok {
					attributes := pdata.NewAttributeMap()
					attributes.Insert(metadata.L.Database, pdata.NewAttributeValueString(metric.database))
					addToIntMetric(backends, attributes, f, now)
				}
			}
		}
	}
}

// parseInt converts string to int64.
func (p *postgreSQLScraper) parseInt(key, value string) (int64, bool) {
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		p.logger.Info(
			"invalid value",
			zap.String("expectedType", "int"),
			zap.String("key", key),
			zap.String("value", value),
		)
		return 0, false
	}
	return i, true
}
