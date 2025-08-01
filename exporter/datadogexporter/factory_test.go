// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var _ inframetadata.Pusher = (*testPusher)(nil)

type testPusher struct {
	mu       sync.Mutex
	payloads []payload.HostMetadata
	stopped  bool
	logger   *zap.Logger
	t        *testing.T
}

func newTestPusher(t *testing.T) *testPusher {
	return &testPusher{
		logger: zaptest.NewLogger(t),
		t:      t,
	}
}

func (p *testPusher) Push(_ context.Context, hm payload.HostMetadata) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		p.logger.Error("Trying to push payload after stopping", zap.Any("payload", hm))
		p.t.Fail()
	}
	p.logger.Info("Storing host metadata payload", zap.Any("payload", hm))
	p.payloads = append(p.payloads, hm)
	return nil
}

func (p *testPusher) Payloads() []payload.HostMetadata {
	p.mu.Lock()
	p.stopped = true
	defer p.mu.Unlock()
	return p.payloads
}

func TestCreateAPIMetricsExporter(t *testing.T) {
	server := testutil.DatadogServerMock()
	defer server.Close()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateMetrics(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateAPIExporterFailOnInvalidKey_Zorkian(t *testing.T) {
	server := testutil.DatadogServerMock(testutil.ValidateAPIKeyEndpointInvalid)
	defer server.Close()

	require.NoError(t, enableZorkianMetricExport())
	defer require.NoError(t, enableMetricExportSerializer())

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	// Use the mock server for API key validation
	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	t.Run("true", func(t *testing.T) {
		c.API.FailOnInvalidKey = true
		ctx := context.Background()
		// metrics exporter
		mexp, err := factory.CreateMetrics(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, mexp)

		texp, err := factory.CreateTraces(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, texp)

		lexp, err := factory.CreateLogs(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		// logs agent exporter does not fail on  invalid api key
		assert.NoError(t, err)
		assert.NotNil(t, lexp)
	})
	t.Run("false", func(t *testing.T) {
		c.API.FailOnInvalidKey = false
		ctx := context.Background()
		exp, err := factory.CreateMetrics(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, exp)

		texp, err := factory.CreateTraces(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, texp)

		lexp, err := factory.CreateLogs(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, lexp)
	})
}

func TestCreateAPIExporterFailOnInvalidKey_Serializer(t *testing.T) {
	server := testutil.DatadogServerMock(testutil.ValidateAPIKeyEndpointInvalid)
	defer server.Close()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	// Use the mock server for API key validation
	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	t.Run("true", func(t *testing.T) {
		c.API.FailOnInvalidKey = true
		ctx := context.Background()
		// metrics exporter
		mexp, err := factory.CreateMetrics(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, mexp)

		texp, err := factory.CreateTraces(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, texp)

		lexp, err := factory.CreateLogs(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		// logs agent exporter does not fail on  invalid api key
		assert.NoError(t, err)
		assert.NotNil(t, lexp)
	})
	t.Run("false", func(t *testing.T) {
		c.API.FailOnInvalidKey = false
		ctx := context.Background()
		exp, err := factory.CreateMetrics(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, exp)

		texp, err := factory.CreateTraces(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, texp)

		lexp, err := factory.CreateLogs(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, lexp)
	})
}

func TestCreateAPILogsExporter(t *testing.T) {
	server := testutil.DatadogLogServerMock()
	defer server.Close()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateLogs(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestOnlyMetadata(t *testing.T) {
	server := testutil.DatadogServerMock()
	defer server.Close()

	factory := NewFactory()
	ctx := context.Background()
	cfg := &datadogconfig.Config{
		ClientConfig:  defaultClientConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),

		API:          datadogconfig.APIConfig{Key: "aaaaaaa"},
		Metrics:      datadogconfig.MetricsConfig{TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL}},
		Traces:       datadogconfig.TracesExporterConfig{TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL}},
		OnlyMetadata: true,

		HostMetadata: datadogconfig.HostMetadataConfig{
			Enabled:        true,
			ReporterPeriod: 30 * time.Minute,
		},
		HostnameDetectionTimeout: 50 * time.Millisecond,
	}

	expTraces, err := factory.CreateTraces(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expTraces)

	expMetrics, err := factory.CreateMetrics(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expMetrics)

	err = expTraces.Start(ctx, nil)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, expTraces.Shutdown(ctx))
	}()

	testTraces := ptrace.NewTraces()
	testutil.TestTraces.CopyTo(testTraces)
	err = expTraces.ConsumeTraces(ctx, testTraces)
	require.NoError(t, err)

	recvMetadata := <-server.MetadataChan
	assert.NotEmpty(t, recvMetadata.InternalHostname)
}

func TestStopExporters(t *testing.T) {
	server := testutil.DatadogServerMock()
	defer server.Close()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	expTraces, err := factory.CreateTraces(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expTraces)
	expMetrics, err := factory.CreateMetrics(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expMetrics)

	err = expTraces.Start(ctx, nil)
	assert.NoError(t, err)
	err = expMetrics.Start(ctx, nil)
	assert.NoError(t, err)

	finishShutdown := make(chan bool)
	go func() {
		err = expMetrics.Shutdown(ctx)
		assert.NoError(t, err)
		err = expTraces.Shutdown(ctx)
		assert.NoError(t, err)
		finishShutdown <- true
	}()

	select {
	case <-finishShutdown:
		break
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out")
	}
}

// TestAdaptableMetricsRouting tests the new adaptable metrics routing with both serializer and default exporters
func TestAdaptableMetricsRouting(t *testing.T) {
	tests := []struct {
		name             string
		enableSerializer bool
	}{
		{
			name:             "with_serializer_exporter",
			enableSerializer: true,
		},
		{
			name:             "with_default_exporter",
			enableSerializer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock server
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
							Enabled:      false, // Disable routing for simpler test - just test the structure
							OTLPEndpoint: "https://test-otlp.datadoghq.com",
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

			// Set the appropriate exporter mode
			if tt.enableSerializer {
				require.NoError(t, enableMetricExportSerializer())
				defer func() {
					require.NoError(t, enableNativeMetricExport())
				}()
			} else {
				require.NoError(t, enableNativeMetricExport())
			}

			f := NewFactory()

			// Create the exporter
			exp, err := f.CreateMetrics(context.Background(), params, cfg)
			require.NoError(t, err)
			assert.NotNil(t, exp)

			// Verify that routing configuration was logged
			logs := observedLogs.TakeAll()
			var foundRoutingConfigLog bool

			for _, log := range logs {
				if log.Message == "Metrics routing configuration" {
					foundRoutingConfigLog = true
					assert.Equal(t, zap.InfoLevel, log.Level)
				}
			}

			assert.True(t, foundRoutingConfigLog, "Should have routing configuration log")

			// Test consuming metrics - verify the exporter works with different scope names
			testCases := []struct {
				scopeName     string
				metricName    string
				expectedRoute string
			}{
				{
					scopeName:     "test.scope", // Should go to OTLP
					metricName:    "test.metric.otlp",
					expectedRoute: "OTLP",
				},
				{
					scopeName:     "regular.scope", // Should go to Datadog
					metricName:    "test.metric.datadog",
					expectedRoute: "Datadog",
				},
			}

			for _, tc := range testCases {
				t.Run("routing_"+tc.scopeName, func(t *testing.T) {
					testMetrics := pmetric.NewMetrics()
					rm := testMetrics.ResourceMetrics().AppendEmpty()
					sm := rm.ScopeMetrics().AppendEmpty()
					sm.Scope().SetName(tc.scopeName)
					metric := sm.Metrics().AppendEmpty()
					metric.SetName(tc.metricName)

					// This should not error regardless of routing
					err = exp.ConsumeMetrics(context.Background(), testMetrics)
					require.NoError(t, err)
				})
			}
		})
	}
}

