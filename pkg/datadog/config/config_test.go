// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestValidate(t *testing.T) {
	idleConnTimeout := 30 * time.Second
	maxIdleConn := 300
	maxIdleConnPerHost := 150
	maxConnPerHost := 250
	ty, err := component.NewType("ty")
	assert.NoError(t, err)
	auth := configauth.Config{AuthenticatorID: component.NewID(ty)}

	tests := []struct {
		name string
		cfg  *Config
		err  string
	}{
		{
			name: "no api::key",
			cfg: &Config{
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
			err: ErrUnsetAPIKey.Error(),
		},
		{
			name: "invalid format api::key",
			cfg: &Config{
				API: APIConfig{Key: "'aaaaaaa"},
			},
			err: ErrAPIKeyFormat.Error(),
		},
		{
			name: "invalid hostname",
			cfg: &Config{
				API:          APIConfig{Key: "aaaaaaa"},
				TagsConfig:   TagsConfig{Hostname: "invalid_host"},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
			err: "hostname field is invalid: invalid_host is not RFC1123 compliant",
		},
		{
			name: "no metadata",
			cfg: &Config{
				API:          APIConfig{Key: "aaaaaaa"},
				OnlyMetadata: true,
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
			err: ErrNoMetadata.Error(),
		},
		{
			name: "span name remapping valid",
			cfg: &Config{
				API:          APIConfig{Key: "aaaaaaa"},
				Traces:       TracesExporterConfig{TracesConfig: TracesConfig{SpanNameRemappings: map[string]string{"old.opentelemetryspan.name": "updated.name"}}},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
		},
		{
			name: "span name remapping empty val",
			cfg: &Config{
				API:          APIConfig{Key: "aaaaaaa"},
				Traces:       TracesExporterConfig{TracesConfig: TracesConfig{SpanNameRemappings: map[string]string{"oldname": ""}}},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
			err: "'' is not valid value for span name remapping",
		},
		{
			name: "span name remapping empty key",
			cfg: &Config{
				API:          APIConfig{Key: "aaaaaaa"},
				Traces:       TracesExporterConfig{TracesConfig: TracesConfig{SpanNameRemappings: map[string]string{"": "newname"}}},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
			err: "'' is not valid key for span name remapping",
		},
		{
			name: "ignore resources valid",
			cfg: &Config{
				API:          APIConfig{Key: "aaaaaaa"},
				Traces:       TracesExporterConfig{TracesConfig: TracesConfig{IgnoreResources: []string{"[123]"}}},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
		},
		{
			name: "ignore resources missing bracket",
			cfg: &Config{
				API:          APIConfig{Key: "aaaaaaa"},
				Traces:       TracesExporterConfig{TracesConfig: TracesConfig{IgnoreResources: []string{"[123"}}},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
			err: "'[123' is not valid resource filter regular expression",
		},
		{
			name: "invalid histogram settings",
			cfg: &Config{
				API: APIConfig{Key: "aaaaaaa"},
				Metrics: MetricsConfig{
					HistConfig: HistogramConfig{
						Mode:             HistogramModeNoBuckets,
						SendAggregations: false,
					},
				},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
			err: "'nobuckets' mode and `send_aggregation_metrics` set to false will send no histogram metrics",
		},
		{
			name: "TLS settings are valid",
			cfg: &Config{
				API: APIConfig{Key: "aaaaaaa"},
				ClientConfig: confighttp.ClientConfig{
					TLSSetting: configtls.ClientConfig{
						InsecureSkipVerify: true,
					},
				},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
		},
		{
			name: "With trace_buffer",
			cfg: &Config{
				API:          APIConfig{Key: "aaaaaaa"},
				Traces:       TracesExporterConfig{TraceBuffer: 10},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
		},
		{
			name: "With peer_tags",
			cfg: &Config{
				API: APIConfig{Key: "aaaaaaa"},
				Traces: TracesExporterConfig{
					TracesConfig: TracesConfig{
						PeerTags: []string{"tag1", "tag2"},
					},
				},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
		},
		{
			name: "With confighttp client configs",
			cfg: &Config{
				API: APIConfig{Key: "aaaaaaa"},
				ClientConfig: confighttp.ClientConfig{
					ReadBufferSize:      100,
					WriteBufferSize:     200,
					Timeout:             10 * time.Second,
					IdleConnTimeout:     idleConnTimeout,
					MaxIdleConns:        maxIdleConn,
					MaxIdleConnsPerHost: maxIdleConnPerHost,
					MaxConnsPerHost:     maxConnPerHost,
					DisableKeepAlives:   true,
					TLSSetting:          configtls.ClientConfig{InsecureSkipVerify: true},
				},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
		},

		{
			name: "unsupported confighttp client configs",
			cfg: &Config{
				API: APIConfig{Key: "aaaaaaa"},
				ClientConfig: confighttp.ClientConfig{
					Endpoint:             "endpoint",
					Compression:          "gzip",
					Auth:                 &auth,
					Headers:              map[string]configopaque.String{"key": "val"},
					HTTP2ReadIdleTimeout: 250,
					HTTP2PingTimeout:     200,
				},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 10 * time.Minute},
			},
			err: "these confighttp client configs are currently not respected by Datadog exporter: auth, endpoint, compression, headers, http2_read_idle_timeout, http2_ping_timeout",
		},
		{
			name: "Invalid reporter_period",
			cfg: &Config{
				API:          APIConfig{Key: "abcdef0"},
				HostMetadata: HostMetadataConfig{Enabled: true, ReporterPeriod: 4 * time.Minute},
			},
			err: "reporter_period must be 5 minutes or higher",
		},
	}
	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			err := testInstance.cfg.Validate()
			if testInstance.err != "" {
				assert.ErrorContains(t, err, testInstance.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	cfgWithHTTPConfigs := CreateDefaultConfig().(*Config)
	idleConnTimeout := 30 * time.Second
	maxIdleConn := 300
	maxIdleConnPerHost := 150
	maxConnPerHost := 250
	cfgWithHTTPConfigs.ReadBufferSize = 100
	cfgWithHTTPConfigs.WriteBufferSize = 200
	cfgWithHTTPConfigs.Timeout = 10 * time.Second
	cfgWithHTTPConfigs.MaxIdleConns = maxIdleConn
	cfgWithHTTPConfigs.MaxIdleConnsPerHost = maxIdleConnPerHost
	cfgWithHTTPConfigs.MaxConnsPerHost = maxConnPerHost
	cfgWithHTTPConfigs.IdleConnTimeout = idleConnTimeout
	cfgWithHTTPConfigs.DisableKeepAlives = true
	cfgWithHTTPConfigs.TLSSetting.InsecureSkipVerify = true
	cfgWithHTTPConfigs.warnings = nil

	tests := []struct {
		name      string
		configMap *confmap.Conf
		cfg       *Config
		err       string
		field     string
	}{
		{
			name: "invalid cumulative monotonic mode",
			configMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"sums": map[string]any{
						"cumulative_monotonic_mode": "invalid_mode",
					},
				},
			}),
			err:   "invalid cumulative monotonic sum mode \"invalid_mode\"",
			field: "metrics.sums.cumulative_monotonic_mode",
		},
		{
			name: "invalid host metadata hostname source",
			configMap: confmap.NewFromStringMap(map[string]any{
				"host_metadata": map[string]any{
					"hostname_source": "invalid_source",
				},
			}),
			err:   "invalid host metadata hostname source \"invalid_source\"",
			field: "host_metadata.hostname_source",
		},
		{
			name: "invalid summary mode",
			configMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"summaries": map[string]any{
						"mode": "invalid_mode",
					},
				},
			}),
			err:   "invalid summary mode \"invalid_mode\"",
			field: "metrics.summaries.mode",
		},
		{
			name: "metrics::send_monotonic_counter custom error",
			configMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"send_monotonic_counter": true,
				},
			}),
			err: "\"metrics::send_monotonic_counter\" was removed in favor of \"metrics::sums::cumulative_monotonic_mode\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8489",
		},
		{
			name: "tags custom error",
			configMap: confmap.NewFromStringMap(map[string]any{
				"tags": []string{},
			}),
			err: "\"tags\" was removed in favor of \"host_metadata::tags\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9099",
		},
		{
			name: "send_metadata custom error",
			configMap: confmap.NewFromStringMap(map[string]any{
				"send_metadata": false,
			}),
			err: "\"send_metadata\" was removed in favor of \"host_metadata::enabled\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9099",
		},
		{
			name: "use_resource_metadata custom error",
			configMap: confmap.NewFromStringMap(map[string]any{
				"use_resource_metadata": false,
			}),
			err: "\"use_resource_metadata\" was removed in favor of \"host_metadata::hostname_source\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9099",
		},
		{
			name: "metrics::report_quantiles custom error",
			configMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"report_quantiles": true,
				},
			}),
			err: "\"metrics::report_quantiles\" was removed in favor of \"metrics::summaries::mode\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8845",
		},
		{
			name: "instrumentation_library_metadata_as_tags custom error",
			configMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"instrumentation_library_metadata_as_tags": true,
				},
			}),
			err: "\"metrics::instrumentation_library_metadata_as_tags\" was removed in favor of \"metrics::instrumentation_scope_as_tags\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11135",
		},
		{
			name: "Empty metric endpoint",
			configMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"endpoint": "",
				},
			}),
			err: ErrEmptyEndpoint.Error(),
		},
		{
			name: "Empty trace endpoint",
			configMap: confmap.NewFromStringMap(map[string]any{
				"traces": map[string]any{
					"endpoint": "",
				},
			}),
			err: ErrEmptyEndpoint.Error(),
		},
		{
			name: "Empty log endpoint",
			configMap: confmap.NewFromStringMap(map[string]any{
				"logs": map[string]any{
					"endpoint": "",
				},
			}),
			err: ErrEmptyEndpoint.Error(),
		},
		{
			name: "invalid initial cumulative monotonic value mode",
			configMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"sums": map[string]any{
						"initial_cumulative_monotonic_value": "invalid_mode",
					},
				},
			}),
			err:   "invalid initial value mode \"invalid_mode\"",
			field: "metrics.sums.initial_cumulative_monotonic_value",
		},
		{
			name: "initial cumulative monotonic value mode set with raw_value",
			configMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"sums": map[string]any{
						"cumulative_monotonic_mode":          "raw_value",
						"initial_cumulative_monotonic_value": "drop",
					},
				},
			}),
			err: "\"metrics::sums::initial_cumulative_monotonic_value\" can only be configured when \"metrics::sums::cumulative_monotonic_mode\" is set to \"to_delta\"",
		},
		{
			name: "unmarshall confighttp client configs",
			configMap: confmap.NewFromStringMap(map[string]any{
				"read_buffer_size":        100,
				"write_buffer_size":       200,
				"timeout":                 "10s",
				"max_idle_conns":          300,
				"max_idle_conns_per_host": 150,
				"max_conns_per_host":      250,
				"disable_keep_alives":     true,
				"idle_conn_timeout":       "30s",
				"tls":                     map[string]any{"insecure_skip_verify": true},
			}),
			cfg: cfgWithHTTPConfigs,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			cfg := CreateDefaultConfig().(*Config)
			err := cfg.Unmarshal(testInstance.configMap)
			if err != nil || testInstance.err != "" {
				assert.ErrorContains(t, err, testInstance.err)
				if testInstance.field != "" {
					assert.ErrorContains(t, err, testInstance.field)
				}
			} else {
				assert.Equal(t, testInstance.cfg, cfg)
			}
		})
	}
}

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	cfg := CreateDefaultConfig()

	assert.Equal(t, &Config{
		ClientConfig:  defaultClientConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),

		API: APIConfig{
			Site: "datadoghq.com",
		},

		Metrics: MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://api.datadoghq.com",
			},
			DeltaTTL: 3600,
			HistConfig: HistogramConfig{
				Mode:             "distributions",
				SendAggregations: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
				InitialCumulativeMonotonicMode: InitialValueModeAuto,
			},
			SummaryConfig: SummaryConfig{
				Mode: SummaryModeGauges,
			},
		},

		Traces: TracesExporterConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			TracesConfig: TracesConfig{
				IgnoreResources:        []string{},
				PeerServiceAggregation: true,
				PeerTagsAggregation:    true,
				ComputeStatsBySpanKind: true,
			},
		},
		Logs: LogsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://http-intake.logs.datadoghq.com",
			},
			UseCompression:   true,
			CompressionLevel: 6,
			BatchWait:        5,
		},

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceConfigOrSystem,
			ReporterPeriod: 30 * time.Minute,
		},
		OnlyMetadata: false,
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

