// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
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

func TestOTLPConfigIsValid(t *testing.T) {
	// prepare
	factory := NewFactory()
	defaultCfg := factory.CreateDefaultConfig().(*Config)

	// test
	otlpCfg := defaultCfg.Protocol.OTLP

	// verify
	assert.NoError(t, otlpCfg.Validate())
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
	// prepare
	creationParams := exportertest.NewNopSettings(metadata.Type)
	testEndpoint := "the-endpoint"
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	creationParams.Logger = zap.New(observedZapCore)

	// test
	exporterParams := buildExporterSettings(creationParams, testEndpoint)
	exporterParams.Logger.Info("test")

	assert.Equal(t, creationParams.ID, exporterParams.ID)

	allLogs := observedLogs.All()
	require.Equal(t, 1, observedLogs.Len())
	assert.Contains(t,
		allLogs[0].Context,
		zap.String(zapEndpointKey, testEndpoint),
	)
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

func TestBuildExporterResilienceOptions(t *testing.T) {
	t.Run("Shouldn't have resilience options by default", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		assert.Empty(t, buildExporterResilienceOptions(o, cfg))
	})
	t.Run("Should have timeout option if defined", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		cfg.TimeoutSettings = exporterhelper.NewDefaultTimeoutConfig()

		assert.Len(t, buildExporterResilienceOptions(o, cfg), 1)
	})
	t.Run("Should have timeout and queue options if defined", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		cfg.TimeoutSettings = exporterhelper.NewDefaultTimeoutConfig()
		cfg.QueueSettings = exporterhelper.NewDefaultQueueConfig()

		assert.Len(t, buildExporterResilienceOptions(o, cfg), 2)
	})
	t.Run("Should have all resilience options if defined", func(t *testing.T) {
		o := []exporterhelper.Option{}
		cfg := createDefaultConfig().(*Config)
		cfg.TimeoutSettings = exporterhelper.NewDefaultTimeoutConfig()
		cfg.QueueSettings = exporterhelper.NewDefaultQueueConfig()
		cfg.BackOffConfig = configretry.NewDefaultBackOffConfig()

		assert.Len(t, buildExporterResilienceOptions(o, cfg), 3)
	})
}
