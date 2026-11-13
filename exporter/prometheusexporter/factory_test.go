// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

// Regression test for GitHub issue #47173.
func TestCreateDefaultConfigServerDefaults(t *testing.T) {
	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	assert.Equal(t, confighttp.NewDefaultServerConfig(), cfg.ServerConfig)
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.ServerConfig.NetAddr.Endpoint = ""
	exp, err := createMetricsExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	require.Equal(t, errBlankPrometheusAddress, err)
	require.Nil(t, exp)
}

func TestCreateMetricsExportHelperError(t *testing.T) {
	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	cfg.ServerConfig.NetAddr.Endpoint = "http://localhost:8889"

	set := exportertest.NewNopSettings(metadata.Type)
	set.Logger = nil

	// Should give us an exporterhelper.errNilLogger
	exp, err := createMetricsExporter(t.Context(), set, cfg)

	assert.Nil(t, exp)
	assert.Error(t, err)
}

func TestExporterHelperBatchingEnabled(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		cfg := createDefaultConfig().(*Config)
		cfg.ServerConfig.NetAddr.Endpoint = "localhost:0"
		queueCfg := cfg.QueueBatchConfig.GetOrInsertDefault()
		if enabled {
			queueCfg.Batch.GetOrInsertDefault()
		} else {
			queueCfg.Batch = configoptional.None[exporterhelper.BatchConfig]()
		}

		exp, err := NewFactory().CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
		require.NoError(t, err)
		b, ok := exp.(interface{ ExporterHelperBatchingEnabled() bool })
		require.True(t, ok)
		assert.Equal(t, enabled, b.ExporterHelperBatchingEnabled())
	}
}

func TestCreateMetrics_SendingQueueWarning(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServerConfig.NetAddr.Endpoint = "localhost:0"
	queueCfg := exporterhelper.NewDefaultQueueConfig()
	cfg.QueueBatchConfig = configoptional.Some(queueCfg)

	observedZapCore, observedLogs := observer.New(zap.WarnLevel)
	set := exportertest.NewNopSettings(metadata.Type)
	set.Logger = zap.New(observedZapCore)

	exp, err := createMetricsExporter(t.Context(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	warnLogs := observedLogs.FilterMessageSnippet("sending_queue is deprecated and will be removed in a future release")
	assert.Equal(t, 1, warnLogs.Len())
}

func TestCreateMetrics_DefaultSendingQueueNoWarning(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServerConfig.NetAddr.Endpoint = "localhost:0"

	observedZapCore, observedLogs := observer.New(zap.WarnLevel)
	set := exportertest.NewNopSettings(metadata.Type)
	set.Logger = zap.New(observedZapCore)

	exp, err := createMetricsExporter(t.Context(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	warnLogs := observedLogs.FilterMessageSnippet("sending_queue is deprecated and will be removed in a future release")
	assert.Equal(t, 0, warnLogs.Len())
}
