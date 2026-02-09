// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadatatest"
)

func TestExporterCreator_Start(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.WatchObservers = []component.ID{component.MustNewID("k8s_observer")}

	params := exportertest.NewNopSettings(metadata.Type)
	ec, err := newExporterCreator(params, cfg)
	require.NoError(t, err)
	require.NotNil(t, ec)

	// Test with proper host but no observers - componenttest.NewNopHost doesn't implement hostcapabilities.ComponentFactory
	// so we expect a different error
	err = ec.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not compatible")
}

func TestExporterCreator_Shutdown(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.WatchObservers = []component.ID{}

	params := exportertest.NewNopSettings(metadata.Type)
	ec, err := newExporterCreator(params, cfg)
	require.NoError(t, err)
	require.NotNil(t, ec)

	// Test shutdown with no observers - need a host that implements ComponentFactory
	// Use the mockHostWithFactory from exporter_runner_test.go
	mockHost := &mockHostWithFactory{
		factories: make(map[component.Type]exporter.Factory),
	}
	err = ec.Start(context.Background(), mockHost)
	require.NoError(t, err)

	err = ec.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestExporterCreator_ConsumeMetrics_NonRoutable(t *testing.T) {
	// Test that non-routable metric points are counted
	tt := componenttest.NewTelemetry()
	telemetrySettings := tt.NewTelemetrySettings()
	telemetry, err := metadata.NewTelemetryBuilder(telemetrySettings)
	require.NoError(t, err)
	defer telemetry.Shutdown()

	cfg := createDefaultConfig().(*Config)
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "app",
			EndpointProperty:  "labels.app",
		},
	}
	// No default exporters configured
	cfg.DefaultExporters = []component.ID{}

	params := metadatatest.NewSettings(tt)
	ec, err := newExporterCreator(params, cfg)
	require.NoError(t, err)
	ec.telemetry = telemetry
	ec.router = newTelemetryRouter(cfg.Routing.Rules, telemetry)

	// Create metrics that don't match any routing rules
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service", "unknown") // Doesn't match "app" rule

	// Add a metric with data points
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test_metric")
	metric.SetEmptyGauge()
	dp1 := metric.Gauge().DataPoints().AppendEmpty()
	dp1.SetIntValue(10)
	dp2 := metric.Gauge().DataPoints().AppendEmpty()
	dp2.SetIntValue(20)

	// Create another metric with more data points
	metric2 := sm.Metrics().AppendEmpty()
	metric2.SetName("test_metric2")
	metric2.SetEmptySum()
	dp3 := metric2.Sum().DataPoints().AppendEmpty()
	dp3.SetIntValue(30)

	// Consume metrics - should record 3 non-routable metric points
	err = ec.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)

	// Verify the metric was recorded
	metadatatest.AssertEqualExporterCreatorNonroutableMetricPointsTotal(t, tt, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(),
			Value:      3, // 3 data points total
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestExporterCreator_ConsumeMetrics_NonRoutable_WithDefaultExporterFailure(t *testing.T) {
	// Test that non-routable metric points are counted when default exporter fails
	tt := componenttest.NewTelemetry()
	telemetrySettings := tt.NewTelemetrySettings()
	telemetry, err := metadata.NewTelemetryBuilder(telemetrySettings)
	require.NoError(t, err)
	defer telemetry.Shutdown()

	cfg := createDefaultConfig().(*Config)
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "app",
			EndpointProperty:  "labels.app",
		},
	}

	params := metadatatest.NewSettings(tt)
	ec, err := newExporterCreator(params, cfg)
	require.NoError(t, err)
	ec.telemetry = telemetry
	ec.router = newTelemetryRouter(cfg.Routing.Rules, telemetry)

	// Create a failing default exporter
	failingExporter := &failingMetricsExporter{}
	ec.defaultExporters = map[component.ID]component.Component{
		component.MustNewID("failing"): failingExporter,
	}

	// Create metrics that don't match any routing rules
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service", "unknown") // Doesn't match "app" rule

	// Add a metric with data points
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test_metric")
	metric.SetEmptyGauge()
	dp1 := metric.Gauge().DataPoints().AppendEmpty()
	dp1.SetIntValue(10)
	dp2 := metric.Gauge().DataPoints().AppendEmpty()
	dp2.SetIntValue(20)

	// Consume metrics - should record 2 non-routable metric points because default exporter failed
	err = ec.ConsumeMetrics(context.Background(), metrics)
	require.Error(t, err) // Should have error from failing exporter

	// Verify the metric was recorded
	metadatatest.AssertEqualExporterCreatorNonroutableMetricPointsTotal(t, tt, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(),
			Value:      2, // 2 data points total
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestExporterCreator_ConsumeMetrics_NonRoutable_WithDefaultExporterSuccess(t *testing.T) {
	// Test that non-routable metric points are NOT counted when default exporter succeeds
	tt := componenttest.NewTelemetry()
	telemetrySettings := tt.NewTelemetrySettings()
	telemetry, err := metadata.NewTelemetryBuilder(telemetrySettings)
	require.NoError(t, err)
	defer telemetry.Shutdown()

	cfg := createDefaultConfig().(*Config)
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "app",
			EndpointProperty:  "labels.app",
		},
	}

	params := metadatatest.NewSettings(tt)
	ec, err := newExporterCreator(params, cfg)
	require.NoError(t, err)
	ec.telemetry = telemetry
	ec.router = newTelemetryRouter(cfg.Routing.Rules, telemetry)

	// Create a successful default exporter
	successfulExporter := &nopExporter{}
	ec.defaultExporters = map[component.ID]component.Component{
		component.MustNewID("successful"): successfulExporter,
	}

	// Create metrics that don't match any routing rules
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service", "unknown") // Doesn't match "app" rule

	// Add a metric with data points
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test_metric")
	metric.SetEmptyGauge()
	dp1 := metric.Gauge().DataPoints().AppendEmpty()
	dp1.SetIntValue(10)

	// Consume metrics - should NOT record non-routable metric points because default exporter succeeded
	err = ec.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)

	// Verify the metric was NOT recorded - check that metric doesn't exist or has no data points
	got, err := tt.GetMetric("otelcol_exporter_creator_nonroutable_metric_points_total")
	if err == nil {
		// If metric exists, it should have no data points or zero value
		sum, ok := got.Data.(metricdata.Sum[int64])
		if ok && sum.DataPoints != nil && len(sum.DataPoints) > 0 {
			// If there are data points, they should all be zero
			for _, dp := range sum.DataPoints {
				assert.Equal(t, int64(0), dp.Value, "non-routable metric should be 0 when default exporter succeeds")
			}
		}
	}
	// If metric doesn't exist (err != nil), that's also fine - it means it was never recorded
}

func TestExporterCreator_ConsumeTraces(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Routing.Rules = []RoutingRule{
		{
			ResourceAttribute: "service.name",
			EndpointProperty:  "labels.service",
		},
	}

	params := exportertest.NewNopSettings(metadata.Type)
	ec, err := newExporterCreator(params, cfg)
	require.NoError(t, err)
	require.NotNil(t, ec)

	// Test that ConsumeTraces doesn't panic - just test with empty traces
	err = ec.ConsumeTraces(context.Background(), ptrace.NewTraces())
	require.NoError(t, err)
}

func TestExporterCreator_Capabilities(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	params := exportertest.NewNopSettings(metadata.Type)
	ec, err := newExporterCreator(params, cfg)
	require.NoError(t, err)

	caps := ec.Capabilities()
	assert.False(t, caps.MutatesData)
}

// failingMetricsExporter is a mock exporter that always fails
type failingMetricsExporter struct {
	exporter.Metrics
}

func (f *failingMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return errors.New("export failed")
}

