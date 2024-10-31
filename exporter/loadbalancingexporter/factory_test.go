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
	// nolint:staticcheck
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
	if c.(*Config).QueueSettings.Enabled {
		defaultCfg.QueueConfig.Enabled = false
	}
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
