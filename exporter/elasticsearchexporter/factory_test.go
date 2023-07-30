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
		cfg.Endpoints = []string{"test:9200"}
	})
	params := exportertest.NewNopCreateSettings()
	exporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.TODO()))
}

func TestFactory_CreateMetricsExporter_Fail(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	params := exportertest.NewNopCreateSettings()
	_, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	require.Error(t, err, "expected an error when creating a traces exporter")
}

func TestFactory_CreateTracesExporter_Fail(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	params := exportertest.NewNopCreateSettings()
	_, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	require.Error(t, err, "expected an error when creating a traces exporter")
}

func TestFactory_CreateLogsAndTracesExporterWithDeprecatedIndexOption(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"test:9200"}
		cfg.Index = "test_index"
	})
	params := exportertest.NewNopCreateSettings()
	logsExporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, logsExporter)
	require.NoError(t, logsExporter.Shutdown(context.TODO()))

	tracesExporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, tracesExporter)
	require.NoError(t, tracesExporter.Shutdown(context.TODO()))
}
