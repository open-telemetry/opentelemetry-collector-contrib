// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

func TestTracesExporterGetsCreatedWithValidConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := exportertest.NewNopSettings(metadata.Type)
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}}),
		},
	}

	// test
	exp, err := factory.CreateTraces(t.Context(), creationParams, cfg)

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestLogExporterGetsCreatedWithValidConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := exportertest.NewNopSettings(metadata.Type)
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}}),
		},
	}

	// test
	exp, err := factory.CreateLogs(t.Context(), creationParams, cfg)

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestLogExporterWithCompressedInMemoryQueueStartsWithoutStorageExtension(t *testing.T) {
	factory := NewFactory()
	creationParams := exportertest.NewNopSettings(metadata.Type)
	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.NumConsumers = 2
	queueCfg.QueueSize = 1000
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}}),
		},
		QueueSettings: QueueSettings{
			QueueConfig:        configoptional.Some(queueCfg),
			PayloadCompression: QueuePayloadCompressionZstd,
			CompressInMemory:   true,
		},
	}

	exp, err := factory.CreateLogs(t.Context(), creationParams, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.WithoutCancel(t.Context())))
	})
}

func TestOTLPConfigIsValid(t *testing.T) {
	// prepare
	factory := NewFactory()
	defaultCfg := factory.CreateDefaultConfig().(*Config)

	// test
	otlpCfg := defaultCfg.Protocol.OTLP

	// verify
	assert.NoError(t, otlpCfg.Validate())
}

func TestDefaultLogBatcherConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	assert.False(t, cfg.LogBatcher.Enabled)
	assert.Equal(t, defaultLogBatchMaxRecords, cfg.LogBatcher.MaxRecords)
	assert.Equal(t, defaultLogBatchMaxBytes, cfg.LogBatcher.MaxBytes)
	assert.Equal(t, defaultLogBatchFlushTimeout, cfg.LogBatcher.FlushInterval)
	assert.Equal(t, QueuePayloadCompressionNone, cfg.LogBatcher.PayloadCompression)
}

func TestDefaultLogRoutingConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	assert.False(t, cfg.LogRouting.IgnoreTraceID)
}

func TestDefaultMetricBatcherConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	assert.False(t, cfg.MetricBatcher.Enabled)
	assert.Equal(t, defaultMetricBatchMaxDataPoints, cfg.MetricBatcher.MaxDataPoints)
	assert.Equal(t, defaultMetricBatchMaxBytes, cfg.MetricBatcher.MaxBytes)
	assert.Equal(t, defaultMetricBatchFlushTimeout, cfg.MetricBatcher.FlushInterval)
	assert.Equal(t, defaultMetricBatchRetryBufferMultiplier, cfg.MetricBatcher.MaxRetryBufferMultiplier)
}

func TestBuildExporterConfig(t *testing.T) {
	// prepare
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factories.Exporters[metadata.Type] = NewFactory()
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "test-build-exporter-config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	c := cfg.Exporters[component.NewID(metadata.Type)]
	require.NotNil(t, c)

	// test
	defaultCfg := otlpexporter.NewFactory().CreateDefaultConfig().(*otlpexporter.Config)
	exporterCfg := buildExporterConfig(c.(*Config), "the-endpoint")

	// verify
	grpcSettings := defaultCfg.ClientConfig
	grpcSettings.Endpoint = "the-endpoint"
	assert.Equal(t, grpcSettings, exporterCfg.ClientConfig)

	assert.Equal(t, defaultCfg.TimeoutConfig, exporterCfg.TimeoutConfig)
	assert.Equal(t, defaultCfg.QueueConfig, exporterCfg.QueueConfig)
	assert.Equal(t, defaultCfg.RetryConfig, exporterCfg.RetryConfig)
}

