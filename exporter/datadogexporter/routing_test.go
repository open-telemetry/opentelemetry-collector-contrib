// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// Compile-time interface implementation checks
var (
	_ consumer.Metrics = (*mockMetricsConsumer)(nil)
	_ exporter.Metrics = (*mockMetricsExporter)(nil)
	_ consumer.Traces  = (*mockTracesConsumer)(nil)
	_ exporter.Traces  = (*mockTracesExporter)(nil)
	_ consumer.Logs    = (*mockLogsConsumer)(nil)
	_ exporter.Logs    = (*mockLogsExporter)(nil)
)

func TestMetricsRouting(t *testing.T) {
	tests := []struct {
		name           string
		config         datadogconfig.MetricsRoutingConfig
		scopeName      string
		resourceAttrs  map[string]string
		expectedTarget string
	}{
		{
			name: "routing_disabled",
			config: datadogconfig.MetricsRoutingConfig{
				RoutingConfig: datadogconfig.RoutingConfig{
					Enabled: false,
				},
			},
			scopeName:      "datadog.trace.metrics",
			expectedTarget: datadogconfig.TargetDatadog,
		},
		{
			name: "route_to_otlp_by_scope_name",
			config: datadogconfig.MetricsRoutingConfig{
				RoutingConfig: datadogconfig.RoutingConfig{
					Enabled:      true,
					OTLPEndpoint: "https://trace.agent.datadoghq.com/api/v0.2/stats",
					OTLPHeaders: map[string]configopaque.String{
						"Dd-Protocol":                  "otlp",
						"Dd-Api-Key":                   "${DD_API_KEY}",
						"X-Datadog-Reported-Languages": "java",
					},
					Rules: []datadogconfig.RoutingRule{
						{
							Name: "datadog_trace_metrics",
							Condition: datadogconfig.RoutingCondition{
								InstrumentationScopeName: "datadog.trace.metrics",
							},
							Target: datadogconfig.TargetOTLP,
						},
					},
				},
			},
			scopeName:      "datadog.trace.metrics",
			expectedTarget: datadogconfig.TargetOTLP,
		},
		{
			name: "route_to_datadog_by_default",
			config: datadogconfig.MetricsRoutingConfig{
				RoutingConfig: datadogconfig.RoutingConfig{
					Enabled:      true,
					OTLPEndpoint: "https://trace.agent.datadoghq.com/api/v0.2/stats",
					Rules: []datadogconfig.RoutingRule{
						{
							Name: "datadog_trace_metrics",
							Condition: datadogconfig.RoutingCondition{
								InstrumentationScopeName: "datadog.trace.metrics",
							},
							Target: datadogconfig.TargetOTLP,
						},
					},
				},
			},
			scopeName:      "other.metrics",
			expectedTarget: datadogconfig.TargetDatadog,
		},
		{
			name: "route_to_otlp_by_resource_attribute",
			config: datadogconfig.MetricsRoutingConfig{
				RoutingConfig: datadogconfig.RoutingConfig{
					Enabled:      true,
					OTLPEndpoint: "https://trace.agent.datadoghq.com/api/v0.2/stats",
					Rules: []datadogconfig.RoutingRule{
						{
							Name: "special_service",
							Condition: datadogconfig.RoutingCondition{
								ResourceAttributes: map[string]string{
									"service.name": "special-service",
								},
							},
							Target: datadogconfig.TargetOTLP,
						},
					},
				},
			},
			scopeName: "regular.metrics",
			resourceAttrs: map[string]string{
				"service.name": "special-service",
			},
			expectedTarget: datadogconfig.TargetOTLP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()

			// Set resource attributes
			if tt.resourceAttrs != nil {
				for key, value := range tt.resourceAttrs {
					rm.Resource().Attributes().PutStr(key, value)
				}
			}

			sm := rm.ScopeMetrics().AppendEmpty()
			sm.Scope().SetName(tt.scopeName)

			// Add a simple metric
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("test.metric")

			target := tt.config.DetermineMetricRoute(rm, sm)
			assert.Equal(t, tt.expectedTarget, target)
		})
	}
}

