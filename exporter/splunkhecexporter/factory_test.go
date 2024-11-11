// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterbatcher"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := exportertest.NewNopSettings()
	_, err := createMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateTraces(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := exportertest.NewNopSettings()
	_, err := createTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := exportertest.NewNopSettings()
	_, err := createLogsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"
	params := exportertest.NewNopSettings()
	exp, err := factory.CreateMetrics(
		context.Background(), params,
		cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	// Set values that don't have a valid default.
	cfg.Token = "testToken"
	cfg.ClientConfig.Endpoint = "https://example.com"
	exp, err = factory.CreateMetrics(
		context.Background(), params,
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestFactory_CreateMetrics(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "https://example.com:8000"
	config := &Config{
		Token:        "testToken",
		ClientConfig: clientConfig,
	}

	params := exportertest.NewNopSettings()
	te, err := createMetricsExporter(context.Background(), params, config)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestFactory_EnabledBatchingMakesExporterMutable(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "https://example.com:8000"

	config := &Config{
		Token:        "testToken",
		ClientConfig: clientConfig,
	}

	me, err := createMetricsExporter(context.Background(), exportertest.NewNopSettings(), config)
	require.NoError(t, err)
	assert.False(t, me.Capabilities().MutatesData)
	te, err := createTracesExporter(context.Background(), exportertest.NewNopSettings(), config)
	require.NoError(t, err)
	assert.False(t, te.Capabilities().MutatesData)
	le, err := createLogsExporter(context.Background(), exportertest.NewNopSettings(), config)
	require.NoError(t, err)
	assert.False(t, le.Capabilities().MutatesData)

	config.BatcherConfig = exporterbatcher.NewDefaultConfig()

	me, err = createMetricsExporter(context.Background(), exportertest.NewNopSettings(), config)
	require.NoError(t, err)
	assert.True(t, me.Capabilities().MutatesData)
	te, err = createTracesExporter(context.Background(), exportertest.NewNopSettings(), config)
	require.NoError(t, err)
	assert.True(t, te.Capabilities().MutatesData)
	le, err = createLogsExporter(context.Background(), exportertest.NewNopSettings(), config)
	require.NoError(t, err)
	assert.True(t, le.Capabilities().MutatesData)
}