var ddtype = component.MustNewType("datadog")

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(ddtype, "default"),
			expected: &Config{
				ClientConfig:  defaultClientConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueConfig(),
				API: APIConfig{
					Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Site:             "datadoghq.com",
					FailOnInvalidKey: false,
				},

				Metrics: MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.com",
					},
					DeltaTTL: 3600,
					HistConfig: HistogramConfig{
						Mode:             "distributions",
						SendAggregations: false,
					},
					SumConfig: SumConfig{
						CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
						InitialCumulativeMonotonicMode: InitialValueModeAuto,
					},
					SummaryConfig: SummaryConfig{
						Mode: SummaryModeGauges,
					},
				},

				Traces: TracesExporterConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.com",
					},
					TracesConfig: TracesConfig{
						IgnoreResources:        []string{},
						PeerServiceAggregation: true,
						PeerTagsAggregation:    true,
						ComputeStatsBySpanKind: true,
					},
				},
				Logs: LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.datadoghq.com",
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        5,
				},
				HostMetadata: HostMetadataConfig{
					Enabled:        true,
					HostnameSource: HostnameSourceConfigOrSystem,
					ReporterPeriod: 30 * time.Minute,
				},
				OnlyMetadata: false,
			},
		},
		{
			id: component.NewIDWithName(ddtype, "api"),
			expected: &Config{
				ClientConfig:  defaultClientConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueConfig(),
				TagsConfig: TagsConfig{
					Hostname: "customhostname",
				},
				API: APIConfig{
					Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Site:             "datadoghq.eu",
					FailOnInvalidKey: true,
				},
				Metrics: MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.eu",
					},
					DeltaTTL: 3600,
					HistConfig: HistogramConfig{
						Mode:             "distributions",
						SendAggregations: false,
					},
					SumConfig: SumConfig{
						CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
						InitialCumulativeMonotonicMode: InitialValueModeAuto,
					},
					SummaryConfig: SummaryConfig{
						Mode: SummaryModeGauges,
					},
				},
				Traces: TracesExporterConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.eu",
					},
					TracesConfig: TracesConfig{
						SpanNameRemappings: map[string]string{
							"old_name1": "new_name1",
							"old_name2": "new_name2",
						},
						SpanNameAsResourceName: true,
						IgnoreResources:        []string{},
						PeerServiceAggregation: true,
						PeerTagsAggregation:    true,
						ComputeStatsBySpanKind: true,
					},
					TraceBuffer: 10,
				},
				Logs: LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.datadoghq.eu",
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        5,
				},
				OnlyMetadata: false,
				HostMetadata: HostMetadataConfig{
					Enabled:        true,
					HostnameSource: HostnameSourceConfigOrSystem,
					ReporterPeriod: 30 * time.Minute,
				},
			},
		},
		{
			id: component.NewIDWithName(ddtype, "api2"),
			expected: &Config{
				ClientConfig:  defaultClientConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueConfig(),
				TagsConfig: TagsConfig{
					Hostname: "customhostname",
				},
				API: APIConfig{
					Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Site:             "datadoghq.eu",
					FailOnInvalidKey: false,
				},
				Metrics: MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.test",
					},
					DeltaTTL: 3600,
					HistConfig: HistogramConfig{
						Mode:             "distributions",
						SendAggregations: false,
					},
					SumConfig: SumConfig{
						CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
						InitialCumulativeMonotonicMode: InitialValueModeAuto,
					},
					SummaryConfig: SummaryConfig{
						Mode: SummaryModeGauges,
					},
				},
				Traces: TracesExporterConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.test",
					},
					TracesConfig: TracesConfig{
						SpanNameRemappings: map[string]string{
							"old_name3": "new_name3",
							"old_name4": "new_name4",
						},
						IgnoreResources:        []string{},
						PeerServiceAggregation: true,
						PeerTagsAggregation:    true,
						ComputeStatsBySpanKind: true,
					},
				},
				Logs: LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.datadoghq.test",
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        5,
				},
				HostMetadata: HostMetadataConfig{
					Enabled:        true,
					HostnameSource: HostnameSourceConfigOrSystem,
					Tags:           []string{"example:tag"},
					ReporterPeriod: 30 * time.Minute,
				},
			},
		},
		{
			id: component.NewIDWithName(ddtype, "customReporterPeriod"),
			expected: &Config{
				ClientConfig:  defaultClientConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueConfig(),
				API: APIConfig{
					Key:              "abc",
					Site:             "datadoghq.com",
					FailOnInvalidKey: false,
				},

				Metrics: MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.com",
					},
					DeltaTTL: 3600,
					HistConfig: HistogramConfig{
						Mode:             "distributions",
						SendAggregations: false,
					},
					SumConfig: SumConfig{
						CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
						InitialCumulativeMonotonicMode: InitialValueModeAuto,
					},
					SummaryConfig: SummaryConfig{
						Mode: SummaryModeGauges,
					},
				},

				Traces: TracesExporterConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.com",
					},
					TracesConfig: TracesConfig{
						IgnoreResources:        []string{},
						ComputeStatsBySpanKind: true,
						PeerServiceAggregation: true,
						PeerTagsAggregation:    true,
					},
				},
				Logs: LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.datadoghq.com",
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        5,
				},
				HostMetadata: HostMetadataConfig{
					Enabled:        true,
					HostnameSource: HostnameSourceConfigOrSystem,
					ReporterPeriod: 10 * time.Minute,
				},
				OnlyMetadata: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestOverrideEndpoints(t *testing.T) {
	tests := []struct {
		componentID             string
		expectedSite            string
		expectedMetricsEndpoint string
		expectedTracesEndpoint  string
		expectedLogsEndpoint    string
	}{
		{
			componentID:             "nositeandnoendpoints",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "https://api.datadoghq.com",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.com",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.com",
		},
		{
			componentID:             "nositeandmetricsendpoint",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.com",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.com",
		},
		{
			componentID:             "nositeandtracesendpoint",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "https://api.datadoghq.com",
			expectedTracesEndpoint:  "tracesendpoint:1234",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.com",
		},
		{
			componentID:             "nositeandlogsendpoint",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "https://api.datadoghq.com",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.com",
			expectedLogsEndpoint:    "logsendpoint:1234",
		},
		{
			componentID:             "nositeandallendpoints",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "tracesendpoint:1234",
			expectedLogsEndpoint:    "logsendpoint:1234",
		},

		{
			componentID:             "siteandnoendpoints",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "https://api.datadoghq.eu",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.eu",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.eu",
		},
		{
			componentID:             "siteandmetricsendpoint",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.eu",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.eu",
		},
		{
			componentID:             "siteandtracesendpoint",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "https://api.datadoghq.eu",
			expectedTracesEndpoint:  "tracesendpoint:1234",
			expectedLogsEndpoint:    "https://http-intake.logs.datadoghq.eu",
		},
		{
			componentID:             "siteandallendpoints",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "tracesendpoint:1234",
			expectedLogsEndpoint:    "logsendpoint:1234",
		},
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "unmarshal.yaml"))
	require.NoError(t, err)

	for _, testInstance := range tests {
		t.Run(testInstance.componentID, func(t *testing.T) {
			cfg := CreateDefaultConfig()
			sub, err := cm.Sub(component.NewIDWithName(ddtype, testInstance.componentID).String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			componentCfg, ok := cfg.(*Config)
			require.True(t, ok, "component.Config is not a Datadog exporter config (wrong ID?)")
			assert.Equal(t, testInstance.expectedSite, componentCfg.API.Site)
			assert.Equal(t, testInstance.expectedMetricsEndpoint, componentCfg.Metrics.Endpoint)
			assert.Equal(t, testInstance.expectedTracesEndpoint, componentCfg.Traces.Endpoint)
			assert.Equal(t, testInstance.expectedLogsEndpoint, componentCfg.Logs.Endpoint)
		})
	}
}
