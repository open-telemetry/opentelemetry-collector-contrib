// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/open-telemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

// Bounds are only validated when the event is enabled, so these tests must turn it on.
func procedureMetricsEnabledLogsConfig() metadata.LogsBuilderConfig {
	logsCfg := metadata.DefaultLogsBuilderConfig()
	logsCfg.Events.DbServerTopProcedure.Enabled = true
	return logsCfg
}

// A deployment that never enables the event must not be held to bounds it does not use.
func TestProcedureMetricsBoundsOnlyValidatedWhenEnabled(t *testing.T) {
	cfg := &Config{
		DataSource:         "oracle://otel:password@localhost:1521/XE",
		ControllerConfig:   scraperhelper.NewDefaultControllerConfig(),
		LogsBuilderConfig:  metadata.DefaultLogsBuilderConfig(),
		TopQueryCollection: TopQueryCollection{MaxQuerySampleCount: 1000, TopQueryCount: 200},
		ProcedureMetrics:   ProcedureMetrics{MaxProcedureSampleCount: 0, TopProcedureCount: 0},
	}

	require.NoError(t, cfg.Validate(), "out-of-range procedure bounds must not fail validation while the event is disabled")

	cfg.LogsBuilderConfig = procedureMetricsEnabledLogsConfig()
	require.Error(t, cfg.Validate(), "the same bounds must fail once the event is enabled")
}

func TestValidateInvalidConfigs(t *testing.T) {
	testCases := []struct {
		name     string
		config   *Config
		expected error
	}{
		{
			name: "Empty endpoint",
			config: &Config{
				Endpoint:         "",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyEndpoint,
		},
		{
			name: "Missing port in endpoint",
			config: &Config{
				Endpoint:         "localhost",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadEndpoint,
		},
		{
			name: "Invalid endpoint format",
			config: &Config{
				Endpoint:         "x;;ef;s;d:::ss:23423423423423423",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadEndpoint,
		},
		{
			name: "Missing host in endpoint",
			config: &Config{
				Endpoint:         ":3001",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadEndpoint,
		},
		{
			name: "Negative port",
			config: &Config{
				Endpoint:         "localhost:-2",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadPort,
		},
		{
			name: "Bad port",
			config: &Config{
				Endpoint:         "localhost:9999999999999999999",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadPort,
		},
		{
			name: "Empty username",
			config: &Config{
				Endpoint:         "localhost:3000",
				Username:         "",
				Password:         "secret",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyUsername,
		},
		{
			name: "Empty password",
			config: &Config{
				Endpoint:         "localhost:3000",
				Username:         "ro_user",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyPassword,
		},
		{
			name: "Empty service",
			config: &Config{
				Endpoint:         "localhost:3000",
				Password:         "password",
				Username:         "ro_user",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyService,
		},
		{
			name: "Invalid data source",
			config: &Config{
				DataSource:       "%%%",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadDataSource,
		},
		{
			name: "Top procedure count below range",
			config: &Config{
				DataSource:         "oracle://otel:password@localhost:1521/XE",
				ControllerConfig:   scraperhelper.NewDefaultControllerConfig(),
				LogsBuilderConfig:  procedureMetricsEnabledLogsConfig(),
				TopQueryCollection: TopQueryCollection{MaxQuerySampleCount: 1000, TopQueryCount: 200},
				ProcedureMetrics:   ProcedureMetrics{MaxProcedureSampleCount: 1000, TopProcedureCount: 0},
			},
			expected: errTopProcedureCount,
		},
		{
			name: "Top procedure count above range",
			config: &Config{
				DataSource:         "oracle://otel:password@localhost:1521/XE",
				ControllerConfig:   scraperhelper.NewDefaultControllerConfig(),
				LogsBuilderConfig:  procedureMetricsEnabledLogsConfig(),
				TopQueryCollection: TopQueryCollection{MaxQuerySampleCount: 1000, TopQueryCount: 200},
				ProcedureMetrics:   ProcedureMetrics{MaxProcedureSampleCount: 1000, TopProcedureCount: 1001},
			},
			expected: errTopProcedureCount,
		},
		{
			name: "Top procedure count above max procedure sample count",
			config: &Config{
				DataSource:         "oracle://otel:password@localhost:1521/XE",
				ControllerConfig:   scraperhelper.NewDefaultControllerConfig(),
				LogsBuilderConfig:  procedureMetricsEnabledLogsConfig(),
				TopQueryCollection: TopQueryCollection{MaxQuerySampleCount: 1000, TopQueryCount: 200},
				ProcedureMetrics:   ProcedureMetrics{MaxProcedureSampleCount: 100, TopProcedureCount: 250},
			},
			expected: errTopProcedureCount,
		},
		{
			name: "Max procedure sample count above range",
			config: &Config{
				DataSource:         "oracle://otel:password@localhost:1521/XE",
				ControllerConfig:   scraperhelper.NewDefaultControllerConfig(),
				LogsBuilderConfig:  procedureMetricsEnabledLogsConfig(),
				TopQueryCollection: TopQueryCollection{MaxQuerySampleCount: 1000, TopQueryCount: 200},
				ProcedureMetrics:   ProcedureMetrics{MaxProcedureSampleCount: 10001, TopProcedureCount: 250},
			},
			expected: errMaxProcedureSampleCount,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			require.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, 10*time.Second, cfg.ControllerConfig.CollectionInterval)
	assert.Equal(t, uint(1000), cfg.ProcedureMetrics.MaxProcedureSampleCount)
	assert.Equal(t, uint(250), cfg.ProcedureMetrics.TopProcedureCount)
	assert.Equal(t, time.Minute, cfg.ProcedureMetrics.CollectionInterval)
}

func TestParseConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	sub, err := cm.Sub("oracledb")
	require.NoError(t, err)
	cfg := createDefaultConfig().(*Config)

	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	assert.Equal(t, "oracle://otel:password@localhost:51521/XE", cfg.DataSource)
	assert.Equal(t, "otel", cfg.Username)
	assert.Equal(t, "password", cfg.Password)
	assert.Equal(t, "localhost:51521", cfg.Endpoint)
	assert.Equal(t, "XE", cfg.Service)
	settings := cfg.MetricsBuilderConfig.Metrics
	assert.False(t, settings.OracledbTablespaceSizeUsage.Enabled)
	assert.False(t, settings.OracledbExchangeDeadlocks.Enabled)
	assert.Equal(t, uint(555), cfg.ProcedureMetrics.MaxProcedureSampleCount)
	assert.Equal(t, uint(111), cfg.ProcedureMetrics.TopProcedureCount)
	assert.True(t, cfg.LogsBuilderConfig.Events.DbServerTopProcedure.Enabled)
}
