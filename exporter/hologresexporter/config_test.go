// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).DSN = "postgresql://user:password@localhost:5432/otel?sslmode=disable"

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: defaultCfg,
		},
		{
			id: component.NewIDWithName(metadata.Type, "full"),
			expected: &Config{
				TimeoutConfig: exporterhelper.TimeoutConfig{
					Timeout: 30 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      300 * time.Second,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: configoptional.Some(func() exporterhelper.QueueBatchConfig {
					queue := exporterhelper.NewDefaultQueueConfig()
					queue.NumConsumers = 10
					queue.QueueSize = 1000
					return queue
				}()),
				DSN:              "postgresql://user:password@localhost:5432/otel?sslmode=disable",
				TracesTableName:  "custom_traces",
				LogsTableName:    "custom_logs",
				MetricsTableName: "custom_metrics",
				CreateSchema:     true,
				TTL:              168 * time.Hour,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		errMsg string
	}{
		{
			name: "valid config",
			cfg: &Config{
				DSN:              "postgresql://user:password@localhost:5432/otel?sslmode=disable",
				TracesTableName:  "otel_traces",
				LogsTableName:    "otel_logs",
				MetricsTableName: "otel_metrics",
			},
			errMsg: "",
		},
		{
			name: "empty dsn",
			cfg: &Config{
				DSN:              "",
				TracesTableName:  "otel_traces",
				LogsTableName:    "otel_logs",
				MetricsTableName: "otel_metrics",
			},
			errMsg: "dsn must be specified",
		},
		{
			name: "empty traces table name",
			cfg: &Config{
				DSN:              "postgresql://user:password@localhost:5432/otel?sslmode=disable",
				TracesTableName:  "",
				LogsTableName:    "otel_logs",
				MetricsTableName: "otel_metrics",
			},
			errMsg: "traces_table_name must not be empty",
		},
		{
			name: "empty logs table name",
			cfg: &Config{
				DSN:              "postgresql://user:password@localhost:5432/otel?sslmode=disable",
				TracesTableName:  "otel_traces",
				LogsTableName:    "",
				MetricsTableName: "otel_metrics",
			},
			errMsg: "logs_table_name must not be empty",
		},
		{
			name: "empty metrics table name",
			cfg: &Config{
				DSN:              "postgresql://user:password@localhost:5432/otel?sslmode=disable",
				TracesTableName:  "otel_traces",
				LogsTableName:    "otel_logs",
				MetricsTableName: "",
			},
			errMsg: "metrics_table_name must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.errMsg == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errMsg)
			}
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg)

	hCfg, ok := cfg.(*Config)
	require.True(t, ok)

	assert.Equal(t, "otel_traces", hCfg.TracesTableName)
	assert.Equal(t, "otel_logs", hCfg.LogsTableName)
	assert.Equal(t, "otel_metrics", hCfg.MetricsTableName)
	assert.True(t, hCfg.CreateSchema)
	assert.Equal(t, time.Duration(0), hCfg.TTL)
	assert.Equal(t, "", hCfg.DSN)
}
