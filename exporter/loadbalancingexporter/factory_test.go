// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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
	creationParams := exportertest.NewNopSettings()
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
	}

	// test
	exp, err := factory.CreateTraces(context.Background(), creationParams, cfg)

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestLogExporterGetsCreatedWithValidConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := exportertest.NewNopSettings()
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
	}

	// test
	exp, err := factory.CreateLogs(context.Background(), creationParams, cfg)

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
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
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
	creationParams := exportertest.NewNopSettings()
	testEndpoint := "the-endpoint"
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	creationParams.Logger = zap.New(observedZapCore)

	// test
	exporterParams := buildExporterSettings(creationParams, testEndpoint)
	exporterParams.Logger.Info("test")

	// verify
	expectedID := component.NewIDWithName(
		creationParams.ID.Type(),
		fmt.Sprintf("%s_%s", creationParams.ID.Name(), testEndpoint),
	)
	assert.Equal(t, expectedID, exporterParams.ID)

	allLogs := observedLogs.All()
	require.Equal(t, 1, observedLogs.Len())
	assert.Contains(t,
		allLogs[0].Context,
		zap.String(zapEndpointKey, testEndpoint),
	)
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
