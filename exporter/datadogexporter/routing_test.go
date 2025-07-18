// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"context"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
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

	logger := zap.NewNop()
	exporter := newRoutingMetricsExporter(datadogPushFunc, mockOTLPExporter, config, logger)

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

func TestRoutingMetricsExporterLogging(t *testing.T) {
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

	// Create an observed logger to capture log messages
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(observedZapCore)

	exporter := newRoutingMetricsExporter(datadogPushFunc, mockOTLPExporter, config, logger)

	// Verify exporter creation logs
	logs := observedLogs.TakeAll()
	require.Len(t, logs, 1)
	assert.Equal(t, "Creating routing metrics exporter", logs[0].Message)
	assert.Equal(t, zap.InfoLevel, logs[0].Level)
	assert.Equal(t, true, logs[0].ContextMap()["routing_enabled"])
	assert.Equal(t, "https://otlp.example.com", logs[0].ContextMap()["otlp_endpoint"])
	assert.Equal(t, true, logs[0].ContextMap()["has_otlp_exporter"])

	// Test routing to OTLP with logging
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

	// Verify OTLP routing logs
	logs = observedLogs.TakeAll()
	require.GreaterOrEqual(t, len(logs), 2)

	// Find the relevant log entries
	var debugEvalLog, infoRoutingLog observer.LoggedEntry
	var foundDebugEval, foundDebugFound, foundInfoRouting, foundDebugSuccess bool

	for _, log := range logs {
		switch log.Message {
		case "Evaluated metric routing":
			debugEvalLog = log
			foundDebugEval = true
		case "Found metrics that should be routed to OTLP":
			foundDebugFound = true
		case "Routing metrics to OTLP endpoint":
			infoRoutingLog = log
			foundInfoRouting = true
		case "Successfully routed metrics to OTLP endpoint":
			foundDebugSuccess = true
		}
	}

	assert.True(t, foundDebugEval, "Should have debug evaluation log")
	assert.True(t, foundDebugFound, "Should have debug found log")
	assert.True(t, foundInfoRouting, "Should have info routing log")
	assert.True(t, foundDebugSuccess, "Should have debug success log")

	if foundDebugEval {
		assert.Equal(t, zap.DebugLevel, debugEvalLog.Level)
		assert.Equal(t, int64(0), debugEvalLog.ContextMap()["resource_index"])
		assert.Equal(t, int64(0), debugEvalLog.ContextMap()["scope_index"])
		assert.Equal(t, "otlp", debugEvalLog.ContextMap()["target"])
		assert.Equal(t, int64(1), debugEvalLog.ContextMap()["metrics_count"])
	}

	if foundInfoRouting {
		assert.Equal(t, zap.InfoLevel, infoRoutingLog.Level)
		assert.Equal(t, int64(1), infoRoutingLog.ContextMap()["resource_metrics_count"])
		assert.Equal(t, int64(1), infoRoutingLog.ContextMap()["total_scope_metrics"])
		assert.Equal(t, "https://otlp.example.com", infoRoutingLog.ContextMap()["otlp_endpoint"])
	}

	// Test routing to Datadog (default) with logging
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

func TestCreateOTLPMetricsExporterLogging(t *testing.T) {
	// Create an observed logger to capture log messages
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)

	set := exportertest.NewNopSettings(component.MustNewType("otlphttp"))
	set.Logger = zap.New(observedZapCore)

	routingConfig := datadogconfig.MetricsRoutingConfig{
		RoutingConfig: datadogconfig.RoutingConfig{
			Enabled:      true,
			OTLPEndpoint: "https://trace.agent.datadoghq.com/api/v0.2/stats",
			OTLPHeaders: map[string]configopaque.String{
				"Dd-Protocol":                  "otlp",
				"X-Datadog-Reported-Languages": "java",
			},
		},
	}
	apiKey := "test-api-key"

	_, err := createOTLPMetricsExporter(context.Background(), set, routingConfig, apiKey, "datadoghq.eu")
	require.NoError(t, err)

	// Verify logs
	logs := observedLogs.TakeAll()
	require.GreaterOrEqual(t, len(logs), 4)

	// Find the relevant log entries
	var infoCreatingLog observer.LoggedEntry
	var foundInfoCreating, foundDebugAddingHeader1, foundDebugAddingHeader2, foundDebugAddedAPIKey, foundDebugDisabled, foundInfoSuccess bool

	for _, log := range logs {
		switch log.Message {
		case "Creating OTLP metrics exporter for routing":
			infoCreatingLog = log
			foundInfoCreating = true
		case "Adding OTLP header":
			if log.ContextMap()["key"] == "Dd-Protocol" {
				foundDebugAddingHeader1 = true
			} else if log.ContextMap()["key"] == "X-Datadog-Reported-Languages" {
				foundDebugAddingHeader2 = true
			}
		case "Added Dd-Api-Key header to OTLP exporter":
			foundDebugAddedAPIKey = true
		case "Disabled retries and queuing for OTLP routing exporter to avoid double buffering":
			foundDebugDisabled = true
		case "Successfully created OTLP metrics exporter for routing":
			foundInfoSuccess = true
		}
	}

	assert.True(t, foundInfoCreating, "Should have info creating log")
	assert.True(t, foundDebugAddingHeader1, "Should have debug adding header log for Dd-Protocol")
	assert.True(t, foundDebugAddingHeader2, "Should have debug adding header log for X-Datadog-Reported-Languages")
	assert.True(t, foundDebugAddedAPIKey, "Should have debug added API key log")
	assert.True(t, foundDebugDisabled, "Should have debug disabled log")
	assert.True(t, foundInfoSuccess, "Should have info success log")

	if foundInfoCreating {
		assert.Equal(t, zap.InfoLevel, infoCreatingLog.Level)
		assert.Equal(t, "https://trace.agent.datadoghq.com/api/v0.2/stats", infoCreatingLog.ContextMap()["endpoint"])
		assert.Equal(t, int64(2), infoCreatingLog.ContextMap()["header_count"])
		assert.Equal(t, true, infoCreatingLog.ContextMap()["has_api_key"])
	}
}

