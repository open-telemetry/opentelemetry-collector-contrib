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
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
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
	for _, tc := range []struct {
		name         string
		gateEnabled  bool
		batchEnabled bool
		wantBatching bool
	}{
		{name: "gate_enabled_by_default_with_batch", gateEnabled: true, batchEnabled: true, wantBatching: false},
		{name: "gate_enabled_by_default_without_batch", gateEnabled: true, batchEnabled: false, wantBatching: false},
		{name: "gate_disabled_with_batch", gateEnabled: false, batchEnabled: true, wantBatching: true},
		{name: "gate_disabled_without_batch", gateEnabled: false, batchEnabled: false, wantBatching: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.gateEnabled {
				defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusDisableSendingQueueFeatureGate, false)()
			}

			cfg := createDefaultConfig().(*Config)
			cfg.ServerConfig.NetAddr.Endpoint = "localhost:0"
			queueCfg := cfg.QueueBatchConfig.GetOrInsertDefault()
			if tc.batchEnabled {
				queueCfg.Batch.GetOrInsertDefault()
			} else {
				queueCfg.Batch = configoptional.None[exporterhelper.BatchConfig]()
			}

			exp, err := NewFactory().CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
			require.NoError(t, err)
			b, ok := exp.(interface{ ExporterHelperBatchingEnabled() bool })
			require.True(t, ok)
			assert.Equal(t, tc.wantBatching, b.ExporterHelperBatchingEnabled())
		})
	}
}

func TestCreateMetrics_SendingQueueWarning(t *testing.T) {
	for _, tc := range []struct {
		name        string
		gateEnabled bool
		setQueue    bool
		wantWarns   int
	}{
		{name: "gate_enabled_by_default_with_queue", gateEnabled: true, setQueue: true, wantWarns: 0},
		{name: "gate_enabled_by_default_no_queue", gateEnabled: true, setQueue: false, wantWarns: 0},
		{name: "gate_disabled_with_queue", gateEnabled: false, setQueue: true, wantWarns: 1},
		{name: "gate_disabled_no_queue", gateEnabled: false, setQueue: false, wantWarns: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.gateEnabled {
				defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusDisableSendingQueueFeatureGate, false)()
			}

			cfg := createDefaultConfig().(*Config)
			cfg.ServerConfig.NetAddr.Endpoint = "localhost:0"
			if tc.setQueue {
				queueCfg := exporterhelper.NewDefaultQueueConfig()
				cfg.QueueBatchConfig = configoptional.Some(queueCfg)
			}

			observedZapCore, observedLogs := observer.New(zap.WarnLevel)
			set := exportertest.NewNopSettings(metadata.Type)
			set.Logger = zap.New(observedZapCore)

			exp, err := createMetricsExporter(t.Context(), set, cfg)
			require.NoError(t, err)
			require.NotNil(t, exp)

			warnLogs := observedLogs.FilterMessageSnippet("sending_queue is deprecated and will be removed in a future release")
			assert.Equal(t, tc.wantWarns, warnLogs.Len())
		})
	}
}