func TestBuildExporterSettings(t *testing.T) {
	otlpType := otlpexporter.NewFactory().Type()

	t.Run("without exporter name", func(t *testing.T) {
		ctx := context.Background() //nolint:usetesting // Context must outlive test for cleanup
		creationParams := exportertest.NewNopSettings(metadata.Type)
		creationParams.ID = component.NewID(metadata.Type)
		telemetry := componenttest.NewTelemetry()
		t.Cleanup(func() {
			require.NoError(t, telemetry.Shutdown(ctx))
		})
		creationParams.TelemetrySettings = telemetry.NewTelemetrySettings()
		originalTelemetry := creationParams.TelemetrySettings
		testEndpoint := "the-endpoint"
		observedZapCore, observedLogs := observer.New(zap.InfoLevel)
		creationParams.Logger = zap.New(observedZapCore)
		originalLogger := creationParams.Logger

		exporterParams := buildExporterSettings(otlpType, creationParams, testEndpoint)
		exporterParams.Logger.Info("test")

		assert.Equal(t, component.NewID(otlpType), exporterParams.ID)
		assert.IsType(t, metricnoop.NewMeterProvider(), exporterParams.MeterProvider)
		assert.IsType(t, tracenoop.NewTracerProvider(), exporterParams.TracerProvider)

		assert.Same(t, originalTelemetry.MeterProvider, creationParams.MeterProvider)
		assert.Same(t, originalTelemetry.TracerProvider, creationParams.TracerProvider)
		assert.Same(t, originalLogger, creationParams.Logger)

		allLogs := observedLogs.All()
		require.Equal(t, 1, observedLogs.Len())
		assert.Contains(t,
			allLogs[0].Context,
			zap.String(zapEndpointKey, testEndpoint),
		)
	})

	t.Run("with exporter name", func(t *testing.T) {
		ctx := context.Background() //nolint:usetesting // Context must outlive test for cleanup
		creationParams := exportertest.NewNopSettings(metadata.Type)
		creationParams.ID = component.NewIDWithName(metadata.Type, "custom")
		telemetry := componenttest.NewTelemetry()
		t.Cleanup(func() {
			require.NoError(t, telemetry.Shutdown(ctx))
		})
		creationParams.TelemetrySettings = telemetry.NewTelemetrySettings()
		originalTelemetry := creationParams.TelemetrySettings
		testEndpoint := "the-endpoint"
		observedZapCore, observedLogs := observer.New(zap.InfoLevel)
		creationParams.Logger = zap.New(observedZapCore)
		originalLogger := creationParams.Logger

		exporterParams := buildExporterSettings(otlpType, creationParams, testEndpoint)
		exporterParams.Logger.Info("test")

		assert.Equal(t, component.NewIDWithName(otlpType, "custom"), exporterParams.ID)
		assert.IsType(t, metricnoop.NewMeterProvider(), exporterParams.MeterProvider)
		assert.IsType(t, tracenoop.NewTracerProvider(), exporterParams.TracerProvider)

		assert.Same(t, originalTelemetry.MeterProvider, creationParams.MeterProvider)
		assert.Same(t, originalTelemetry.TracerProvider, creationParams.TracerProvider)
		assert.Same(t, originalLogger, creationParams.Logger)

		allLogs := observedLogs.All()
		require.Equal(t, 1, observedLogs.Len())
		assert.Contains(t,
			allLogs[0].Context,
			zap.String(zapEndpointKey, testEndpoint),
		)
	})
}