// TestAdaptableMetricsExporterHelpers tests the helper functions for adaptable metrics export
func TestAdaptableMetricsExporterHelpers(t *testing.T) {
	ctx := context.Background()

	// Mock consumer function
	var callCount int
	//nolint:unparam
	mockConsumer := func(_ context.Context, _ pmetric.Metrics) error {
		callCount++
		return nil
	}

	// Create router config
	routerConfig := metricsRouterConfig{
		enabled: true,
		routingConfig: datadogconfig.MetricsRoutingConfig{
			RoutingConfig: datadogconfig.RoutingConfig{
				Enabled:      true,
				OTLPEndpoint: "https://test.datadoghq.com",
			},
		},
		apiKey:           "test-key",
		site:             "datadoghq.com",
		logger:           zap.NewNop(),
		exporterSettings: exportertest.NewNopSettings(metadata.Type),
	}

	t.Run("disabled_routing", func(t *testing.T) {
		disabledConfig := routerConfig
		disabledConfig.enabled = false

		adaptableExporter, err := newAdaptableMetricsExporter(ctx, mockConsumer, disabledConfig)
		require.NoError(t, err)
		assert.NotNil(t, adaptableExporter)

		// Should return the base exporter function when routing is disabled
		consumeFunc := adaptableExporter.GetConsumeMetricsFunc()
		assert.NotNil(t, consumeFunc)

		// Should not have start function when routing is disabled
		startFunc := adaptableExporter.GetStartFunc()
		assert.Nil(t, startFunc)

		// Test that it calls the base consumer
		testMetrics := pmetric.NewMetrics()
		err = consumeFunc(ctx, testMetrics)
		require.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("enabled_routing", func(t *testing.T) {
		// Reset call count
		callCount = 0

		adaptableExporter, err := newAdaptableMetricsExporter(ctx, mockConsumer, routerConfig)
		require.NoError(t, err)
		assert.NotNil(t, adaptableExporter)

		// Should return router's consume function when routing is enabled
		consumeFunc := adaptableExporter.GetConsumeMetricsFunc()
		assert.NotNil(t, consumeFunc)

		// Should have start function when routing is enabled
		startFunc := adaptableExporter.GetStartFunc()
		assert.NotNil(t, startFunc)

		// Test shutdown function
		shutdownFunc := adaptableExporter.GetShutdownFunc(func(context.Context) error {
			return nil
		})
		assert.NotNil(t, shutdownFunc)

		// Test that shutdown function works
		err = shutdownFunc(ctx)
		require.NoError(t, err)
	})
}
