// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"compress/gzip"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).Endpoints = []string{"https://elastic.example.com:9200"}

	defaultLogstashFormatCfg := createDefaultConfig()
	defaultLogstashFormatCfg.(*Config).Endpoints = []string{"http://localhost:9200"}
	defaultLogstashFormatCfg.(*Config).LogstashFormat.Enabled = true

	defaultRawCfg := createDefaultConfig()
	defaultRawCfg.(*Config).Endpoints = []string{"http://localhost:9200"}
	defaultRawCfg.(*Config).Mapping.Mode = "raw"

	defaultMaxIdleConns := 100
	defaultIdleConnTimeout := 90 * time.Second
	defaultCompression := configcompression.TypeGzip

	tests := []struct {
		configFile string
		id         component.ID
		expected   component.Config
	}{
		{
			id:         component.NewIDWithName(metadata.Type, ""),
			configFile: "config.yaml",
			expected:   defaultCfg,
		},
		{
			configFile: "config.yaml",
			id:         component.NewIDWithName(metadata.Type, "trace"),
			expected: &Config{
				QueueBatchConfig: exporterhelper.QueueBatchConfig{
					Enabled:         true,
					NumConsumers:    10,
					QueueSize:       10,
					BlockOnOverflow: true,
					Sizer:           exporterhelper.RequestSizerTypeRequests,
					Batch: configoptional.Some(exporterhelper.BatchConfig{
						FlushTimeout: 10 * time.Second,
						Sizer:        exporterhelper.RequestSizerTypeBytes,
						MinSize:      1000000,
						MaxSize:      5000000,
					}),
				},
				Endpoints: []string{
					"https://elastic.example.com:9200",
				},
				LogsDynamicIndex: DynamicIndexSetting{
					Enabled: false,
				},
				MetricsDynamicIndex: DynamicIndexSetting{
					Enabled: false,
				},
				TracesIndex: "trace_index",
				TracesDynamicIndex: DynamicIndexSetting{
					Enabled: false,
				},
				LogsDynamicID: DynamicIDSettings{
					Enabled: false,
				},
				LogsDynamicPipeline: DynamicPipelineSettings{
					Enabled: false,
				},
				Pipeline: "mypipeline",
				ClientConfig: withDefaultHTTPClientConfig(func(cfg *confighttp.ClientConfig) {
					cfg.Timeout = 2 * time.Minute
					cfg.MaxIdleConns = defaultMaxIdleConns
					cfg.IdleConnTimeout = defaultIdleConnTimeout
					cfg.Headers = configopaque.MapList{
						{Name: "myheader", Value: "test"},
					}
					cfg.Compression = defaultCompression
					cfg.CompressionParams.Level = gzip.BestSpeed
				},
				),
				Authentication: AuthenticationSettings{
					User:     "elastic",
					Password: "search",
					APIKey:   "AvFsEiPs==",
				},
				Discovery: DiscoverySettings{
					OnStart: true,
				},
				Retry: RetrySettings{
					Enabled:         true,
					MaxRetries:      5,
					InitialInterval: 100 * time.Millisecond,
					MaxInterval:     1 * time.Minute,
					RetryOnStatus: []int{
						http.StatusTooManyRequests,
						http.StatusInternalServerError,
					},
				},
				Mapping: MappingsSettings{
					Mode: "otel",
					AllowedModes: []string{
						"bodymap",
						"ecs",
						"none",
						"otel",
						"raw",
					},
				},
				LogstashFormat: LogstashFormatSettings{
					Enabled:         false,
					PrefixSeparator: "-",
					DateFormat:      "%Y.%m.%d",
				},
				TelemetrySettings: TelemetrySettings{
					LogFailedDocsInputRateLimit: time.Second,
				},
			},
		},
		{
			id:         component.NewIDWithName(metadata.Type, "log"),
			configFile: "config.yaml",
			expected: &Config{
				QueueBatchConfig: exporterhelper.QueueBatchConfig{
					Enabled:         true,
					NumConsumers:    10,
					QueueSize:       10,
					BlockOnOverflow: true,
					Sizer:           exporterhelper.RequestSizerTypeRequests,
					Batch: configoptional.Some(exporterhelper.BatchConfig{
						FlushTimeout: 10 * time.Second,
						Sizer:        exporterhelper.RequestSizerTypeBytes,
						MinSize:      1000000,
						MaxSize:      5000000,
					}),
				},
				Endpoints: []string{"http://localhost:9200"},
				LogsIndex: "my_log_index",
				LogsDynamicIndex: DynamicIndexSetting{
					Enabled: false,
				},
				MetricsDynamicIndex: DynamicIndexSetting{
					Enabled: false,
				},
				TracesDynamicIndex: DynamicIndexSetting{
					Enabled: false,
				},
				LogsDynamicID: DynamicIDSettings{
					Enabled: false,
				},
				LogsDynamicPipeline: DynamicPipelineSettings{
					Enabled: false,
				},
				Pipeline: "mypipeline",
				ClientConfig: withDefaultHTTPClientConfig(func(cfg *confighttp.ClientConfig) {
					cfg.Timeout = 2 * time.Minute
					cfg.MaxIdleConns = defaultMaxIdleConns
					cfg.IdleConnTimeout = defaultIdleConnTimeout
					cfg.Headers = configopaque.MapList{
						{Name: "myheader", Value: "test"},
					}
					cfg.Compression = defaultCompression
					cfg.CompressionParams.Level = gzip.BestSpeed
				}),
				Authentication: AuthenticationSettings{
					User:     "elastic",
					Password: "search",
					APIKey:   "AvFsEiPs==",
				},
				Discovery: DiscoverySettings{
					OnStart: true,
				},
				Retry: RetrySettings{
					Enabled:         true,
					MaxRetries:      5,
					InitialInterval: 100 * time.Millisecond,
					MaxInterval:     1 * time.Minute,
					RetryOnStatus:   []int{http.StatusTooManyRequests, http.StatusInternalServerError},
				},
				Mapping: MappingsSettings{
					Mode:         "otel",
					AllowedModes: []string{"bodymap", "ecs", "none", "otel", "raw"},
				},
				LogstashFormat: LogstashFormatSettings{
					Enabled:         false,
					PrefixSeparator: "-",
					DateFormat:      "%Y.%m.%d",
				},
				TelemetrySettings: TelemetrySettings{
					LogFailedDocsInputRateLimit: time.Second,
				},
			},
		},
		{
			id:         component.NewIDWithName(metadata.Type, "metric"),
			configFile: "config.yaml",
			expected: &Config{
				QueueBatchConfig: exporterhelper.QueueBatchConfig{
					Enabled:         true,
					NumConsumers:    10,
					QueueSize:       10,
					BlockOnOverflow: true,
					Sizer:           exporterhelper.RequestSizerTypeRequests,
					Batch: configoptional.Some(exporterhelper.BatchConfig{
						FlushTimeout: 10 * time.Second,
						Sizer:        exporterhelper.RequestSizerTypeBytes,
						MinSize:      1000000,
						MaxSize:      5000000,
					}),
				},
				Endpoints: []string{"http://localhost:9200"},
				LogsDynamicIndex: DynamicIndexSetting{
					Enabled: false,
				},
				MetricsIndex: "my_metric_index",
				MetricsDynamicIndex: DynamicIndexSetting{
					Enabled: false,
				},
				TracesDynamicIndex: DynamicIndexSetting{
					Enabled: false,
				},
				LogsDynamicID: DynamicIDSettings{
					Enabled: false,
				},
				LogsDynamicPipeline: DynamicPipelineSettings{
					Enabled: false,
				},
				Pipeline: "mypipeline",
				ClientConfig: withDefaultHTTPClientConfig(func(cfg *confighttp.ClientConfig) {
					cfg.Timeout = 2 * time.Minute
					cfg.MaxIdleConns = defaultMaxIdleConns
					cfg.IdleConnTimeout = defaultIdleConnTimeout
					cfg.Headers = configopaque.MapList{
						{Name: "myheader", Value: "test"},
					}
					cfg.Compression = defaultCompression
					cfg.CompressionParams.Level = gzip.BestSpeed
				}),
				Authentication: AuthenticationSettings{
					User:     "elastic",
					Password: "search",
					APIKey:   "AvFsEiPs==",
				},
				Discovery: DiscoverySettings{
					OnStart: true,
				},
				Retry: RetrySettings{
					Enabled:         true,
					MaxRetries:      5,
					InitialInterval: 100 * time.Millisecond,
					MaxInterval:     1 * time.Minute,
					RetryOnStatus:   []int{http.StatusTooManyRequests, http.StatusInternalServerError},
				},
				Mapping: MappingsSettings{
					Mode:         "otel",
					AllowedModes: []string{"bodymap", "ecs", "none", "otel", "raw"},
				},
				LogstashFormat: LogstashFormatSettings{
					Enabled:         false,
					PrefixSeparator: "-",
					DateFormat:      "%Y.%m.%d",
				},
				TelemetrySettings: TelemetrySettings{
					LogFailedDocsInputRateLimit: time.Second,
				},
			},
		},
		{
			id:         component.NewIDWithName(metadata.Type, "logstash_format"),
			configFile: "config.yaml",
			expected:   defaultLogstashFormatCfg,
		},
		{
			id:         component.NewIDWithName(metadata.Type, "raw"),
			configFile: "config.yaml",
			expected:   defaultRawCfg,
		},
		{
			id:         component.NewIDWithName(metadata.Type, "cloudid"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.CloudID = "foo:YmFyLmNsb3VkLmVzLmlvJGFiYzEyMyRkZWY0NTY="
			}),
		},
		{
			id:         component.NewIDWithName(metadata.Type, "confighttp_endpoint"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "https://elastic.example.com:9200"
			}),
		},
		{
			id:         component.NewIDWithName(metadata.Type, "compression_none"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "https://elastic.example.com:9200"

				cfg.Compression = "none"
			}),
		},
		{
			id:         component.NewIDWithName(metadata.Type, "compression_gzip"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "https://elastic.example.com:9200"

				cfg.Compression = "gzip"
			}),
		},
		{
			id:         component.NewIDWithName(metadata.Type, "include_source_on_error"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "https://elastic.example.com:9200"
				includeSource := true
				cfg.IncludeSourceOnError = &includeSource
			}),
		},
		{
			id:         component.NewIDWithName(metadata.Type, "metadata_keys"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "https://elastic.example.com:9200"

				cfg.MetadataKeys = []string{"x-test-1", "x-test-2"}
			}),
		},
		{
			id:         component.NewIDWithName(metadata.Type, "sendingqueue_disabled"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "https://elastic.example.com:9200"

				cfg.QueueBatchConfig.Enabled = false
			}),
		},
		{
			id:         component.NewIDWithName(metadata.Type, "sendingqueue_enabled"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "https://elastic.example.com:9200"

				cfg.QueueBatchConfig.NumConsumers = 100
				cfg.QueueBatchConfig.Batch = configoptional.Some(
					exporterhelper.BatchConfig{
						Sizer:        exporterhelper.RequestSizerTypeItems,
						FlushTimeout: time.Second,
						MinSize:      1000,
						MaxSize:      5000,
					},
				)
			}),
		},
		{
			id:         component.NewIDWithName(metadata.Type, "backward_compat_for_deprecated_cfgs/new_config_takes_priority"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "https://elastic.example.com:9200"

				cfg.NumWorkers = 11
				cfg.Flush = FlushSettings{
					Bytes:    1001,
					Interval: 11 * time.Second,
				}
				cfg.QueueBatchConfig.NumConsumers = 111
				// QueueBatchConfig is set by default
				qbCfg := cfg.QueueBatchConfig.Batch.Get()
				qbCfg.FlushTimeout = 111 * time.Second
				qbCfg.MaxSize = 1_000_001
				qbCfg.Sizer = exporterhelper.RequestSizerTypeBytes
			}),
		},
		{
			id:         component.NewIDWithName(metadata.Type, "backward_compat_for_deprecated_cfgs/fallback_to_old_cfg"),
			configFile: "config.yaml",
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "https://elastic.example.com:9200"

				cfg.NumWorkers = 11
				cfg.Flush = FlushSettings{
					Bytes:    1_000_001,
					Interval: 11 * time.Second,
				}
				cfg.QueueBatchConfig.NumConsumers = 11
				// QueueBatchConfig is set by default
				qbCfg := cfg.QueueBatchConfig.Batch.Get()
				qbCfg.FlushTimeout = 11 * time.Second
				qbCfg.MaxSize = 1_000_001
				qbCfg.Sizer = exporterhelper.RequestSizerTypeBytes
			}),
		},
	}

	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.id.String(), "/", "_"), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.configFile))
			require.NoError(t, err)

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// TestConfig_Validate tests the error cases of Config.Validate.
//
// Successful validation should be covered by TestConfig above.
func TestConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		config *Config
		err    string
	}{
		"no endpoints": {
			config: withDefaultConfig(),
			err:    "exactly one of [endpoint, endpoints, cloudid] must be specified",
		},
		"empty endpoint": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{""}
			}),
			err: `invalid endpoint "": endpoint must not be empty`,
		},
		"invalid endpoint": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"*:!"}
			}),
			err: `invalid endpoint "*:!": parse "*:!": first path segment in URL cannot contain colon`,
		},
		"invalid cloudid": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.CloudID = "invalid"
			}),
			err: `invalid CloudID "invalid"`,
		},
		"invalid decoded cloudid": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.CloudID = "foo:YWJj"
			}),
			err: `invalid decoded CloudID "abc"`,
		},
		"endpoints and cloudid both set": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"http://test:9200"}
				cfg.CloudID = "foo:YmFyLmNsb3VkLmVzLmlvJGFiYzEyMyRkZWY0NTY="
			}),
			err: "exactly one of [endpoint, endpoints, cloudid] must be specified",
		},
		"endpoint and endpoints both set": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoint = "http://test:9200"
				cfg.Endpoints = []string{"http://test:9200"}
			}),
			err: "exactly one of [endpoint, endpoints, cloudid] must be specified",
		},
		"invalid mapping mode": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"http://test:9200"}
				cfg.Mapping.Mode = "invalid"
			}),
			err: `invalid or disallowed default mapping mode "invalid"`,
		},
		"invalid allowed mapping modes": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"http://test:9200"}
				cfg.Mapping.AllowedModes = []string{"foo"}
			}),
			err: `unknown allowed mapping mode name "foo"`,
		},
		"disallowed mapping mode": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"http://test:9200"}
				cfg.Mapping.Mode = "otel"
				cfg.Mapping.AllowedModes = []string{"raw"}
			}),
			err: `invalid or disallowed default mapping mode "otel"`,
		},
		"invalid scheme": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"without_scheme"}
			}),
			err: `invalid endpoint "without_scheme": invalid scheme "", expected "http" or "https"`,
		},
		"compression unsupported": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"http://test:9200"}
				cfg.Compression = configcompression.TypeSnappy
			}),
			err: `compression must be one of [none, gzip]`,
		},
		"both max_retries and max_requests specified": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"http://test:9200"}
				cfg.Retry.MaxRetries = 1
				cfg.Retry.MaxRequests = 1
			}),
			err: `must not specify both retry::max_requests and retry::max_retries`,
		},
		"duplicate metadata_keys specified": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"http://test:9200"}
				cfg.MetadataKeys = []string{"x-test-1", "x-test-2", "x-test-1"}
			}),
			err: `metadata_keys must be case-insenstive and unique, found duplicate: x-test-1`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.ErrorContains(t, xconfmap.Validate(tt.config), tt.err)
		})
	}
}

func TestConfig_Validate_Environment(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("ELASTICSEARCH_URL", "http://test:9200")
		config := withDefaultConfig()
		err := xconfmap.Validate(config)
		require.NoError(t, err)
	})
	t.Run("invalid", func(t *testing.T) {
		t.Setenv("ELASTICSEARCH_URL", "http://valid:9200, *:!")
		config := withDefaultConfig()
		err := xconfmap.Validate(config)
		assert.ErrorContains(t, err, `invalid endpoint "*:!": parse "*:!": first path segment in URL cannot contain colon`)
	})
}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
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
