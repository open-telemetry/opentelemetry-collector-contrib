// Copyright The OpenTelemetry Authors
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

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
)

type metricsExporter struct {
	client *sql.DB

	logger *zap.Logger
	cfg    *Config
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) (*metricsExporter, error) {
	client, err := newClickhouseClient(cfg)
	if err != nil {
		return nil, err
	}

	return &metricsExporter{
		client: client,
		logger: logger,
		cfg:    cfg,
	}, nil
}

func (e *metricsExporter) start(ctx context.Context, _ component.Host) error {
	if err := createDatabase(ctx, e.cfg); err != nil {
		return err
	}

	internal.SetLogger(e.logger)
	if err := internal.NewMetricsTable(ctx, e.cfg.MetricsTableName, e.cfg.TTLDays, e.client); err != nil {
		return err
	}
	return nil
}

// shutdown will shut down the exporter.
func (e *metricsExporter) shutdown(ctx context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *metricsExporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	metricsMap := internal.NewMetricsModel(e.cfg.MetricsTableName)
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		metrics := md.ResourceMetrics().At(i)
		resAttr := attributesToMap(metrics.Resource().Attributes())
		for j := 0; j < metrics.ScopeMetrics().Len(); j++ {
			rs := metrics.ScopeMetrics().At(j).Metrics()
			scopeInstr := metrics.ScopeMetrics().At(j).Scope()
			scopeURL := metrics.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				var errs error
				switch r.Type() {
				case pmetric.MetricTypeGauge:
					errs = multierr.Append(errs, metricsMap[pmetric.MetricTypeGauge].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.Gauge(), r.Name(), r.Description(), r.Unit()))
				case pmetric.MetricTypeSum:
					errs = multierr.Append(errs, metricsMap[pmetric.MetricTypeSum].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.Sum(), r.Name(), r.Description(), r.Unit()))
				case pmetric.MetricTypeHistogram:
					errs = multierr.Append(errs, metricsMap[pmetric.MetricTypeHistogram].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.Histogram(), r.Name(), r.Description(), r.Unit()))
				case pmetric.MetricTypeExponentialHistogram:
					errs = multierr.Append(errs, metricsMap[pmetric.MetricTypeExponentialHistogram].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.ExponentialHistogram(), r.Name(), r.Description(), r.Unit()))
				case pmetric.MetricTypeSummary:
					errs = multierr.Append(errs, metricsMap[pmetric.MetricTypeSummary].Add(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, r.Summary(), r.Name(), r.Description(), r.Unit()))
				default:
					return fmt.Errorf("unsupported metrics type")
				}
				if errs != nil {
					return errs
				}
			}
		}
	}
	// batch insert https://clickhouse.com/docs/en/about-us/performance/#performance-when-inserting-data
	if err := internal.InsertMetrics(ctx, e.client, metricsMap); err != nil {
		return err
	}
	return nil
}
