// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestUnmarshal(t *testing.T) {
	cfgWithHTTPConfigs := NewFactory().CreateDefaultConfig().(*Config)
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
			err: datadogconfig.ErrEmptyEndpoint.Error(),
		},
		{
			name: "Empty trace endpoint",
			configMap: confmap.NewFromStringMap(map[string]any{
				"traces": map[string]any{
					"endpoint": "",
				},
			}),
			err: datadogconfig.ErrEmptyEndpoint.Error(),
		},
		{
			name: "Empty log endpoint",
			configMap: confmap.NewFromStringMap(map[string]any{
				"logs": map[string]any{
					"endpoint": "",
				},
			}),
			err: datadogconfig.ErrEmptyEndpoint.Error(),
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

	f := NewFactory()
	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			cfg := f.CreateDefaultConfig().(*Config)
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
