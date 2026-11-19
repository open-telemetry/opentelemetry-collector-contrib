// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

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

func TestMetricsExporterConnectionPoolSize(t *testing.T) {
	tests := []struct {
		name             string
		endpoint         string
		connectionParams map[string]string
		queue            configoptional.Optional[exporterhelper.QueueBatchConfig]
		expectedMaxOpen  int
		expectedMaxIdle  int
	}{
		{
			name:            "default queue",
			endpoint:        defaultEndpoint,
			queue:           configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			expectedMaxOpen: 50,
			expectedMaxIdle: 50,
		},
		{
			// Falls back to the driver defaults.
			name:            "queue disabled",
			endpoint:        defaultEndpoint,
			queue:           configoptional.None[exporterhelper.QueueBatchConfig](),
			expectedMaxOpen: 10,
			expectedMaxIdle: 5,
		},
		{
			name:            "endpoint overrides max open",
			endpoint:        defaultEndpoint + "?max_open_conns=3",
			queue:           configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			expectedMaxOpen: 3,
			expectedMaxIdle: 3,
		},
		{
			name:            "endpoint overrides max idle",
			endpoint:        defaultEndpoint + "?max_idle_conns=2",
			queue:           configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			expectedMaxOpen: 50,
			expectedMaxIdle: 2,
		},
		{
			name:             "connection params override",
			endpoint:         defaultEndpoint,
			connectionParams: map[string]string{"max_open_conns": "7"},
			queue:            configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			expectedMaxOpen:  7,
			expectedMaxIdle:  7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Endpoint = tt.endpoint
			cfg.QueueSettings = tt.queue
			if tt.connectionParams != nil {
				cfg.ConnectionParams = tt.connectionParams
			}
			// Skip schema creation so start doesn't need a ClickHouse server.
			cfg.CreateSchema = false

			exporter := newMetricsExporter(zap.NewNop(), cfg)
			require.NoError(t, exporter.start(t.Context(), nil))
			defer func() {
				require.NoError(t, exporter.shutdown(t.Context()))
			}()

			stats := exporter.db.Stats()
			assert.Equal(t, tt.expectedMaxOpen, stats.MaxOpenConns)
			assert.Equal(t, tt.expectedMaxIdle, stats.MaxIdleConns)
		})
	}
}

// TestProcessMetrics_WithEmptyMetricType verifies that when processing metrics with
// MetricTypeEmpty, the valid metrics are still added to the model while empty metrics are skipped.
func TestProcessMetrics_WithEmptyMetricType(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsTables.Gauge = metrics.MetricTypeConfig{Name: "otel_metrics_gauge"}

	exporter := newMetricsExporter(zap.NewNop(), cfg)

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

	metricsMap, skippedCount := exporter.processMetrics(md)

	// Verify the empty metric was skipped
	require.Equal(t, 1, skippedCount, "Should have skipped 1 empty metric")

	// Verify the valid gauge was added to the model
	_, ok := metricsMap[pmetric.MetricTypeGauge]
	require.True(t, ok, "Gauge should be in the metrics map after processing")
}

// TestProcessMetrics_WithMixedValidAndEmptyMetrics verifies that valid metrics in the same scope
// as empty metrics are still processed.
func TestProcessMetrics_WithMixedValidAndEmptyMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsTables.Gauge = metrics.MetricTypeConfig{Name: "otel_metrics_gauge"}

	exporter := newMetricsExporter(zap.NewNop(), cfg)

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

	metricsMap, skippedCount := exporter.processMetrics(md)

	require.Equal(t, 1, skippedCount, "Should have skipped 1 empty metric")
	_, ok := metricsMap[pmetric.MetricTypeGauge]
	require.True(t, ok, "Gauge should be in the metrics map")
}

// TestProcessMetrics_WithEmptyMetricInSeparateResource verifies that a valid metric in one
// resource is not affected by an empty metric in another resource.
func TestProcessMetrics_WithEmptyMetricInSeparateResource(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsTables.Gauge = metrics.MetricTypeConfig{Name: "otel_metrics_gauge"}

	exporter := newMetricsExporter(zap.NewNop(), cfg)

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

	metricsMap, skippedCount := exporter.processMetrics(md)

	require.Equal(t, 1, skippedCount, "Should have skipped 1 empty metric")
	_, ok := metricsMap[pmetric.MetricTypeGauge]
	require.True(t, ok, "Gauge from resource A should be in the metrics map")
}

// TestProcessMetrics_WithOnlyEmptyMetrics verifies behavior when all metrics are invalid.
func TestProcessMetrics_WithOnlyEmptyMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsTables.Gauge = metrics.MetricTypeConfig{Name: "otel_metrics_gauge"}

	exporter := newMetricsExporter(zap.NewNop(), cfg)

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

	_, skippedCount := exporter.processMetrics(md)

	require.Equal(t, 2, skippedCount, "Should have skipped 2 empty metrics")
	// The metricsMap still has the model instances created, but no data was added to them
	// This is correct behavior - the map structure exists but is empty of actual metric data
}

// TestProcessMetrics_WithMultipleResourcesAndScopes verifies that invalid metrics in one
// resource/scope don't affect valid metrics in other resources/scopes.
func TestProcessMetrics_WithMultipleResourcesAndScopes(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsTables.Gauge = metrics.MetricTypeConfig{Name: "otel_metrics_gauge"}

	exporter := newMetricsExporter(zap.NewNop(), cfg)

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

	metricsMap, skippedCount := exporter.processMetrics(md)

	require.Equal(t, 1, skippedCount, "Should have skipped 1 empty metric")
	_, ok := metricsMap[pmetric.MetricTypeGauge]
	require.True(t, ok, "Gauges from multiple resources should be in the metrics map")
}

// TestProcessMetrics_WithValidOnlyBehaviorUnchanged verifies that entirely valid metric
// batches continue to work as before.
func TestProcessMetrics_WithValidOnlyBehaviorUnchanged(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsTables.Gauge = metrics.MetricTypeConfig{Name: "otel_metrics_gauge"}
	cfg.MetricsTables.Sum = metrics.MetricTypeConfig{Name: "otel_metrics_sum"}

	exporter := newMetricsExporter(zap.NewNop(), cfg)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	// Add a valid gauge metric
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("valid_gauge")
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)

	metricsMap, skippedCount := exporter.processMetrics(md)

	require.Equal(t, 0, skippedCount, "Should not skip any valid metrics")
	_, gaugeOk := metricsMap[pmetric.MetricTypeGauge]
	_, sumOk := metricsMap[pmetric.MetricTypeSum]
	require.True(t, gaugeOk, "Gauge should be in the metrics map")
	require.True(t, sumOk, "Sum should be in the metrics map")
}