func TestRoutingMetricsExporter(t *testing.T) {
	var datadogCalled, otlpCalled bool

	datadogPushFunc := func(ctx context.Context, md pmetric.Metrics) error {
		datadogCalled = true
		return nil
	}

	// Mock OTLP exporter
	mockOTLPExporter := &mockMetricsExporter{
		consumeFunc: func(ctx context.Context, md pmetric.Metrics) error {
			otlpCalled = true
			return nil
		},
	}

	config := datadogconfig.MetricsRoutingConfig{
		RoutingConfig: datadogconfig.RoutingConfig{
			Enabled:      true,
			OTLPEndpoint: "https://otlp.example.com",
			Rules: []datadogconfig.RoutingRule{
				{
					Name: "datadog_trace_metrics",
					Condition: datadogconfig.RoutingCondition{
						InstrumentationScopeName: "datadog.trace.metrics",
					},
					Target: datadogconfig.TargetOTLP,
				},
			},
		},
	}

	exporter := newRoutingMetricsExporter(datadogPushFunc, mockOTLPExporter, config)

	// Test routing to OTLP
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("datadog.trace.metrics")
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric")

	datadogCalled = false
	otlpCalled = false
	err := exporter.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
	assert.False(t, datadogCalled, "Datadog exporter should not be called")
	assert.True(t, otlpCalled, "OTLP exporter should be called")

	// Test routing to Datadog (default)
	md2 := pmetric.NewMetrics()
	rm2 := md2.ResourceMetrics().AppendEmpty()
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	sm2.Scope().SetName("other.metrics")
	metric2 := sm2.Metrics().AppendEmpty()
	metric2.SetName("test.metric")

	datadogCalled = false
	otlpCalled = false
	err = exporter.ConsumeMetrics(context.Background(), md2)
	require.NoError(t, err)
	assert.True(t, datadogCalled, "Datadog exporter should be called")
	assert.False(t, otlpCalled, "OTLP exporter should not be called")
}

// Mock implementations for testing
type mockMetricsConsumer struct {
	consumeFunc func(ctx context.Context, md pmetric.Metrics) error
}

func (m *mockMetricsConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if m.consumeFunc != nil {
		return m.consumeFunc(ctx, md)
	}
	return nil
}

func (m *mockMetricsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

// mockMetricsExporter implements both consumer.Metrics and exporter.Metrics
type mockMetricsExporter struct {
	consumeFunc func(ctx context.Context, md pmetric.Metrics) error
}

func (m *mockMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if m.consumeFunc != nil {
		return m.consumeFunc(ctx, md)
	}
	return nil
}

func (m *mockMetricsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m *mockMetricsExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (m *mockMetricsExporter) Shutdown(ctx context.Context) error {
	return nil
}

// Mock traces implementations
type mockTracesConsumer struct {
	consumeFunc func(ctx context.Context, td ptrace.Traces) error
}

func (m *mockTracesConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if m.consumeFunc != nil {
		return m.consumeFunc(ctx, td)
	}
	return nil
}

func (m *mockTracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

type mockTracesExporter struct {
	consumeFunc func(ctx context.Context, td ptrace.Traces) error
}

func (m *mockTracesExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if m.consumeFunc != nil {
		return m.consumeFunc(ctx, td)
	}
	return nil
}

func (m *mockTracesExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m *mockTracesExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (m *mockTracesExporter) Shutdown(ctx context.Context) error {
	return nil
}

// Mock logs implementations
type mockLogsConsumer struct {
	consumeFunc func(ctx context.Context, ld plog.Logs) error
}

func (m *mockLogsConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if m.consumeFunc != nil {
		return m.consumeFunc(ctx, ld)
	}
	return nil
}

func (m *mockLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

type mockLogsExporter struct {
	consumeFunc func(ctx context.Context, ld plog.Logs) error
}

func (m *mockLogsExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if m.consumeFunc != nil {
		return m.consumeFunc(ctx, ld)
	}
	return nil
}

func (m *mockLogsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m *mockLogsExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (m *mockLogsExporter) Shutdown(ctx context.Context) error {
	return nil
}
