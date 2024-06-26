// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_CreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"http://test:9200"}
	})
	params := exportertest.NewNopSettings()
	exporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.Background()))
}

func TestFactory_CreateLogsExporter_Fail(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	params := exportertest.NewNopSettings()
	_, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	require.Error(t, err, "expected an error when creating a logs exporter")
	assert.EqualError(t, err, "cannot configure Elasticsearch exporter: exactly one of [endpoint, endpoints, cloudid] must be specified")
}

func TestFactory_CreateMetricsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"http://test:9200"}
	})
	params := exportertest.NewNopSettings()
	exporter, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.Background()))
}

func TestFactory_CreateTracesExporter(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"http://test:9200"}
	})
	params := exportertest.NewNopSettings()
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.Background()))
}

func TestFactory_CreateTracesExporter_Fail(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	params := exportertest.NewNopSettings()
	_, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	require.Error(t, err, "expected an error when creating a traces exporter")
	assert.EqualError(t, err, "cannot configure Elasticsearch exporter: exactly one of [endpoint, endpoints, cloudid] must be specified")
}

func TestFactory_CreateLogsAndTracesExporterWithDeprecatedIndexOption(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"http://test:9200"}
		cfg.Index = "test_index"
	})
	params := exportertest.NewNopSettings()
	logsExporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, logsExporter)
	require.NoError(t, logsExporter.Shutdown(context.Background()))

	tracesExporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, tracesExporter)
	require.NoError(t, tracesExporter.Shutdown(context.Background()))
}
