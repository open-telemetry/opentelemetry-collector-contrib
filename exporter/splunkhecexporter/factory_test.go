// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := exportertest.NewNopSettings(metadata.Type)
	_, err := createMetricsExporter(t.Context(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateTraces(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := exportertest.NewNopSettings(metadata.Type)
	_, err := createTracesExporter(t.Context(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := exportertest.NewNopSettings(metadata.Type)
	_, err := createLogsExporter(t.Context(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"
	params := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateMetrics(
		t.Context(), params,
		cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	// Set values that don't have a valid default.
	cfg.Token = "testToken"
	cfg.Endpoint = "https://example.com"
	exp, err = factory.CreateMetrics(
		t.Context(), params,
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(t.Context()))
}

func TestFactory_CreateMetrics(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "https://example.com:8000"
	config := &Config{
		Token:        "testToken",
		ClientConfig: clientConfig,
	}

	params := exportertest.NewNopSettings(metadata.Type)
	te, err := createMetricsExporter(t.Context(), params, config)
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

	me, err := createMetricsExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), config)
	require.NoError(t, err)
	assert.False(t, me.Capabilities().MutatesData)
	te, err := createTracesExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), config)
	require.NoError(t, err)
	assert.False(t, te.Capabilities().MutatesData)
	le, err := createLogsExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), config)
	require.NoError(t, err)
	assert.False(t, le.Capabilities().MutatesData)

	config.QueueSettings = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
	config.QueueSettings.Get().Sizer = exporterhelper.RequestSizerTypeItems
	config.QueueSettings.Get().Batch = configoptional.Some(exporterhelper.BatchConfig{
		FlushTimeout: 200 * time.Millisecond,
		MinSize:      8192,
	})

	me, err = createMetricsExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), config)
	require.NoError(t, err)
	assert.True(t, me.Capabilities().MutatesData)
	te, err = createTracesExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), config)
	require.NoError(t, err)
	assert.True(t, te.Capabilities().MutatesData)
	le, err = createLogsExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), config)
	require.NoError(t, err)
	assert.True(t, le.Capabilities().MutatesData)
}
