// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubsexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	// Verify default values
	defaultCfg := cfg.(*Config)
	assert.Equal(t, ConnectionString, defaultCfg.Auth.Type)
	assert.Equal(t, "metrics", defaultCfg.EventHub.Metrics)
	assert.Equal(t, "logs", defaultCfg.EventHub.Logs)
	assert.Equal(t, "traces", defaultCfg.EventHub.Traces)
	assert.Equal(t, "json", defaultCfg.FormatType)
	assert.Equal(t, "random", defaultCfg.PartitionKey.Source)
	assert.Equal(t, 1024*1024, defaultCfg.MaxEventSize)
	assert.Equal(t, 100, defaultCfg.BatchSize)
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.ConnectionString = "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=Test;SharedAccessKey=test"

	exp, err := createTracesExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
}

func TestCreateLogsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.ConnectionString = "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=Test;SharedAccessKey=test"

	exp, err := createLogsExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.ConnectionString = "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=Test;SharedAccessKey=test"

	exp, err := createMetricsExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
}

func TestCreateTracesExporterWithInvalidConfig(t *testing.T) {
	cfg := &Config{
		Auth: Authentication{
			Type: ServicePrincipal,
			// Missing required fields
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
	}

	_, err := createTracesExporter(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	assert.Error(t, err)
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, metadata.Type, factory.Type())

	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}
