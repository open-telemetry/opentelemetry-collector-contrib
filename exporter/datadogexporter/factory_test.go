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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
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

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		ClientConfig:  defaultClientConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),

		API: APIConfig{
			Site: "datadoghq.com",
		},

		Metrics: MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://api.datadoghq.com",
			},
			DeltaTTL: 3600,
			HistConfig: HistogramConfig{
				Mode:             "distributions",
				SendAggregations: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
				InitialCumulativeMonotonicMode: InitialValueModeAuto,
			},
			SummaryConfig: SummaryConfig{
				Mode: SummaryModeGauges,
			},
		},

		Traces: TracesConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			IgnoreResources: []string{},
		},
		Logs: LogsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://http-intake.logs.datadoghq.com",
			},
			UseCompression:   true,
			CompressionLevel: 6,
			BatchWait:        5,
		},

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceConfigOrSystem,
		},
		OnlyMetadata: false,
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

// Test that the factory creates the default configuration
func TestCreateDefaultConfigLogsAgent(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("exporter.datadogexporter.UseLogsAgentExporter", true)
	assert.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		ClientConfig:  defaultClientConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),

		API: APIConfig{
			Site: "datadoghq.com",
		},

		Metrics: MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://api.datadoghq.com",
			},
			DeltaTTL: 3600,
			HistConfig: HistogramConfig{
				Mode:             "distributions",
				SendAggregations: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
				InitialCumulativeMonotonicMode: InitialValueModeAuto,
			},
			SummaryConfig: SummaryConfig{
				Mode: SummaryModeGauges,
			},
		},

		Traces: TracesConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			IgnoreResources: []string{},
		},
		Logs: LogsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://http-intake.logs.datadoghq.com",
			},
			UseCompression:   true,
			CompressionLevel: 6,
			BatchWait:        5,
		},

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceConfigOrSystem,
		},
		OnlyMetadata: false,
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	err = featuregate.GlobalRegistry().Set("exporter.datadogexporter.UseLogsAgentExporter", false)
	assert.NoError(t, err)
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "default"),
			expected: &Config{
				ClientConfig:  defaultClientConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				API: APIConfig{
					Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Site:             "datadoghq.com",
					FailOnInvalidKey: false,
				},

				Metrics: MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.com",
					},
					DeltaTTL: 3600,
					HistConfig: HistogramConfig{
						Mode:             "distributions",
						SendAggregations: false,
					},
					SumConfig: SumConfig{
						CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
						InitialCumulativeMonotonicMode: InitialValueModeAuto,
					},
					SummaryConfig: SummaryConfig{
						Mode: SummaryModeGauges,
					},
				},

				Traces: TracesConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.com",
					},
					IgnoreResources: []string{},
				},
				Logs: LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.datadoghq.com",
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        5,
				},
				HostMetadata: HostMetadataConfig{
					Enabled:        true,
					HostnameSource: HostnameSourceConfigOrSystem,
				},
				OnlyMetadata: false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "api"),
			expected: &Config{
				ClientConfig:  defaultClientConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				TagsConfig: TagsConfig{
					Hostname: "customhostname",
				},
				API: APIConfig{
					Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Site:             "datadoghq.eu",
					FailOnInvalidKey: true,
				},
				Metrics: MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.eu",
					},
					DeltaTTL: 3600,
					HistConfig: HistogramConfig{
						Mode:             "distributions",
						SendAggregations: false,
					},
					SumConfig: SumConfig{
						CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
						InitialCumulativeMonotonicMode: InitialValueModeAuto,
					},
					SummaryConfig: SummaryConfig{
						Mode: SummaryModeGauges,
					},
				},
				Traces: TracesConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.eu",
					},
					SpanNameRemappings: map[string]string{
						"old_name1": "new_name1",
						"old_name2": "new_name2",
					},
					SpanNameAsResourceName: true,
					IgnoreResources:        []string{},
					TraceBuffer:            10,
				},
				Logs: LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.datadoghq.eu",
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        5,
				},
				OnlyMetadata: false,
				HostMetadata: HostMetadataConfig{
					Enabled:        true,
					HostnameSource: HostnameSourceConfigOrSystem,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "api2"),
			expected: &Config{
				ClientConfig:  defaultClientConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				TagsConfig: TagsConfig{
					Hostname: "customhostname",
				},
				API: APIConfig{
					Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Site:             "datadoghq.eu",
					FailOnInvalidKey: false,
				},
				Metrics: MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.test",
					},
					DeltaTTL: 3600,
					HistConfig: HistogramConfig{
						Mode:             "distributions",
						SendAggregations: false,
					},
					SumConfig: SumConfig{
						CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
						InitialCumulativeMonotonicMode: InitialValueModeAuto,
					},
					SummaryConfig: SummaryConfig{
						Mode: SummaryModeGauges,
					},
				},
				Traces: TracesConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.test",
					},
					SpanNameRemappings: map[string]string{
						"old_name3": "new_name3",
						"old_name4": "new_name4",
					},
					IgnoreResources: []string{},
				},
				Logs: LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.datadoghq.test",
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        5,
				},
				HostMetadata: HostMetadataConfig{
					Enabled:        true,
					HostnameSource: HostnameSourceConfigOrSystem,
					Tags:           []string{"example:tag"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestOverrideEndpoints(t *testing.T) {
	tests := []struct {
		componentID             string
		expectedSite            string
		expectedMetricsEndpoint string
		expectedTracesEndpoint  string
		expectedLogsEndpoint    string
	}{
		{
			componentID:             "nositeandnoendpoints",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "https://api.datadoghq.com",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.com",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.com",
		},
		{
			componentID:             "nositeandmetricsendpoint",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.com",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.com",
		},
		{
			componentID:             "nositeandtracesendpoint",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "https://api.datadoghq.com",
			expectedTracesEndpoint:  "tracesendpoint:1234",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.com",
		},
		{
			componentID:             "nositeandlogsendpoint",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "https://api.datadoghq.com",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.com",
			expectedLogsEndpoint:    "logsendpoint:1234",
		},
		{
			componentID:             "nositeandallendpoints",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "tracesendpoint:1234",
			expectedLogsEndpoint:    "logsendpoint:1234",
		},

		{
			componentID:             "siteandnoendpoints",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "https://api.datadoghq.eu",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.eu",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.eu",
		},
		{
			componentID:             "siteandmetricsendpoint",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.eu",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.eu",
		},
		{
			componentID:             "siteandtracesendpoint",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "https://api.datadoghq.eu",
			expectedTracesEndpoint:  "tracesendpoint:1234",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.eu",
		},
		{
			componentID:             "siteandallendpoints",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "tracesendpoint:1234",
			expectedLogsEndpoint:    "logsendpoint:1234",
		},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "unmarshal.yaml"))
	require.NoError(t, err)
	factory := NewFactory()

	for _, testInstance := range tests {
		t.Run(testInstance.componentID, func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, testInstance.componentID).String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			componentCfg, ok := cfg.(*Config)
			require.True(t, ok, "component.Config is not a Datadog exporter config (wrong ID?)")
			assert.Equal(t, testInstance.expectedSite, componentCfg.API.Site)
			assert.Equal(t, testInstance.expectedMetricsEndpoint, componentCfg.Metrics.Endpoint)
			assert.Equal(t, testInstance.expectedTracesEndpoint, componentCfg.Traces.Endpoint)
			assert.Equal(t, testInstance.expectedLogsEndpoint, componentCfg.Logs.Endpoint)
		})
	}
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

	c := cfg.(*Config)
	c.Metrics.TCPAddrConfig.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		exportertest.NewNopSettings(),
		cfg,
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateAPIExporterFailOnInvalidKey_Zorkian(t *testing.T) {
	server := testutil.DatadogServerMock(testutil.ValidateAPIKeyEndpointInvalid)
	defer server.Close()

	if isMetricExportV2Enabled() {
		require.NoError(t, enableZorkianMetricExport())
		defer require.NoError(t, enableNativeMetricExport())
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	// Use the mock server for API key validation
	c := cfg.(*Config)
	c.Metrics.TCPAddrConfig.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	t.Run("true", func(t *testing.T) {
		c.API.FailOnInvalidKey = true
		ctx := context.Background()
		// metrics exporter
		mexp, err := factory.CreateMetricsExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, mexp)

		texp, err := factory.CreateTracesExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, texp)

		lexp, err := factory.CreateLogsExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, lexp)
	})
	t.Run("false", func(t *testing.T) {
		c.API.FailOnInvalidKey = false
		ctx := context.Background()
		exp, err := factory.CreateMetricsExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, exp)

		texp, err := factory.CreateTracesExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, texp)

		lexp, err := factory.CreateLogsExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, lexp)
	})
}

