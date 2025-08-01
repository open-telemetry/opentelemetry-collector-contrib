// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var testComponentType = component.MustNewType("routing_test")

func TestMetricsRouting(t *testing.T) {
	tests := []struct {
		name           string
		config         config.MetricsRoutingConfig
		scopeName      string
		resourceAttrs  map[string]string
		expectedTarget string
	}{
		{
			name: "routing_disabled",
			config: config.MetricsRoutingConfig{
				RoutingConfig: config.RoutingConfig{
					Enabled: false,
				},
			},
			scopeName:      "datadog.trace.metrics",
			expectedTarget: config.TargetDatadog,
		},
		{
			name: "route_to_otlp_by_scope_name",
			config: config.MetricsRoutingConfig{
				RoutingConfig: config.RoutingConfig{
					Enabled:      true,
					OTLPEndpoint: "https://trace.agent.datadoghq.com/api/v0.2/stats",
					OTLPHeaders: map[string]configopaque.String{
						"Dd-Protocol":                  "otlp",
						"Dd-Api-Key":                   "${DD_API_KEY}",
						"X-Datadog-Reported-Languages": "java",
					},
					Rules: []config.RoutingRule{
						{
							Name: "datadog_trace_metrics",
							Condition: config.RoutingCondition{
								InstrumentationScopeName: "datadog.trace.metrics",
							},
							Target: config.TargetOTLP,
						},
					},
				},
			},
			scopeName:      "datadog.trace.metrics",
			expectedTarget: config.TargetOTLP,
		},
		{
			name: "route_to_datadog_by_default",
			config: config.MetricsRoutingConfig{
				RoutingConfig: config.RoutingConfig{
					Enabled:      true,
					OTLPEndpoint: "https://trace.agent.datadoghq.com/api/v0.2/stats",
					Rules: []config.RoutingRule{
						{
							Name: "datadog_trace_metrics",
							Condition: config.RoutingCondition{
								InstrumentationScopeName: "datadog.trace.metrics",
							},
							Target: config.TargetOTLP,
						},
					},
				},
			},
			scopeName:      "other.metrics",
			expectedTarget: config.TargetDatadog,
		},
		{
			name: "route_to_otlp_by_resource_attribute",
			config: config.MetricsRoutingConfig{
				RoutingConfig: config.RoutingConfig{
					Enabled:      true,
					OTLPEndpoint: "https://trace.agent.datadoghq.com/api/v0.2/stats",
					Rules: []config.RoutingRule{
						{
							Name: "special_service",
							Condition: config.RoutingCondition{
								ResourceAttributes: map[string]string{
									"service.name": "special-service",
								},
							},
							Target: config.TargetOTLP,
						},
					},
				},
			},
			scopeName: "regular.metrics",
			resourceAttrs: map[string]string{
				"service.name": "special-service",
			},
			expectedTarget: config.TargetOTLP,
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

func TestMetricsRouterDisabled(t *testing.T) {
	var datadogCalled bool

	datadogPushFunc := func(ctx context.Context, md pmetric.Metrics) error {
		datadogCalled = true
		return nil
	}

	config := config.MetricsRoutingConfig{
		RoutingConfig: config.RoutingConfig{
			Enabled: false, // Routing disabled
		},
	}

	logger := zap.NewNop()

	// Create metrics router using the new API
	settings := MetricsRouterSettings{
		DatadogPushFunc:  datadogPushFunc,
		RoutingConfig:    config,
		APIKey:           "test-api-key",
		Site:             "datadoghq.com",
		Logger:           logger,
		ExporterSettings: exportertest.NewNopSettings(testComponentType),
	}

	router, err := NewMetricsRouter(context.Background(), settings)
	require.NoError(t, err)
	require.NotNil(t, router)

	// Test that with routing disabled, all metrics go to Datadog
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("any.scope.name")
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric")

	datadogCalled = false
	err = router.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
	assert.True(t, datadogCalled, "Datadog exporter should be called when routing is disabled")
}

func TestMetricsRouterWithLogging(t *testing.T) {
	var datadogCalled bool

	datadogPushFunc := func(ctx context.Context, md pmetric.Metrics) error {
		datadogCalled = true
		return nil
	}

	config := config.MetricsRoutingConfig{
		RoutingConfig: config.RoutingConfig{
			Enabled:      true,
			OTLPEndpoint: "https://otlp.example.com",
			Rules: []config.RoutingRule{
				{
					Name: "datadog_trace_metrics",
					Condition: config.RoutingCondition{
						InstrumentationScopeName: "datadog.trace.metrics",
					},
					Target: config.TargetOTLP,
				},
			},
		},
	}

	// Create an observed logger to capture log messages
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(observedZapCore)

	// Create metrics router using the new API
	settings := MetricsRouterSettings{
		DatadogPushFunc:  datadogPushFunc,
		RoutingConfig:    config,
		APIKey:           "test-api-key",
		Site:             "datadoghq.com",
		Logger:           logger,
		ExporterSettings: exportertest.NewNopSettings(testComponentType),
	}

	router, err := NewMetricsRouter(context.Background(), settings)
	require.NoError(t, err)
	require.NotNil(t, router)

	// Verify router creation logs
	logs := observedLogs.TakeAll()
	require.Len(t, logs, 1)
	assert.Equal(t, "Created metrics router", logs[0].Message)
	assert.Equal(t, zap.InfoLevel, logs[0].Level)
	assert.Equal(t, true, logs[0].ContextMap()["routing_enabled"])
	assert.Equal(t, "https://otlp.example.com", logs[0].ContextMap()["otlp_endpoint"])
	assert.Equal(t, true, logs[0].ContextMap()["has_otlp_exporter"])

	// Test routing to Datadog (default) with logging
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("other.metrics")
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric")

	datadogCalled = false
	err = router.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
	assert.True(t, datadogCalled, "Datadog exporter should be called")

	// Verify Datadog routing logs
	logs = observedLogs.TakeAll()
	require.GreaterOrEqual(t, len(logs), 2)

	var debugNoMatchLog observer.LoggedEntry
	var foundDebugNoMatch, foundDebugDatadog bool

	for _, log := range logs {
		switch log.Message {
		case "No metrics matched OTLP routing rules, using standard Datadog exporter":
			debugNoMatchLog = log
			foundDebugNoMatch = true
		case "Routing metrics to Datadog":
			foundDebugDatadog = true
		}
	}

	assert.True(t, foundDebugNoMatch, "Should have debug no match log")
	assert.True(t, foundDebugDatadog, "Should have debug Datadog routing log")

	if foundDebugNoMatch {
		assert.Equal(t, zap.DebugLevel, debugNoMatchLog.Level)
		assert.Equal(t, int64(1), debugNoMatchLog.ContextMap()["resource_metrics_count"])
		assert.Equal(t, int64(1), debugNoMatchLog.ContextMap()["total_scope_metrics"])
	}
}

func TestMetricsRouterCapabilities(t *testing.T) {
	datadogPushFunc := func(ctx context.Context, md pmetric.Metrics) error {
		return nil
	}

	config := config.MetricsRoutingConfig{
		RoutingConfig: config.RoutingConfig{
			Enabled: false,
		},
	}

	settings := MetricsRouterSettings{
		DatadogPushFunc:  datadogPushFunc,
		RoutingConfig:    config,
		APIKey:           "test-api-key",
		Site:             "datadoghq.com",
		Logger:           zap.NewNop(),
		ExporterSettings: exportertest.NewNopSettings(testComponentType),
	}

	router, err := NewMetricsRouter(context.Background(), settings)
	require.NoError(t, err)
	require.NotNil(t, router)

	capabilities := router.Capabilities()
	assert.False(t, capabilities.MutatesData, "Router should not mutate data")
}

func TestMetricsRouterLifecycle(t *testing.T) {
	datadogPushFunc := func(ctx context.Context, md pmetric.Metrics) error {
		return nil
	}

	config := config.MetricsRoutingConfig{
		RoutingConfig: config.RoutingConfig{
			Enabled: false,
		},
	}

	settings := MetricsRouterSettings{
		DatadogPushFunc:  datadogPushFunc,
		RoutingConfig:    config,
		APIKey:           "test-api-key",
		Site:             "datadoghq.com",
		Logger:           zap.NewNop(),
		ExporterSettings: exportertest.NewNopSettings(testComponentType),
	}

	router, err := NewMetricsRouter(context.Background(), settings)
	require.NoError(t, err)
	require.NotNil(t, router)

	// Test Start (should not fail even with no OTLP exporter)
	err = router.Start(context.Background(), nil)
	assert.NoError(t, err)

	// Test Shutdown (should not fail even with no OTLP exporter)
	err = router.Shutdown(context.Background())
	assert.NoError(t, err)
}