func TestWrappedExporterHasEndpointAttribute(t *testing.T) {
	// This test verifies that the endpoint is available as an attribute for metrics
	// rather than being part of the exporter ID (which would cause high cardinality)
	testEndpoint := "10.11.68.62:4317"

	mockComponent := &struct{ component.Component }{}

	wrappedExp := newWrappedExporter(mockComponent, testEndpoint)

	endpointValue, found := wrappedExp.endpointAttr.Value("endpoint")
	require.True(t, found, "endpoint attribute should exist")
	assert.Equal(t, testEndpoint, endpointValue.AsString(), "endpoint attribute should have correct value")

	endpointValue, found = wrappedExp.successAttr.Value("endpoint")
	require.True(t, found, "success attr should have endpoint")
	assert.Equal(t, testEndpoint, endpointValue.AsString())
	successValue, found := wrappedExp.successAttr.Value("success")
	require.True(t, found, "success attr should have success field")
	assert.True(t, successValue.AsBool())

	endpointValue, found = wrappedExp.failureAttr.Value("endpoint")
	require.True(t, found, "failure attr should have endpoint")
	assert.Equal(t, testEndpoint, endpointValue.AsString())
	successValue, found = wrappedExp.failureAttr.Value("success")
	require.True(t, found, "failure attr should have success field")
	assert.False(t, successValue.AsBool())
}

func TestWrappedExporterNormalizesEndpointAttributeWithoutPort(t *testing.T) {
	mockComponent := &struct{ component.Component }{}

	wrappedExp := newWrappedExporter(mockComponent, "endpoint-1")

	endpointValue, found := wrappedExp.endpointAttr.Value("endpoint")
	require.True(t, found)
	assert.Equal(t, "endpoint-1:4317", endpointValue.AsString())
}

func TestBuildExporterResilienceOptions(t *testing.T) {
	newSettings := xexporterhelper.NewLogsQueueBatchSettings

	t.Run("Shouldn't have resilience options by default", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		assert.Empty(t, buildExporterResilienceOptions(o, cfg, nil, newSettings()))
	})
	t.Run("Should have timeout option if defined", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		cfg.TimeoutSettings = exporterhelper.NewDefaultTimeoutConfig()

		assert.Len(t, buildExporterResilienceOptions(o, cfg, nil, newSettings()), 1)
	})
	t.Run("Should have timeout and queue options if defined", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		cfg.TimeoutSettings = exporterhelper.NewDefaultTimeoutConfig()
		cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())

		assert.Len(t, buildExporterResilienceOptions(o, cfg, nil, newSettings()), 2)
	})
	t.Run("Should have timeout, queue and payload codec options when compression is enabled", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		cfg.TimeoutSettings = exporterhelper.NewDefaultTimeoutConfig()
		cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
		cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionSnappy

		assert.Len(t, buildExporterResilienceOptions(o, cfg, newQueuePayloadCodec(cfg.QueueSettings.PayloadCompression), newSettings()), 2)
	})
	t.Run("Should keep queue options count when compression in memory is enabled", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		cfg.TimeoutSettings = exporterhelper.NewDefaultTimeoutConfig()
		cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
		cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionZstd
		cfg.QueueSettings.CompressInMemory = true

		assert.Len(t, buildExporterResilienceOptions(o, cfg, newQueuePayloadCodec(cfg.QueueSettings.PayloadCompression), newSettings()), 2)
	})
	t.Run("Should have all resilience options if defined", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		cfg.TimeoutSettings = exporterhelper.NewDefaultTimeoutConfig()
		cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
		cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionNone
		cfg.BackOffConfig = configretry.NewDefaultBackOffConfig()

		assert.Len(t, buildExporterResilienceOptions(o, cfg, newQueuePayloadCodecIfEnabled(cfg), newSettings()), 3)
	})
}

func TestNewQueuePayloadCodecIfEnabled(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())

	t.Run("legacy empty string disables codec", func(t *testing.T) {
		cfg.QueueSettings.PayloadCompression = ""
		assert.Nil(t, newQueuePayloadCodecIfEnabled(cfg))
	})

	t.Run("explicit none enables compatibility codec", func(t *testing.T) {
		cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionNone
		assert.NotNil(t, newQueuePayloadCodecIfEnabled(cfg))
	})
}

func TestQueueConfigForExportKeepsMemoryQueueWhenRequested(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
	cfg.QueueSettings.CompressInMemory = true

	queueCfg, ok := queueConfigForExport(cfg)
	require.True(t, ok)
	require.True(t, queueCfg.HasValue())
	require.Nil(t, queueCfg.Get().StorageID)
}
