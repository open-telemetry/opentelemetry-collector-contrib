// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datadogexporter

import (
	"testing"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestValidate(t *testing.T) {
	override.IMDSRetryer = nil
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
				LimitedHTTPClientSettings: LimitedHTTPClientSettings{
					TLSSetting: LimitedTLSClientSettings{
						InsecureSkipVerify: true,
					},
				},
			},
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
	tests := []struct {
		name      string
		configMap *confmap.Conf
		cfg       Config
		err       string
	}{
		{
			name: "invalid cumulative monotonic mode",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"sums": map[string]interface{}{
						"cumulative_monotonic_mode": "invalid_mode",
					},
				},
			}),
			err: "1 error(s) decoding:\n\n* error decoding 'metrics.sums.cumulative_monotonic_mode': invalid cumulative monotonic sum mode \"invalid_mode\"",
		},
		{
			name: "invalid host metadata hostname source",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"host_metadata": map[string]interface{}{
					"hostname_source": "invalid_source",
				},
			}),
			err: "1 error(s) decoding:\n\n* error decoding 'host_metadata.hostname_source': invalid host metadata hostname source \"invalid_source\"",
		},
		{
			name: "invalid summary mode",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"summaries": map[string]interface{}{
						"mode": "invalid_mode",
					},
				},
			}),
			err: "1 error(s) decoding:\n\n* error decoding 'metrics.summaries.mode': invalid summary mode \"invalid_mode\"",
		},
		{
			name: "metrics::send_monotonic_counter custom error",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"send_monotonic_counter": true,
				},
			}),
			err: "\"metrics::send_monotonic_counter\" was removed in favor of \"metrics::sums::cumulative_monotonic_mode\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8489",
		},
		{
			name: "tags custom error",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"tags": []string{},
			}),
			err: "\"tags\" was removed in favor of \"host_metadata::tags\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9099",
		},
		{
			name: "send_metadata custom error",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"send_metadata": false,
			}),
			err: "\"send_metadata\" was removed in favor of \"host_metadata::enabled\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9099",
		},
		{
			name: "use_resource_metadata custom error",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"use_resource_metadata": false,
			}),
			err: "\"use_resource_metadata\" was removed in favor of \"host_metadata::hostname_source\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9099",
		},
		{
			name: "metrics::report_quantiles custom error",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"report_quantiles": true,
				},
			}),
			err: "\"metrics::report_quantiles\" was removed in favor of \"metrics::summaries::mode\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8845",
		},
		{
			name: "instrumentation_library_metadata_as_tags custom error",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"instrumentation_library_metadata_as_tags": true,
				},
			}),
			err: "\"metrics::instrumentation_library_metadata_as_tags\" was removed in favor of \"metrics::instrumentation_scope_as_tags\". See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11135",
		},
		{
			name: "Empty metric endpoint",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"endpoint": "",
				},
			}),
			err: errEmptyEndpoint.Error(),
		},
		{
			name: "Empty trace endpoint",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"traces": map[string]interface{}{
					"endpoint": "",
				},
			}),
			err: errEmptyEndpoint.Error(),
		},
		{
			name: "Empty log endpoint",
			configMap: confmap.NewFromStringMap(map[string]interface{}{
				"logs": map[string]interface{}{
					"endpoint": "",
				},
			}),
			err: errEmptyEndpoint.Error(),
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