func TestCreateAPIExporterFailOnInvalidKey(t *testing.T) {
	server := testutil.DatadogServerMock(testutil.ValidateAPIKeyEndpointInvalid)
	defer server.Close()

	if !isMetricExportV2Enabled() {
		require.NoError(t, enableNativeMetricExport())
		defer require.NoError(t, enableZorkianMetricExport())
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	// Use the mock server for API key validation
	c := cfg.(*Config)
	c.Metrics.TCPAddrConfig.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	t.Run("true", func(t *testing.T) {
		c.API.FailOnInvalidKey = true
		ctx := context.Background()
		// metrics exporter
		mexp, err := factory.CreateMetricsExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, mexp)

		texp, err := factory.CreateTracesExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, texp)

		lexp, err := factory.CreateLogsExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, lexp)
	})
	t.Run("false", func(t *testing.T) {
		c.API.FailOnInvalidKey = false
		ctx := context.Background()
		exp, err := factory.CreateMetricsExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, exp)

		texp, err := factory.CreateTracesExporter(
			ctx,
			exportertest.NewNopSettings(),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, texp)

		lexp, err := factory.CreateLogsExporter(
			ctx,
			exportertest.NewNopSettings(),
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

	c := cfg.(*Config)
	c.Metrics.TCPAddrConfig.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateLogsExporter(
		ctx,
		exportertest.NewNopSettings(),
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
	cfg := &Config{
		ClientConfig:  defaultClientConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),

		API:          APIConfig{Key: "notnull"},
		Metrics:      MetricsConfig{TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL}},
		Traces:       TracesConfig{TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL}},
		OnlyMetadata: true,

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceFirstResource,
			sourceTimeout:  50 * time.Millisecond,
		},
	}

	expTraces, err := factory.CreateTracesExporter(
		ctx,
		exportertest.NewNopSettings(),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expTraces)

	expMetrics, err := factory.CreateMetricsExporter(
		ctx,
		exportertest.NewNopSettings(),
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
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")
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

	c := cfg.(*Config)
	c.Metrics.TCPAddrConfig.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	expTraces, err := factory.CreateTracesExporter(
		ctx,
		exportertest.NewNopSettings(),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expTraces)
	expMetrics, err := factory.CreateMetricsExporter(
		ctx,
		exportertest.NewNopSettings(),
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
