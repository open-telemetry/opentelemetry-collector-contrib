// Copyright 2023, OpenTelemetry Authors
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

package opensearchexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).Endpoints = []string{"https://opensearch.example.com:9200"}

	tests := []struct {
		id                   component.ID
		expected             component.Config
		configValidateAssert assert.ErrorAssertionFunc
	}{
		{
			id:                   component.NewIDWithName(typeStr, ""),
			expected:             defaultCfg,
			configValidateAssert: assert.NoError,
		},
		{
			id: component.NewIDWithName(typeStr, "trace"),
			expected: &Config{
				Endpoints:   []string{"https://opensearch.example.com:9200"},
				LogsIndex:   "logs-generic-default",
				TracesIndex: "trace_index",
				Pipeline:    "mypipeline",
				HTTPClientSettings: HTTPClientSettings{
					Authentication: AuthenticationSettings{
						User:     "open",
						Password: "search",
					},
					Timeout: 2 * time.Minute,
					Headers: map[string]string{
						"myheader": "test",
					},
				},
				Flush: FlushSettings{
					Bytes: 10485760,
				},
				Retry: RetrySettings{
					Enabled:         true,
					MaxRequests:     5,
					InitialInterval: 100 * time.Millisecond,
					MaxInterval:     1 * time.Minute,
				},
				Mapping: MappingsSettings{
					Mode:  "sso",
					Dedup: true,
					Dedot: true,
				},
			},
			configValidateAssert: assert.NoError,
		},
		{
			id: component.NewIDWithName(typeStr, "log"),
			expected: &Config{
				Endpoints:   []string{"http://localhost:9200"},
				LogsIndex:   "my_log_index",
				TracesIndex: "traces-generic-default",
				Pipeline:    "mypipeline",
				HTTPClientSettings: HTTPClientSettings{
					Authentication: AuthenticationSettings{
						User:     "open",
						Password: "search",
					},
					Timeout: 2 * time.Minute,
					Headers: map[string]string{
						"myheader": "test",
					},
				},
				Flush: FlushSettings{
					Bytes: 10485760,
				},
				Retry: RetrySettings{
					Enabled:         true,
					MaxRequests:     5,
					InitialInterval: 100 * time.Millisecond,
					MaxInterval:     1 * time.Minute,
				},
				Mapping: MappingsSettings{
					Mode:  "sso",
					Dedup: true,
					Dedot: true,
				},
			},
			configValidateAssert: assert.NoError,
		},
		{
			id: component.NewIDWithName(typeStr, "bad_mode"),
			expected: &Config{
				Endpoints:   []string{"http://localhost:9200"},
				LogsIndex:   "my_log_index",
				TracesIndex: "traces-generic-default",
				Pipeline:    "mypipeline",
				HTTPClientSettings: HTTPClientSettings{
					Authentication: AuthenticationSettings{
						User:     "open",
						Password: "search",
					},
					Timeout: 2 * time.Minute,
					Headers: map[string]string{
						"myheader": "test",
					},
				},
				Flush: FlushSettings{
					Bytes: 10485760,
				},
				Retry: RetrySettings{
					Enabled:         true,
					MaxRequests:     5,
					InitialInterval: 100 * time.Millisecond,
					MaxInterval:     1 * time.Minute,
				},
				Mapping: MappingsSettings{
					Mode:  "rando",
					Dedup: true,
					Dedot: true,
				},
			},
			configValidateAssert: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			vv := component.ValidateConfig(cfg)
			tt.configValidateAssert(t, vv)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}