func TestCreateOTLPMetricsExporterDisabledLogging(t *testing.T) {
	// Create an observed logger to capture log messages
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)

	set := exportertest.NewNopSettings(component.MustNewType("otlphttp"))
	set.Logger = zap.New(observedZapCore)

	// Test disabled routing
	routingConfig := datadogconfig.MetricsRoutingConfig{
		RoutingConfig: datadogconfig.RoutingConfig{
			Enabled: false,
		},
	}

	exporter, err := createOTLPMetricsExporter(context.Background(), set, routingConfig, "", "datadoghq.com")
	require.NoError(t, err)
	assert.Nil(t, exporter)

	// Verify logs
	logs := observedLogs.TakeAll()
	require.Len(t, logs, 1)
	assert.Equal(t, "OTLP metrics exporter not created", logs[0].Message)
	assert.Equal(t, zap.DebugLevel, logs[0].Level)
	assert.Equal(t, false, logs[0].ContextMap()["routing_enabled"])
	assert.Equal(t, "", logs[0].ContextMap()["otlp_endpoint"])
}

func TestCreateOTLPMetricsExporterAutoEndpoint(t *testing.T) {
	tests := []struct {
		name             string
		otlpEndpoint     string
		site             string
		expectedEndpoint string
		expectAutoGen    bool
	}{
		{
			name:             "auto_generate_default_site",
			otlpEndpoint:     "",
			site:             "datadoghq.com",
			expectedEndpoint: "https://trace.agent.datadoghq.com/api/v0.2/stats",
			expectAutoGen:    true,
		},
		{
			name:             "auto_generate_eu_site",
			otlpEndpoint:     "",
			site:             "datadoghq.eu",
			expectedEndpoint: "https://trace.agent.datadoghq.eu/api/v0.2/stats",
			expectAutoGen:    true,
		},
		{
			name:             "auto_generate_custom_site",
			otlpEndpoint:     "",
			site:             "custom.example.com",
			expectedEndpoint: "https://trace.agent.custom.example.com/api/v0.2/stats",
			expectAutoGen:    true,
		},
		{
			name:             "auto_generate_empty_site_uses_default",
			otlpEndpoint:     "",
			site:             "",
			expectedEndpoint: "https://trace.agent.datadoghq.com/api/v0.2/stats",
			expectAutoGen:    true,
		},
		{
			name:             "explicit_endpoint_not_overridden",
			otlpEndpoint:     "https://custom.endpoint.com/otlp",
			site:             "datadoghq.eu",
			expectedEndpoint: "https://custom.endpoint.com/otlp",
			expectAutoGen:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create an observed logger to capture log messages
			observedZapCore, observedLogs := observer.New(zap.DebugLevel)

			set := exportertest.NewNopSettings(component.MustNewType("otlphttp"))
			set.Logger = zap.New(observedZapCore)

			routingConfig := datadogconfig.MetricsRoutingConfig{
				RoutingConfig: datadogconfig.RoutingConfig{
					Enabled:      true,
					OTLPEndpoint: tt.otlpEndpoint,
					OTLPHeaders: map[string]configopaque.String{
						"Dd-Protocol": "otlp",
					},
				},
			}
			apiKey := "test-api-key"

			exporter, err := createOTLPMetricsExporter(context.Background(), set, routingConfig, apiKey, tt.site)
			require.NoError(t, err)
			assert.NotNil(t, exporter)

			// Verify logs
			logs := observedLogs.TakeAll()
			require.GreaterOrEqual(t, len(logs), 2)

			// Check for auto-generation log
			var foundAutoGenLog, foundCreatingLog bool
			var autoGenLog, creatingLog observer.LoggedEntry

			for _, log := range logs {
				switch log.Message {
				case "Auto-generated OTLP endpoint for metrics routing":
					foundAutoGenLog = true
					autoGenLog = log
				case "Creating OTLP metrics exporter for routing":
					foundCreatingLog = true
					creatingLog = log
				}
			}

			// Verify auto-generation log presence
			assert.Equal(t, tt.expectAutoGen, foundAutoGenLog, "Auto-generation log presence mismatch")
			assert.True(t, foundCreatingLog, "Should have creating log")

			if tt.expectAutoGen && foundAutoGenLog {
				assert.Equal(t, zap.InfoLevel, autoGenLog.Level)
				expectedSite := tt.site
				if expectedSite == "" {
					expectedSite = "datadoghq.com"
				}
				assert.Equal(t, expectedSite, autoGenLog.ContextMap()["site"])
				assert.Equal(t, tt.expectedEndpoint, autoGenLog.ContextMap()["generated_endpoint"])
			}

			if foundCreatingLog {
				assert.Equal(t, zap.InfoLevel, creatingLog.Level)
				assert.Equal(t, tt.expectedEndpoint, creatingLog.ContextMap()["endpoint"])
			}
		})
	}
}

