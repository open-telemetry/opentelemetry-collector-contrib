// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"net/url"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/metrics"
)

type metricsExporter struct {
	db driver.Conn

	logger       *zap.Logger
	cfg          *Config
	tablesConfig metrics.MetricTablesConfigMapper
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) *metricsExporter {
	tablesConfig := generateMetricTablesConfigMapper(cfg)

	return &metricsExporter{
		logger:       logger,
		cfg:          cfg,
		tablesConfig: tablesConfig,
	}
}

func (e *metricsExporter) start(ctx context.Context, _ component.Host) error {
	metrics.SetLogger(e.logger)

	opt, err := e.cfg.buildClickHouseOptions()
	if err != nil {
		return err
	}

	// InsertMetrics inserts each metric type concurrently, so scale the pool sized from the queue's
	// number of consumers. Values set in the endpoint or connection params are left unchanged.
	dsn, err := e.cfg.buildDSN()
	if err != nil {
		return err
	}
	dsnURL, err := url.Parse(dsn)
	if err != nil {
		return err
	}
	dsnParams := dsnURL.Query()
	if !dsnParams.Has("max_open_conns") {
		opt.MaxOpenConns *= metrics.MaxConcurrentInserts()
	}
	if !dsnParams.Has("max_idle_conns") {
		opt.MaxIdleConns = min(opt.MaxIdleConns*metrics.MaxConcurrentInserts(), opt.MaxOpenConns)
	}

	e.db, err = internal.NewClickhouseClientFromOptions(opt, e.cfg.shouldCreateSchema())
	if err != nil {
		return err
	}

	if e.cfg.shouldCreateSchema() {
		database := e.cfg.database()
		if err := internal.CreateDatabase(ctx, e.db, database, e.cfg.clusterString(), e.cfg.databaseEngineString()); err != nil {
			return err
		}

		ttlExpr := internal.GenerateTTLExpr(e.cfg.TTL, "toDateTime(TimeUnix)")
		err := metrics.NewMetricsTable(ctx, e.tablesConfig, database, e.cfg.tableClusterString(), e.cfg.tableEngineString(), ttlExpr, e.db)
		if err != nil {
			return err
		}
	}

	return nil
}

func generateMetricTablesConfigMapper(cfg *Config) metrics.MetricTablesConfigMapper {
	return metrics.MetricTablesConfigMapper{
		pmetric.MetricTypeGauge:                cfg.MetricsTables.Gauge,
		pmetric.MetricTypeSum:                  cfg.MetricsTables.Sum,
		pmetric.MetricTypeSummary:              cfg.MetricsTables.Summary,
		pmetric.MetricTypeHistogram:            cfg.MetricsTables.Histogram,
		pmetric.MetricTypeExponentialHistogram: cfg.MetricsTables.ExponentialHistogram,
	}
}

// shutdown will shut down the exporter.
func (e *metricsExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		return e.db.Close()
	}

	return nil
}

func (e *metricsExporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	metricsMap := metrics.NewMetricsModel(e.tablesConfig, e.cfg.database())

	// Metrics that cannot be represented (no data type set, or a type without a table) are skipped
	// rather than failing the request. Returning an error here would drop every valid metric in the
	// batch, and since the data never changes, retrying could not succeed.
	var skipped skippedMetrics
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resAttr := rm.Resource().Attributes()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			ms := sm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				model, ok := metricsMap[m.Type()]
				if !ok {
					skipped.add(m)
					continue
				}
				model.Add(resAttr, rm.SchemaUrl(), sm.Scope(), sm.SchemaUrl(), m)
			}
		}
	}
	skipped.log(e.logger)

	return metrics.InsertMetrics(ctx, e.db, metricsMap)
}

// maxSkippedMetricNames bounds how many skipped metric names are included in the per-batch warning.
const maxSkippedMetricNames = 5

// skippedMetrics tracks the metrics a batch could not export, so they can be reported once per batch.
type skippedMetrics struct {
	count int
	types map[string]int
	names []string
}

func (s *skippedMetrics) add(m pmetric.Metric) {
	if s.types == nil {
		s.types = make(map[string]int)
	}
	s.count++
	s.types[m.Type().String()]++
	if len(s.names) < maxSkippedMetricNames {
		s.names = append(s.names, m.Name())
	}
}

func (s *skippedMetrics) log(logger *zap.Logger) {
	if s.count == 0 {
		return
	}
	logger.Warn("Skipped metrics with an unset or unsupported type",
		zap.Int("skipped_metrics", s.count),
		zap.Any("skipped_by_type", s.types),
		zap.Strings("example_metric_names", s.names),
	)
}
