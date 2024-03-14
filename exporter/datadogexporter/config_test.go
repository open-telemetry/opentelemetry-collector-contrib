// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
)

func TestValidate(t *testing.T) {
	idleConnTimeout := 30 * time.Second
	maxIdleConn := 300
	maxIdleConnPerHost := 150
	maxConnPerHost := 250
	ty, err := component.NewType("ty")
	assert.NoError(t, err)
	auth := configauth.Authentication{AuthenticatorID: component.NewID(ty)}

	tests := []struct {
		name string
		cfg  *Config
		err  string
	}{
		{
			name: "no api::key",
			cfg:  &Config{},
			err:  errUnsetAPIKey.Error(),
		},
		{
			name: "invalid hostname",
			cfg: &Config{
				API:        APIConfig{Key: "notnull"},
				TagsConfig: TagsConfig{Hostname: "invalid_host"},
			},
			err: "hostname field is invalid: 'invalid_host' is not RFC1123 compliant",
		},
		{
			name: "no metadata",
			cfg: &Config{
				API:          APIConfig{Key: "notnull"},
				OnlyMetadata: true,
				HostMetadata: HostMetadataConfig{Enabled: false},
			},
			err: errNoMetadata.Error(),
		},
		{
			name: "span name remapping valid",
			cfg: &Config{
				API:    APIConfig{Key: "notnull"},
				Traces: TracesConfig{SpanNameRemappings: map[string]string{"old.opentelemetryspan.name": "updated.name"}},
			},
		},
		{
			name: "span name remapping empty val",
			cfg: &Config{
				API:    APIConfig{Key: "notnull"},
				Traces: TracesConfig{SpanNameRemappings: map[string]string{"oldname": ""}},
			},
			err: "'' is not valid value for span name remapping",
		},
		{
			name: "span name remapping empty key",
			cfg: &Config{
				API:    APIConfig{Key: "notnull"},
				Traces: TracesConfig{SpanNameRemappings: map[string]string{"": "newname"}},
			},
			err: "'' is not valid key for span name remapping",
		},
		{
			name: "ignore resources valid",
			cfg: &Config{
				API:    APIConfig{Key: "notnull"},
				Traces: TracesConfig{IgnoreResources: []string{"[123]"}},
			},
		},
		{
			name: "ignore resources missing bracket",
			cfg: &Config{
				API:    APIConfig{Key: "notnull"},
				Traces: TracesConfig{IgnoreResources: []string{"[123"}},
			},
			err: "'[123' is not valid resource filter regular expression",
		},
		{
			name: "invalid histogram settings",
			cfg: &Config{
				API: APIConfig{Key: "notnull"},
				Metrics: MetricsConfig{
					HistConfig: HistogramConfig{
						Mode:             HistogramModeNoBuckets,
						SendAggregations: false,
					},
				},
			},
			err: "'nobuckets' mode and `send_aggregation_metrics` set to false will send no histogram metrics",
		},
		{
			name: "TLS settings are valid",
			cfg: &Config{
				API: APIConfig{Key: "notnull"},
				ClientConfig: confighttp.ClientConfig{
					TLSSetting: configtls.ClientConfig{
						InsecureSkipVerify: true,
					},
				},
			},
		},
		{
			name: "With trace_buffer",
			cfg: &Config{
				API:    APIConfig{Key: "notnull"},
				Traces: TracesConfig{TraceBuffer: 10},
			},
		},
		{
			name: "With peer_tags",
			cfg: &Config{
				API:    APIConfig{Key: "notnull"},
				Traces: TracesConfig{PeerTags: []string{"tag1", "tag2"}},
			},
		},
		{
			name: "With confighttp client configs",
			cfg: &Config{
				API: APIConfig{Key: "notnull"},
				ClientConfig: confighttp.ClientConfig{
					ReadBufferSize:      100,
					WriteBufferSize:     200,
					Timeout:             10 * time.Second,
					IdleConnTimeout:     &idleConnTimeout,
					MaxIdleConns:        &maxIdleConn,
					MaxIdleConnsPerHost: &maxIdleConnPerHost,
					MaxConnsPerHost:     &maxConnPerHost,
					DisableKeepAlives:   true,
					TLSSetting:          configtls.ClientConfig{InsecureSkipVerify: true},
				},
			},
		},

		{
			name: "unsupported confighttp client configs",
			cfg: &Config{
				API: APIConfig{Key: "notnull"},
				ClientConfig: confighttp.ClientConfig{
					Endpoint:             "endpoint",
					Compression:          "gzip",
					ProxyURL:             "proxy",
					Auth:                 &auth,
					Headers:              map[string]configopaque.String{"key": "val"},
					HTTP2ReadIdleTimeout: 250,
					HTTP2PingTimeout:     200,
				},
			},
			err: "these confighttp client configs are currently not respected by Datadog exporter: auth, endpoint, compression, proxy_url, headers, http2_read_idle_timeout, http2_ping_timeout",
		},
	}
	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			err := testInstance.cfg.Validate()
			if testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	cfgWithHTTPConfigs := NewFactory().CreateDefaultConfig().(*Config)
	idleConnTimeout := 30 * time.Second
	maxIdleConn := 300
	maxIdleConnPerHost := 150
	maxConnPerHost := 250
	cfgWithHTTPConfigs.ReadBufferSize = 100
	cfgWithHTTPConfigs.WriteBufferSize = 200
	cfgWithHTTPConfigs.Timeout = 10 * time.Second
	cfgWithHTTPConfigs.MaxIdleConns = &maxIdleConn
	cfgWithHTTPConfigs.MaxIdleConnsPerHost = &maxIdleConnPerHost
	cfgWithHTTPConfigs.MaxConnsPerHost = &maxConnPerHost
	cfgWithHTTPConfigs.IdleConnTimeout = &idleConnTimeout
	cfgWithHTTPConfigs.DisableKeepAlives = true
	cfgWithHTTPConfigs.TLSSetting.InsecureSkipVerify = true
	cfgWithHTTPConfigs.warnings = nil

	tests := []struct {
		name      string
		configMap *confmap.Conf
		cfg       *Config
		err       string
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
			err: "1 error(s) decoding:\n\n* error decoding 'metrics.sums.cumulative_monotonic_mode': invalid cumulative monotonic sum mode \"invalid_mode\"",
		},
		{
			name: "invalid host metadata hostname source",
			configMap: confmap.NewFromStringMap(map[string]any{
				"host_metadata": map[string]any{
					"hostname_source": "invalid_source",
				},
			}),
			err: "1 error(s) decoding:\n\n* error decoding 'host_metadata.hostname_source': invalid host metadata hostname source \"invalid_source\"",
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
			err: "1 error(s) decoding:\n\n* error decoding 'metrics.summaries.mode': invalid summary mode \"invalid_mode\"",
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
			err: errEmptyEndpoint.Error(),
		},
		{
			name: "Empty trace endpoint",
			configMap: confmap.NewFromStringMap(map[string]any{
				"traces": map[string]any{
					"endpoint": "",
				},
			}),
			err: errEmptyEndpoint.Error(),
		},
		{
			name: "Empty log endpoint",
			configMap: confmap.NewFromStringMap(map[string]any{
				"logs": map[string]any{
					"endpoint": "",
				},
			}),
			err: errEmptyEndpoint.Error(),
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
			err: "1 error(s) decoding:\n\n* error decoding 'metrics.sums.initial_cumulative_monotonic_value': invalid initial value mode \"invalid_mode\"",
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

	f := NewFactory()
	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			cfg := f.CreateDefaultConfig().(*Config)
			err := cfg.Unmarshal(testInstance.configMap)
			if err != nil || testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.cfg, cfg)
			}
		})
	}
}
