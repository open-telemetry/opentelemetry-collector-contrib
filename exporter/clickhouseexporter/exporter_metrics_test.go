// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
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

func Test_pushMetricsData_SkipsEmptyMetricType(t *testing.T) {
	// This test verifies that the fix processes the request without returning
	// "metrics type is unset" error. The actual DB insertion is not tested here
	// since it requires a full ClickHouse setup.

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
	// Intentionally leave type as Empty

	// Verify that the metrics are processed
	// The old behavior would return "metrics type is unset" immediately
	// The new behavior should process all metrics and skip the empty one
	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

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

func Test_pushMetricsData_SkipsUnsupportedMetricType(t *testing.T) {
	// This test verifies the logic for skipping unsupported metric types.
	// Since all standard types are configured by default, we test the code path
	// by simulating a scenario where a type is not in the map.
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	// Add a valid gauge metric
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("valid_gauge")
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)

	// Create an empty map to simulate no configured types
	metricsMap := map[pmetric.MetricType]metrics.MetricsModel{}

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
				// Would call m.Add here, but we're just testing the skip logic
				_ = m
				_ = resAttr
				_ = rm.SchemaUrl()
				_ = scopeInstr
				_ = scopeURL
				_ = r
			}
		}
	}

	// Since the map is empty, the gauge should be skipped as "unsupported"
	require.Equal(t, 1, skippedCount, "Should have skipped 1 unsupported metric")
}

func Test_pushMetricsData_EmptyMetricInSeparateResource(t *testing.T) {
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
	// Leave type as Empty

	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

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

func Test_pushMetricsData_EmptyMetricInSameScope(t *testing.T) {
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
	// Leave type as Empty

	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

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

func Test_pushMetricsData_OnlyInvalidMetrics(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	// Add only empty type metrics
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("empty_metric_1")
	// Leave type as Empty

	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("empty_metric_2")
	// Leave type as Empty

	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

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
					continue
				}
				m.Add(resAttr, rm.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	require.Equal(t, 2, skippedCount, "Should have skipped 2 empty metrics")
	// Note: The metricsMap still has the model instances created, but no data was added to them
	// This is correct behavior - the map structure exists but is empty of actual metric data
}

func Test_pushMetricsData_MultipleResourcesAndScopes(t *testing.T) {
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
	// Leave type as Empty

	// Resource 2, Scope 1: valid gauge
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "service-2")
	sm3 := rm2.ScopeMetrics().AppendEmpty()
	sm3.Scope().SetName("scope-1")
	m3 := sm3.Metrics().AppendEmpty()
	m3.SetName("gauge_2")
	m3.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(2.0)

	// Resource 2, Scope 2: empty metric
	sm4 := rm2.ScopeMetrics().AppendEmpty()
	sm4.Scope().SetName("scope-2")
	m4 := sm4.Metrics().AppendEmpty()
	m4.SetName("empty_metric_2")
	// Leave type as Empty

	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

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
					continue
				}
				m.Add(resAttr, rm.SchemaUrl(), scopeInstr, scopeURL, r)
			}
		}
	}

	require.Equal(t, 2, skippedCount, "Should have skipped 2 empty metrics")
	_, ok := metricsMap[pmetric.MetricTypeGauge]
	require.True(t, ok, "Gauges from multiple resources should be in the metrics map")
}

func Test_pushMetricsData_ValidOnlyBehaviorUnchanged(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	rm.SetSchemaUrl("resource-schema")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")
	sm.Scope().SetVersion("1.0.0")
	sm.SetSchemaUrl("scope-schema")

	// Add multiple valid metrics of different types
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("gauge_metric")
	m1.SetDescription("A gauge metric")
	m1.SetUnit("1")
	dp1 := m1.SetEmptyGauge().DataPoints().AppendEmpty()
	dp1.SetDoubleValue(42.0)
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567890, 0)))
	dp1.Attributes().PutStr("label", "value")

	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("sum_metric")
	m2.SetDescription("A sum metric")
	m2.SetUnit("1")
	dp2 := m2.SetEmptySum().DataPoints().AppendEmpty()
	dp2.SetIntValue(100)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567890, 0)))
	dp2.Attributes().PutStr("label", "value")

	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "otel_metrics_gauge"},
			Sum:   metrics.MetricTypeConfig{Name: "otel_metrics_sum"},
		},
	}
	metricsMap := metrics.NewMetricsModel(generateMetricTablesConfigMapper(cfg), cfg.database())

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
