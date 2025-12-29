// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starrocksexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/metrics"
)

func TestConfig_DefaultValues(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg)

	starRocksCfg := cfg.(*Config)
	assert.Equal(t, defaultDatabase, starRocksCfg.Database)
	assert.Equal(t, "otel_logs", starRocksCfg.LogsTableName)
	assert.Equal(t, "otel_traces", starRocksCfg.TracesTableName)
	assert.True(t, starRocksCfg.CreateSchema)
	assert.Equal(t, defaultMaxOpenConns, starRocksCfg.MaxOpenConns)
	assert.Equal(t, defaultMaxIdleConns, starRocksCfg.MaxIdleConns)
	assert.Equal(t, defaultConnMaxLifetime, starRocksCfg.ConnMaxLifetime)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Endpoint: "localhost:9030",
				Username: "root",
				Password: "password",
				Database: "otel",
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			config: &Config{
				Username: "root",
				Password: "password",
				Database: "otel",
			},
			wantErr: true,
		},
		{
			name: "invalid endpoint format",
			config: &Config{
				Endpoint: "invalid",
				Username: "root",
				Password: "password",
			},
			wantErr: false, // MySQL driver will handle validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_buildDSN(t *testing.T) {
	cfg := &Config{
		Endpoint: "localhost:9030",
		Username: "root",
		Password: configopaque.String("password"),
		Database: "otel",
		ConnectionParams: map[string]string{
			"timeout": "10s",
		},
	}

	dsn, err := cfg.buildDSN()
	require.NoError(t, err)
	assert.Contains(t, dsn, "root:password@tcp(localhost:9030)/otel")
	assert.Contains(t, dsn, "timeout=10s")
}

func TestConfig_buildMetricTableNames(t *testing.T) {
	cfg := &Config{
		MetricsTables: MetricTablesConfig{},
	}

	cfg.buildMetricTableNames()

	assert.Equal(t, defaultMetricTableName+defaultGaugeSuffix, cfg.MetricsTables.Gauge.Name)
	assert.Equal(t, defaultMetricTableName+defaultSumSuffix, cfg.MetricsTables.Sum.Name)
	assert.Equal(t, defaultMetricTableName+defaultSummarySuffix, cfg.MetricsTables.Summary.Name)
	assert.Equal(t, defaultMetricTableName+defaultHistogramSuffix, cfg.MetricsTables.Histogram.Name)
	assert.Equal(t, defaultMetricTableName+defaultExpHistogramSuffix, cfg.MetricsTables.ExponentialHistogram.Name)
}

func TestConfig_buildMetricTableNames_WithDeprecatedMetricsTableName(t *testing.T) {
	cfg := &Config{
		MetricsTableName: "custom_metrics",
		MetricsTables:    MetricTablesConfig{},
	}

	cfg.buildMetricTableNames()

	assert.Equal(t, "custom_metrics"+defaultGaugeSuffix, cfg.MetricsTables.Gauge.Name)
	assert.Equal(t, "custom_metrics"+defaultSumSuffix, cfg.MetricsTables.Sum.Name)
}

func TestConfig_buildMetricTableNames_WithCustomNames(t *testing.T) {
	cfg := &Config{
		MetricsTables: MetricTablesConfig{
			Gauge: metrics.MetricTypeConfig{Name: "custom_gauge"},
		},
	}

	cfg.buildMetricTableNames()

	assert.Equal(t, "custom_gauge", cfg.MetricsTables.Gauge.Name)
	assert.Equal(t, defaultMetricTableName+defaultSumSuffix, cfg.MetricsTables.Sum.Name)
}

func TestConfig_database(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "custom database",
			config: &Config{
				Database: "custom_db",
			},
			expected: "custom_db",
		},
		{
			name: "default database",
			config: &Config{
				Database: defaultDatabase,
			},
			expected: defaultDatabase,
		},
		{
			name: "empty database",
			config: &Config{
				Database: "",
			},
			expected: defaultDatabase,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.database())
		})
	}
}

func TestConfig_shouldCreateSchema(t *testing.T) {
	cfg := &Config{CreateSchema: true}
	assert.True(t, cfg.shouldCreateSchema())

	cfg.CreateSchema = false
	assert.False(t, cfg.shouldCreateSchema())
}

