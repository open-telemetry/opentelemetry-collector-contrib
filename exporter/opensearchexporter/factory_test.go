// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_CreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.ClientConfig.Endpoint = "https://opensearch.example.com:9200"
	})
	params := exportertest.NewNopSettings(metadata.Type)
	exporter, err := factory.CreateTraces(t.Context(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(t.Context()))
}

func TestFactory_CreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.ClientConfig.Endpoint = "https://opensearch.example.com:9200"
	})
	params := exportertest.NewNopSettings(metadata.Type)
	exporter, err := factory.CreateLogs(t.Context(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(t.Context()))
}

func TestFactory_CreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.ClientConfig.Endpoint = "https://opensearch.example.com:9200"
	})
	params := exportertest.NewNopSettings(metadata.Type)
	exporter, err := factory.CreateMetrics(t.Context(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(t.Context()))
}

func TestFactory_CreateMetrics_UnsupportedMode(t *testing.T) {
	factory := NewFactory()
	params := exportertest.NewNopSettings(metadata.Type)

	for _, mode := range []string{"ecs", "flatten_attributes", "bodymap"} {
		cfg := withDefaultConfig(func(cfg *Config) {
			cfg.ClientConfig.Endpoint = "https://opensearch.example.com:9200"
			cfg.MappingsSettings.Mode = mode
		})
		exporter, err := factory.CreateMetrics(t.Context(), params, cfg)
		require.ErrorIs(t, err, errMetricsMappingModeUnsupported, "mode %q", mode)
		require.Nil(t, exporter)
	}
}

func TestCreateLogsExporter_WithDynamicIndex(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.LogsIndex = "otel-logs-%{service.name}"
		cfg.LogsIndexFallback = "fallback"
		cfg.LogsIndexTimeFormat = "yyyy.MM.dd"
	})
	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateLogs(t.Context(), set, cfg)
	if err != nil {
		t.Fatalf("failed to create logs exporter: %v", err)
	}
	if exp == nil {
		t.Fatal("expected exporter, got nil")
	}
}
