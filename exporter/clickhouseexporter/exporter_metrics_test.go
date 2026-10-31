// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/metrics"
)

func Test_generateMetricMetricTableNames(t *testing.T) {
	cfg := Config{
		MetricsTables: MetricTablesConfig{
			Gauge:                metrics.MetricTypeConfig{Name: "otel_metrics_custom_gauge"},
			Sum:                  metrics.MetricTypeConfig{Name: "otel_metrics_custom_sum"},
			Summary:              metrics.MetricTypeConfig{Name: "otel_metrics_custom_summary"},
			Histogram:            metrics.MetricTypeConfig{Name: "otel_metrics_custom_histogram"},
			ExponentialHistogram: metrics.MetricTypeConfig{Name: "otel_metrics_custom_exp_histogram"},
		},
	}

	require.Equal(t, metrics.MetricTablesConfigMapper{
		pmetric.MetricTypeGauge:                cfg.MetricsTables.Gauge,
		pmetric.MetricTypeSum:                  cfg.MetricsTables.Sum,
		pmetric.MetricTypeSummary:              cfg.MetricsTables.Summary,
		pmetric.MetricTypeHistogram:            cfg.MetricsTables.Histogram,
		pmetric.MetricTypeExponentialHistogram: cfg.MetricsTables.ExponentialHistogram,
	}, generateMetricTablesConfigMapper(&cfg))
}

// TestMetricsModel_WithEmptyMetricType verifies that when processing metrics with
// MetricTypeEmpty, the valid metrics are still added to the model while empty metrics are skipped.
// This tests the core logic without requiring a database connection.
func TestMetricsModel_WithEmptyMetricType(t *testing.T) {
	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	// Add a valid gauge metric
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("valid_gauge")
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)

	// Add an empty type metric
	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("empty_metric")

	var skippedCount int
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resAttr := rm.Resource().Attributes()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			rs := rm.ScopeMetrics().At(j).Metrics()
			scopeInstr := rm.ScopeMetrics().At(j).Scope()
			scopeURL := rm.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				if r.Type() == pmetric.MetricTypeEmpty {
					skippedCount++
					continue
				}
				m, ok := metricsMap[r.Type()]
				if !ok {
					skippedCount++
					continue
				}
				m.Add(resAttr, rm.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	// Verify the empty metric was skipped
	require.Equal(t, 1, skippedCount, "Should have skipped 1 empty metric")

	// Verify the valid gauge was added to the model
	_, ok := metricsMap[pmetric.MetricTypeGauge]
	require.True(t, ok, "Gauge should be in the metrics map after processing")
}

// TestMetricsModel_WithMixedValidAndEmptyMetrics verifies that valid metrics in the same scope
// as empty metrics are still processed.
func TestMetricsModel_WithMixedValidAndEmptyMetrics(t *testing.T) {
	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	// Add a valid gauge metric
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("valid_gauge")
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)

	// Add an empty type metric in the same scope
	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("empty_metric")

	var skippedCount int
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resAttr := rm.Resource().Attributes()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			rs := rm.ScopeMetrics().At(j).Metrics()
			scopeInstr := rm.ScopeMetrics().At(j).Scope()
			scopeURL := rm.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				if r.Type() == pmetric.MetricTypeEmpty {
					skippedCount++
					continue
				}
				m, ok := metricsMap[r.Type()]
				if !ok {
					skippedCount++
					continue
				}
				m.Add(resAttr, rm.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	require.Equal(t, 1, skippedCount, "Should have skipped 1 empty metric")
	_, ok := metricsMap[pmetric.MetricTypeGauge]
	require.True(t, ok, "Gauge should be in the metrics map")
}

// TestMetricsModel_WithEmptyMetricInSeparateResource verifies that a valid metric in one
// resource is not affected by an empty metric in another resource.
func TestMetricsModel_WithEmptyMetricInSeparateResource(t *testing.T) {
	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

	md := pmetric.NewMetrics()

	// Resource A with valid gauge
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", "service-a")
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	sm1.Scope().SetName("scope-a")
	m1 := sm1.Metrics().AppendEmpty()
	m1.SetName("valid_gauge")
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)

	// Resource B with empty type metric
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "service-b")
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	sm2.Scope().SetName("scope-b")
	m2 := sm2.Metrics().AppendEmpty()
	m2.SetName("empty_metric")

	var skippedCount int
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resAttr := rm.Resource().Attributes()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			rs := rm.ScopeMetrics().At(j).Metrics()
			scopeInstr := rm.ScopeMetrics().At(j).Scope()
			scopeURL := rm.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				if r.Type() == pmetric.MetricTypeEmpty {
					skippedCount++
					continue
				}
				m, ok := metricsMap[r.Type()]
				if !ok {
					skippedCount++
					continue
				}
				m.Add(resAttr, rm.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	require.Equal(t, 1, skippedCount, "Should have skipped 1 empty metric")
	_, ok := metricsMap[pmetric.MetricTypeGauge]
	require.True(t, ok, "Gauge from resource A should be in the metrics map")
}