func TestConfig_areMetricTableNamesSet(t *testing.T) {
	cfg := &Config{
		MetricsTables: MetricTablesConfig{},
	}
	assert.False(t, cfg.areMetricTableNamesSet())

	cfg.MetricsTables.Gauge.Name = "test"
	assert.True(t, cfg.areMetricTableNamesSet())
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).Endpoint = "localhost:9030"
	defaultCfg.(*Config).Username = "root"
	defaultCfg.(*Config).Password = "password"
	defaultCfg.(*Config).Database = "otel" // Match testdata/config.yaml

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
				collectorVersion: "unknown",
				Endpoint:         "localhost:9030",
				Username:         "root",
				Password:         "password",
				Database:         "otel",
				LogsTableName:    "otel_logs",
				TracesTableName:  "otel_traces",
				CreateSchema:     true,
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 5 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      300 * time.Second,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				MetricsTables: MetricTablesConfig{
					Gauge:                metrics.MetricTypeConfig{Name: "otel_metrics_custom_gauge"},
					Sum:                  metrics.MetricTypeConfig{Name: "otel_metrics_custom_sum"},
					Summary:              metrics.MetricTypeConfig{Name: "otel_metrics_custom_summary"},
					Histogram:            metrics.MetricTypeConfig{Name: "otel_metrics_custom_histogram"},
					ExponentialHistogram: metrics.MetricTypeConfig{Name: "otel_metrics_custom_exp_histogram"},
				},
				ConnectionParams: map[string]string{},
				QueueSettings: configoptional.Some(func() exporterhelper.QueueBatchConfig {
					queue := exporterhelper.NewDefaultQueueConfig()
					queue.QueueSize = 100
					return queue
				}()),
				TLS: configtls.ClientConfig{
					Config: configtls.Config{
						CertFile: "client.crt",
						KeyFile:  "client.key",
					},
				},
				MaxOpenConns:    20,
				MaxIdleConns:    10,
				ConnMaxLifetime: 10 * time.Minute,
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

			// For full config, compare all fields
			if tt.id.Name() == "full" {
				actualCfg := cfg.(*Config)
				expectedCfg := tt.expected.(*Config)

				assert.Equal(t, expectedCfg.Endpoint, actualCfg.Endpoint)
				assert.Equal(t, expectedCfg.Username, actualCfg.Username)
				assert.Equal(t, expectedCfg.Password, actualCfg.Password)
				assert.Equal(t, expectedCfg.Database, actualCfg.Database)
				assert.Equal(t, expectedCfg.LogsTableName, actualCfg.LogsTableName)
				assert.Equal(t, expectedCfg.TracesTableName, actualCfg.TracesTableName)
				assert.Equal(t, expectedCfg.CreateSchema, actualCfg.CreateSchema)
				assert.Equal(t, expectedCfg.MaxOpenConns, actualCfg.MaxOpenConns)
				assert.Equal(t, expectedCfg.MaxIdleConns, actualCfg.MaxIdleConns)
				assert.Equal(t, expectedCfg.ConnMaxLifetime, actualCfg.ConnMaxLifetime)
				assert.Equal(t, expectedCfg.MetricsTables, actualCfg.MetricsTables)
				assert.Equal(t, expectedCfg.TimeoutSettings.Timeout, actualCfg.TimeoutSettings.Timeout)
				assert.Equal(t, expectedCfg.BackOffConfig.Enabled, actualCfg.BackOffConfig.Enabled)
				assert.Equal(t, expectedCfg.BackOffConfig.InitialInterval, actualCfg.BackOffConfig.InitialInterval)
				assert.Equal(t, expectedCfg.BackOffConfig.MaxInterval, actualCfg.BackOffConfig.MaxInterval)
				assert.Equal(t, expectedCfg.BackOffConfig.MaxElapsedTime, actualCfg.BackOffConfig.MaxElapsedTime)
				assert.Equal(t, expectedCfg.TLS.CertFile, actualCfg.TLS.CertFile)
				assert.Equal(t, expectedCfg.TLS.KeyFile, actualCfg.TLS.KeyFile)
			} else {
				// For basic config, just verify it was loaded correctly
				actualCfg := cfg.(*Config)
				expectedCfg := tt.expected.(*Config)
				assert.Equal(t, expectedCfg.Endpoint, actualCfg.Endpoint)
				assert.Equal(t, expectedCfg.Username, actualCfg.Username)
				assert.Equal(t, expectedCfg.Password, actualCfg.Password)
				assert.Equal(t, expectedCfg.Database, actualCfg.Database)
			}
		})
	}
}
