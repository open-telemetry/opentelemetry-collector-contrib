// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"

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

	e.db, err = internal.NewClickhouseClientFromOptions(opt, e.cfg.shouldCreateSchema())
	if err != nil {
		return err
	}

	if e.cfg.shouldCreateSchema() {
		database := e.cfg.database()
		clusterStr := e.cfg.clusterString()
		if err := internal.CreateDatabase(ctx, e.db, database, clusterStr, e.cfg.databaseEngineString()); err != nil {
			return err
		}

		ttlExpr := internal.GenerateTTLExpr(e.cfg.TTL, "toDateTime(TimeUnix)")
		err := metrics.NewMetricsTable(ctx, e.tablesConfig, database, clusterStr, e.cfg.tableEngineString(), ttlExpr, e.db)
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
	var skippedCount int

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		metrics := md.ResourceMetrics().At(i)
		resAttr := metrics.Resource().Attributes()
		for j := 0; j < metrics.ScopeMetrics().Len(); j++ {
			rs := metrics.ScopeMetrics().At(j).Metrics()
			scopeInstr := metrics.ScopeMetrics().At(j).Scope()
			scopeURL := metrics.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				if r.Type() == pmetric.MetricTypeEmpty {
					e.logger.Debug(
						"Skipping metric with unset type",
						zap.String("metric_name", r.Name()),
						zap.String("metric_type", r.Type().String()),
					)
					skippedCount++
					continue
				}
				m, ok := metricsMap[r.Type()]
				if !ok {
					e.logger.Debug(
						"Skipping metric with unsupported type",
						zap.String("metric_name", r.Name()),
						zap.String("metric_type", r.Type().String()),
					)
					skippedCount++
					continue
				}
				m.Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	if skippedCount > 0 {
		e.logger.Debug(
			"Skipped metrics during export",
			zap.Int("skipped_count", skippedCount),
		)
	}

	return metrics.InsertMetrics(ctx, e.db, metricsMap)
}
