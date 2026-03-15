// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		subName      string
		expected     *Config
		errorMessage string
	}{
		{
			id:      component.NewIDWithName(component.MustNewType(typeStr), ""),
			subName: "tinybird",
			expected: &Config{
				ClientConfig: func() confighttp.ClientConfig {
					cfg := createDefaultConfig().(*Config).ClientConfig
					cfg.Endpoint = "https://api.tinybird.co"
					return cfg
				}(),
				RetryConfig: configretry.NewDefaultBackOffConfig(),
				QueueConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				Token:       "test-token",
				Metrics: metricSignalConfigs{
					MetricsGauge:                SignalConfig{Datasource: "gauge"},
					MetricsSum:                  SignalConfig{Datasource: "sum"},
					MetricsHistogram:            SignalConfig{Datasource: "histogram"},
					MetricsExponentialHistogram: SignalConfig{Datasource: "exponential_histogram"},
				},
				Traces: SignalConfig{Datasource: "traces"},
				Logs:   SignalConfig{Datasource: "logs"},
			},
		},
		{
			id:      component.NewIDWithName(component.MustNewType(typeStr), "full"),
			subName: "tinybird/full",
			expected: &Config{
				ClientConfig: func() confighttp.ClientConfig {
					cfg := createDefaultConfig().(*Config).ClientConfig
					cfg.Endpoint = "https://api.tinybird.co"
					cfg.Compression = configcompression.TypeZstd
					return cfg
				}(),
				RetryConfig: func() configretry.BackOffConfig {
					cfg := createDefaultConfig().(*Config).RetryConfig
					cfg.Enabled = false
					return cfg
				}(),
				QueueConfig: configoptional.None[exporterhelper.QueueBatchConfig](),
				Token:       "test-token",
				Metrics: metricSignalConfigs{
					MetricsGauge:                SignalConfig{Datasource: "gauge"},
					MetricsSum:                  SignalConfig{Datasource: "sum"},
					MetricsHistogram:            SignalConfig{Datasource: "histogram"},
					MetricsExponentialHistogram: SignalConfig{Datasource: "exponential_histogram"},
				},
				Traces: SignalConfig{Datasource: "traces"},
				Logs:   SignalConfig{Datasource: "logs"},
				Wait:   true,
			},
		},
		{
			id:      component.NewIDWithName(component.MustNewType(typeStr), "invalid_datasource"),
			subName: "tinybird/invalid_datasource",
			errorMessage: "metrics::gauge: invalid datasource \"metrics-with-dashes\": only letters, numbers, and underscores are allowed" + "\n" +
				"metrics::sum: invalid datasource \"metrics-with-dashes\": only letters, numbers, and underscores are allowed" + "\n" +
				"metrics::histogram: invalid datasource \"metrics-with-dashes\": only letters, numbers, and underscores are allowed" + "\n" +
				"metrics::exponential_histogram: invalid datasource \"metrics-with-dashes\": only letters, numbers, and underscores are allowed" + "\n" +
				"traces: invalid datasource \"traces-with-dashes\": only letters, numbers, and underscores are allowed" + "\n" +
				"logs: invalid datasource \"logs-with-dashes\": only letters, numbers, and underscores are allowed",
		},
		{
			id:           component.NewIDWithName(component.MustNewType(typeStr), "missing_token"),
			subName:      "tinybird/missing_token",
			errorMessage: "missing Tinybird API token",
		},
		{
			id:           component.NewIDWithName(component.MustNewType(typeStr), "missing_endpoint"),
			subName:      "tinybird/missing_endpoint",
			errorMessage: "missing Tinybird API endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
