// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		err  string
	}{
		{
			name: "span name remapping valid",
			cfg: &Config{
				Traces: datadogconfig.TracesConnectorConfig{
					TracesConfig: datadogconfig.TracesConfig{
						SpanNameRemappings: map[string]string{"old.opentelemetryspan.name": "updated.name"},
					},
				},
			},
		},
		{
			name: "span name remapping empty val",
			cfg: &Config{Traces: datadogconfig.TracesConnectorConfig{
				TracesConfig: datadogconfig.TracesConfig{
					SpanNameRemappings: map[string]string{"oldname": ""},
				},
			}},
			err: "'' is not valid value for span name remapping",
		},
		{
			name: "span name remapping empty key",
			cfg: &Config{Traces: datadogconfig.TracesConnectorConfig{
				TracesConfig: datadogconfig.TracesConfig{
					SpanNameRemappings: map[string]string{"": "newname"},
				},
			}},
			err: "'' is not valid key for span name remapping",
		},
		{
			name: "ignore resources valid",
			cfg: &Config{Traces: datadogconfig.TracesConnectorConfig{
				TracesConfig: datadogconfig.TracesConfig{
					IgnoreResources: []string{"[123]"},
				},
			}},
		},
		{
			name: "ignore resources missing bracket",
			cfg: &Config{Traces: datadogconfig.TracesConnectorConfig{
				TracesConfig: datadogconfig.TracesConfig{
					IgnoreResources: []string{"[123"},
				},
			}},
			err: "'[123' is not valid resource filter regular expression",
		},
		{
			name: "With trace_buffer",
			cfg: &Config{Traces: datadogconfig.TracesConnectorConfig{
				TraceBuffer: 10,
			}},
		},
		{
			name: "neg trace_buffer",
			cfg: &Config{Traces: datadogconfig.TracesConnectorConfig{
				TraceBuffer: -10,
			}},
			err: "trace buffer must be non-negative",
		},
		{
			name: "With peer_tags",
			cfg: &Config{
				Traces: datadogconfig.TracesConnectorConfig{
					TracesConfig: datadogconfig.TracesConfig{
						PeerTags: []string{"tag1", "tag2"},
					},
				},
			},
		},
		{
			name: "With bucket_interval",
			cfg: &Config{
				Traces: datadogconfig.TracesConnectorConfig{BucketInterval: 30 * time.Second},
			},
		},
		{
			name: "neg bucket_interval",
			cfg: &Config{
				Traces: datadogconfig.TracesConnectorConfig{BucketInterval: -30 * time.Second},
			},
			err: "bucket interval must be non-negative",
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