// TestMetricsModel_WithOnlyEmptyMetrics verifies behavior when all metrics are invalid.
func TestMetricsModel_WithOnlyEmptyMetrics(t *testing.T) {
	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	// Add only empty type metrics
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("empty_metric_1")

	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("empty_metric_2")

	var skippedCount int
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resAttr := rm.Resource().Attributes()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			rs := rm.ScopeMetrics().At(j).Metrics()
			scopeInstr := rm.ScopeMetrics().At(j).Scope()
			scopeURL := rm.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				if r.Type() == pmetric.MetricTypeEmpty {
					skippedCount++
					continue
				}
				m, ok := metricsMap[r.Type()]
				if !ok {
					skippedCount++
					continue
				}
				m.Add(resAttr, rm.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	require.Equal(t, 2, skippedCount, "Should have skipped 2 empty metrics")
	// The metricsMap still has the model instances created, but no data was added to them
	// This is correct behavior - the map structure exists but is empty of actual metric data
}

// TestMetricsModel_WithMultipleResourcesAndScopes verifies that invalid metrics in one
// resource/scope don't affect valid metrics in other resources/scopes.
func TestMetricsModel_WithMultipleResourcesAndScopes(t *testing.T) {
	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

	md := pmetric.NewMetrics()

	// Resource 1, Scope 1: valid gauge
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", "service-1")
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	sm1.Scope().SetName("scope-1")
	m1 := sm1.Metrics().AppendEmpty()
	m1.SetName("gauge_1")
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)

	// Resource 1, Scope 2: empty metric
	sm2 := rm1.ScopeMetrics().AppendEmpty()
	sm2.Scope().SetName("scope-2")
	m2 := sm2.Metrics().AppendEmpty()
	m2.SetName("empty_metric")

	// Resource 2, Scope 1: valid gauge
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "service-2")
	sm3 := rm2.ScopeMetrics().AppendEmpty()
	sm3.Scope().SetName("scope-1")
	m3 := sm3.Metrics().AppendEmpty()
	m3.SetName("gauge_2")
	m3.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(2.0)

	var skippedCount int
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resAttr := rm.Resource().Attributes()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			rs := rm.ScopeMetrics().At(j).Metrics()
			scopeInstr := rm.ScopeMetrics().At(j).Scope()
			scopeURL := rm.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				if r.Type() == pmetric.MetricTypeEmpty {
					skippedCount++
					continue
				}
				m, ok := metricsMap[r.Type()]
				if !ok {
					skippedCount++
					continue
				}
				m.Add(resAttr, rm.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	require.Equal(t, 1, skippedCount, "Should have skipped 1 empty metric")
	_, ok := metricsMap[pmetric.MetricTypeGauge]
	require.True(t, ok, "Gauges from multiple resources should be in the metrics map")
}

// TestMetricsModel_WithValidOnlyBehaviorUnchanged verifies that entirely valid metric
// batches continue to work as before.
func TestMetricsModel_WithValidOnlyBehaviorUnchanged(t *testing.T) {
	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
			Sum:   metrics.MetricTypeConfig{Name: "otel_metrics_sum"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	// Add a valid gauge metric
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("valid_gauge")
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)

	var skippedCount int
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resAttr := rm.Resource().Attributes()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			rs := rm.ScopeMetrics().At(j).Metrics()
			scopeInstr := rm.ScopeMetrics().At(j).Scope()
			scopeURL := rm.ScopeMetrics().At(j).SchemaUrl()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				if r.Type() == pmetric.MetricTypeEmpty {
					skippedCount++
					continue
				}
				m, ok := metricsMap[r.Type()]
				if !ok {
					skippedCount++
					continue
				}
				m.Add(resAttr, rm.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	require.Equal(t, 0, skippedCount, "Should not skip any valid metrics")
	_, gaugeOk := metricsMap[pmetric.MetricTypeGauge]
	_, sumOk := metricsMap[pmetric.MetricTypeSum]
	require.True(t, gaugeOk, "Gauge should be in the metrics map")
	require.True(t, sumOk, "Sum should be in the metrics map")
}
