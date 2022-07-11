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

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestHostTagsTagsConfig(t *testing.T) {
	tc := TagsConfig{
		Hostname: "customhost",
		Env:      "customenv",
		// Service and version should be only used for traces
		Service: "customservice",
		Version: "customversion",
		Tags:    []string{"key1:val1", "key2:val2"},
	}

	assert.ElementsMatch(t,
		[]string{
			"env:customenv",
			"key1:val1",
			"key2:val2",
		},
		tc.getHostTags(),
	)

	tc = TagsConfig{
		Hostname: "customhost",
		Env:      "customenv",
		// Service and version should be only used for traces
		Service:    "customservice",
		Version:    "customversion",
		Tags:       []string{"key1:val1", "key2:val2"},
		EnvVarTags: "key3:val3 key4:val4",
	}

	assert.ElementsMatch(t,
		[]string{
			"env:customenv",
			"key1:val1",
			"key2:val2",
		},
		tc.getHostTags(),
	)

	tc = TagsConfig{
		Hostname: "customhost",
		Env:      "customenv",
		// Service and version should be only used for traces
		Service:    "customservice",
		Version:    "customversion",
		EnvVarTags: "key3:val3 key4:val4",
	}

	assert.ElementsMatch(t,
		[]string{
			"env:customenv",
			"key3:val3",
			"key4:val4",
		},
		tc.getHostTags(),
	)
}

func TestValidate(t *testing.T) {

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
				SendMetadata: false,
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
						Mode:         HistogramModeNoBuckets,
						SendCountSum: false,
					},
				},
			},
			err: "'nobuckets' mode and `send_count_sum_metrics` set to false will send no histogram metrics",
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
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			cfg := futureDefaultConfig()
			err := cfg.Unmarshal(testInstance.configMap)
			if err != nil || testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.cfg, cfg)
			}
		})
	}
}
