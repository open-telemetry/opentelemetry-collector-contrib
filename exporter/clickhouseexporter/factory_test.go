// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

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
		cfg.Endpoint = defaultEndpoint
	})
	params := exportertest.NewNopCreateSettings()
	exporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.TODO()))
}

func TestFactory_CreateTracesExporter(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoint = defaultEndpoint
	})
	params := exportertest.NewNopCreateSettings()
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.TODO()))
}

func TestFactory_CreateMetricsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoint = defaultEndpoint
	})
	params := exportertest.NewNopCreateSettings()
	exporter, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.TODO()))
}
