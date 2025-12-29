// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starrocksexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter"

import (
	"context"
	"database/sql"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/metrics"
)

type metricsExporter struct {
	db *sql.DB

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

	db, err := e.cfg.buildStarRocksDB()
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.shouldCreateSchema() {
		database := e.cfg.database()
		if err := internal.CreateDatabase(ctx, e.db, database); err != nil {
			return err
		}

		err := metrics.NewMetricsTable(ctx, e.tablesConfig, database, e.db)
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
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics().At(i)
		resAttr := resourceMetrics.Resource().Attributes()
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			rs := resourceMetrics.ScopeMetrics().At(j).Metrics()
			scopeInstr := resourceMetrics.ScopeMetrics().At(j).Scope()
			scopeURL := resourceMetrics.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				if r.Type() == pmetric.MetricTypeEmpty {
					return errors.New("metrics type is unset")
				}
				m, ok := metricsMap[r.Type()]
				if !ok {
					return errors.New("unsupported metrics type")
				}
				m.Add(resAttr, resourceMetrics.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	return metrics.InsertMetrics(ctx, e.db, metricsMap)
}



