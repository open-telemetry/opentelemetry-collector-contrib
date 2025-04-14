// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"database/sql"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
)

type metricsExporter struct {
	client *sql.DB

	logger       *zap.Logger
	cfg          *Config
	tablesConfig internal.MetricTablesConfigMapper
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) (*metricsExporter, error) {
	client, err := newClickhouseClient(cfg)
	if err != nil {
		return nil, err
	}

	tablesConfig := generateMetricTablesConfigMapper(cfg)

	return &metricsExporter{
		client:       client,
		logger:       logger,
		cfg:          cfg,
		tablesConfig: tablesConfig,
	}, nil
}

func (e *metricsExporter) start(ctx context.Context, _ component.Host) error {
	internal.SetLogger(e.logger)

	if !e.cfg.shouldCreateSchema() {
		return nil
	}

	if err := createDatabase(ctx, e.cfg); err != nil {
		return err
	}

	ttlExpr := generateTTLExpr(e.cfg.TTL, "toDateTime(TimeUnix)")
	return internal.NewMetricsTable(ctx, e.tablesConfig, e.cfg.clusterString(), e.cfg.tableEngineString(), ttlExpr, e.client)
}

func generateMetricTablesConfigMapper(cfg *Config) internal.MetricTablesConfigMapper {
	return internal.MetricTablesConfigMapper{
		pmetric.MetricTypeGauge:                cfg.MetricsTables.Gauge,
		pmetric.MetricTypeSum:                  cfg.MetricsTables.Sum,
		pmetric.MetricTypeSummary:              cfg.MetricsTables.Summary,
		pmetric.MetricTypeHistogram:            cfg.MetricsTables.Histogram,
		pmetric.MetricTypeExponentialHistogram: cfg.MetricsTables.ExponentialHistogram,
	}
}

// shutdown will shut down the exporter.
func (e *metricsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *metricsExporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	metricsMap := internal.NewMetricsModel(e.tablesConfig)
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		metrics := md.ResourceMetrics().At(i)
		resAttr := metrics.Resource().Attributes()
		for j := 0; j < metrics.ScopeMetrics().Len(); j++ {
			rs := metrics.ScopeMetrics().At(j).Metrics()
			scopeInstr := metrics.ScopeMetrics().At(j).Scope()
			scopeURL := metrics.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				var errs error
				//exhaustive:enforce
				switch r.Type() {
				case pmetric.MetricTypeGauge:
					errs = errors.Join(errs, metricsMap[pmetric.MetricTypeGauge].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.Gauge(), r.Name(), r.Description(), r.Unit()))
				case pmetric.MetricTypeSum:
					errs = errors.Join(errs, metricsMap[pmetric.MetricTypeSum].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.Sum(), r.Name(), r.Description(), r.Unit()))
				case pmetric.MetricTypeHistogram:
					errs = errors.Join(errs, metricsMap[pmetric.MetricTypeHistogram].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.Histogram(), r.Name(), r.Description(), r.Unit()))
				case pmetric.MetricTypeExponentialHistogram:
					errs = errors.Join(errs, metricsMap[pmetric.MetricTypeExponentialHistogram].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.ExponentialHistogram(), r.Name(), r.Description(), r.Unit()))
				case pmetric.MetricTypeSummary:
					errs = errors.Join(errs, metricsMap[pmetric.MetricTypeSummary].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.Summary(), r.Name(), r.Description(), r.Unit()))
				case pmetric.MetricTypeEmpty:
					return errors.New("metrics type is unset")
				default:
					return errors.New("unsupported metrics type")
				}
				if errs != nil {
					return errs
				}
			}
		}
	}
	// batch insert https://clickhouse.com/docs/en/about-us/performance/#performance-when-inserting-data
	return internal.InsertMetrics(ctx, e.client, metricsMap)
}