func TestMetricsExporterWithSerializerRouting(t *testing.T) {
	// This test validates that routing works with the serializer exporter path
	server := testutil.DatadogServerMock()
	defer server.Close()

	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site: "datadoghq.com",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: server.URL,
			},
			DeltaTTL: 3600,
			HistConfig: datadogconfig.HistogramConfig{
				Mode:             datadogconfig.HistogramModeDistributions,
				SendAggregations: false,
			},
			SumConfig: datadogconfig.SumConfig{
				CumulativeMonotonicMode: datadogconfig.CumulativeMonotonicSumModeToDelta,
			},
			Routing: datadogconfig.MetricsRoutingConfig{
				RoutingConfig: datadogconfig.RoutingConfig{
					Enabled: true,
					// Use auto-generated endpoint (empty endpoint)
					OTLPHeaders: map[string]configopaque.String{
						"Dd-Protocol": "otlp",
					},
					Rules: []datadogconfig.RoutingRule{
						{
							Name: "test_routing",
							Condition: datadogconfig.RoutingCondition{
								InstrumentationScopeName: "test.scope",
							},
							Target: datadogconfig.TargetOTLP,
						},
					},
				},
			},
		},
		HostMetadata: datadogconfig.HostMetadataConfig{
			Enabled: false, // Disable to avoid complexity in test
		},
		HostnameDetectionTimeout: 50 * time.Millisecond,
	}

	// Create an observed logger to capture log messages
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)

	params := exportertest.NewNopSettings(metadata.Type)
	params.Logger = zap.New(observedZapCore)

	// Force the serializer path to be taken
	require.NoError(t, enableMetricExportSerializer())
	defer func() {
		// Reset to default after test
		require.NoError(t, enableNativeMetricExport())
	}()

	f := NewFactory()

	// The client should have been created correctly with routing
	exp, err := f.CreateMetrics(context.Background(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)

	// Verify that routing configuration was logged
	logs := observedLogs.TakeAll()
	var foundRoutingConfigLog, foundAutoGenLog bool

	for _, log := range logs {
		switch log.Message {
		case "Metrics routing configuration":
			foundRoutingConfigLog = true
			assert.Equal(t, zap.InfoLevel, log.Level)
		case "Auto-generated OTLP endpoint for metrics routing":
			foundAutoGenLog = true
			assert.Equal(t, zap.InfoLevel, log.Level)
			assert.Equal(t, "datadoghq.com", log.ContextMap()["site"])
			assert.Equal(t, "https://trace.agent.datadoghq.com/api/v0.2/stats", log.ContextMap()["generated_endpoint"])
		}
	}

	assert.True(t, foundRoutingConfigLog, "Should have routing configuration log")
	assert.True(t, foundAutoGenLog, "Should have auto-generated endpoint log")

	// Test consuming metrics - this verifies the exporter was created correctly
	testMetrics := pmetric.NewMetrics()
	rm := testMetrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("regular.scope") // This should go to Datadog, not OTLP
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric")

	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
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
