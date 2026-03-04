// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	sampleEndpoint := "https://opensearch.example.com:9200"
	sampleCfg := withDefaultConfig(func(config *Config) {
		config.Endpoint = sampleEndpoint
		config.BulkAction = defaultBulkAction
	})
	maxIdleConns := 100
	idleConnTimeout := 90 * time.Second

	tests := []struct {
		id                   component.ID
		expected             component.Config
		configValidateAssert assert.ErrorAssertionFunc
	}{
		{
			id:                   component.NewIDWithName(metadata.Type, ""),
			expected:             sampleCfg,
			configValidateAssert: assert.NoError,
		},
		{
			id:       component.NewIDWithName(metadata.Type, "default"),
			expected: withDefaultConfig(),
			configValidateAssert: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, "endpoint must be specified")
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "trace"),
			expected: &Config{
				Dataset:   "ngnix",
				Namespace: "eu",
				ClientConfig: withDefaultHTTPClientConfig(func(config *confighttp.ClientConfig) {
					config.Endpoint = sampleEndpoint
					config.Timeout = 2 * time.Minute
					config.Headers = configopaque.MapList{
						{Name: "myheader", Value: "test"},
					}
					config.MaxIdleConns = maxIdleConns
					config.IdleConnTimeout = idleConnTimeout
					config.Auth = configoptional.Some(configauth.Config{AuthenticatorID: component.MustNewID("sample_basic_auth")})
				}),
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     100 * time.Millisecond,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
					Multiplier:          1.5,
					RandomizationFactor: 0.5,
				},
				BulkAction: defaultBulkAction,
				MappingsSettings: MappingsSettings{
					Mode: "ss4o",
				},
				QueueConfig: configoptional.Default(exporterhelper.NewDefaultQueueConfig()),
			},
			configValidateAssert: assert.NoError,
		},
		{
			id: component.NewIDWithName(metadata.Type, "empty_dataset"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.Dataset = ""
				config.Namespace = "eu"
			}),
			configValidateAssert: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, errDatasetNoValue.Error())
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "empty_namespace"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.Dataset = "ngnix"
				config.Namespace = ""
			}),
			configValidateAssert: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, errNamespaceNoValue.Error())
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "invalid_bulk_action"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.BulkAction = "delete"
			}),
			configValidateAssert: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, errBulkActionInvalid.Error())
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "dynamic_log_indexing"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.LogsIndex = "otel-logs-%{service.name}"
				config.LogsIndexFallback = "default-service"
				config.LogsIndexTimeFormat = "yyyy.MM.dd"
			}),
			configValidateAssert: assert.NoError,
		},

		{
			id: component.NewIDWithName(metadata.Type, "log_index_time_format_valid"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.LogsIndex = "otel-logs-%{service.name}"
				config.LogsIndexFallback = "default-service"
				config.LogsIndexTimeFormat = "yyyy.MM.dd"
			}),
			configValidateAssert: assert.NoError,
		},
		{
			id: component.NewIDWithName(metadata.Type, "log_index_time_format_empty"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.LogsIndex = "otel-logs-%{service.name}"
				config.LogsIndexFallback = "default-service"
				config.LogsIndexTimeFormat = ""
			}),
			configValidateAssert: assert.NoError,
		},
		{
			id: component.NewIDWithName(metadata.Type, "log_index_time_format_invalid"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.LogsIndex = "otel-logs-%{service.name}"
				config.LogsIndexFallback = "default-service"
				config.LogsIndexTimeFormat = "invalid_format!"
			}),
			configValidateAssert: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, errLogsIndexTimeFormatInvalid.Error())
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "log_index_time_format_whitespace"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.LogsIndex = "otel-logs-%{service.name}"
				config.LogsIndexFallback = "default-service"
				config.LogsIndexTimeFormat = "   "
			}),
			configValidateAssert: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, errLogsIndexTimeFormatInvalid.Error())
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "log_index_time_format_special_chars"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.LogsIndex = "otel-logs-%{service.name}"
				config.LogsIndexFallback = "default-service"
				config.LogsIndexTimeFormat = "yyyy/MM/dd@!#"
			}),
			configValidateAssert: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, errLogsIndexTimeFormatInvalid.Error())
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "traces_index_valid"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.TracesIndex = "otel-traces-%{service.name}"
				config.TracesIndexFallback = "default-service"
				config.TracesIndexTimeFormat = "yyyy.MM.dd"
			}),
			configValidateAssert: assert.NoError,
		},

		{
			id: component.NewIDWithName(metadata.Type, "traces_index_time_format_valid"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.TracesIndex = "otel-traces-%{service.name}"
				config.TracesIndexFallback = "default-service"
				config.TracesIndexTimeFormat = "yyyy.MM.dd"
			}),
			configValidateAssert: assert.NoError,
		},
		{
			id: component.NewIDWithName(metadata.Type, "traces_index_time_format_empty"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.TracesIndex = "otel-traces-%{service.name}"
				config.TracesIndexFallback = "default-service"
				config.TracesIndexTimeFormat = ""
			}),
			configValidateAssert: assert.NoError,
		},
		{
			id: component.NewIDWithName(metadata.Type, "traces_index_time_format_invalid"),
			expected: withDefaultConfig(func(config *Config) {
				config.Endpoint = sampleEndpoint
				config.TracesIndex = "otel-traces-%{service.name}"
				config.TracesIndexFallback = "default-service"
				config.TracesIndexTimeFormat = "invalid_format!"
			}),
			configValidateAssert: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorContains(t, err, errTracesIndexTimeFormatInvalid.Error())
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

			vv := xconfmap.Validate(cfg)
			tt.configValidateAssert(t, vv)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestQueueConfigDefaults verifies that sending_queue gets proper defaults when only batch is configured
func TestQueueConfigDefaults(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "sending_queue_with_batch").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	actualCfg := cfg.(*Config)

	// Verify QueueConfig has the expected defaults
	require.True(t, actualCfg.QueueConfig.HasValue(), "QueueConfig should have a value")
	queueCfg := actualCfg.QueueConfig.Get()
	assert.Equal(t, 10, queueCfg.NumConsumers, "NumConsumers should default to 10")
	assert.Equal(t, int64(1000), queueCfg.QueueSize, "QueueSize should default to 1000")
	assert.True(t, queueCfg.Batch.HasValue(), "Batch should be configured")

	// Verify config is valid (no crash)
	require.NoError(t, actualCfg.Validate())
}

// withDefaultConfig create a new default configuration
// and applies provided functions to it.
func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := newDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}

func withDefaultHTTPClientConfig(fns ...func(config *confighttp.ClientConfig)) confighttp.ClientConfig {
	cfg := confighttp.NewDefaultClientConfig()
	for _, fn := range fns {
		fn(&cfg)
	}
	return cfg
}
