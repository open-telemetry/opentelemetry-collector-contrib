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

package sumologicexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfigValidation(t *testing.T) {
	testcases := []struct {
		name        string
		cfg         *Config
		expectedErr string
	}{
		{
			name: "invalid log format",
			cfg: &Config{
				LogFormat:        "test_format",
				MetricFormat:     "carbon2",
				CompressEncoding: "gzip",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout:  defaultTimeout,
					Endpoint: "test_endpoint",
				},
			},
			expectedErr: "unexpected log format: test_format",
		},
		{
			name: "invalid metric format",
			cfg: &Config{
				LogFormat:    "json",
				MetricFormat: "test_format",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout:  defaultTimeout,
					Endpoint: "test_endpoint",
				},
				CompressEncoding: "gzip",
			},
			expectedErr: "unexpected metric format: test_format",
		},
		{
			name: "invalid compress encoding",
			cfg: &Config{
				LogFormat:        "json",
				MetricFormat:     "carbon2",
				CompressEncoding: "test_format",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout:  defaultTimeout,
					Endpoint: "test_endpoint",
				},
			},
			expectedErr: "unexpected compression encoding: test_format",
		},
		{
			name: "invalid endpoint",
			cfg: &Config{
				LogFormat:        "json",
				MetricFormat:     "carbon2",
				CompressEncoding: "gzip",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout: defaultTimeout,
				},
			},
			expectedErr: "endpoint is not set",
		},
		{
			name: "invalid log format",
			cfg: &Config{
				LogFormat:        "json",
				MetricFormat:     "carbon2",
				CompressEncoding: "gzip",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout:  defaultTimeout,
					Endpoint: "test_endpoint",
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:   true,
					QueueSize: -10,
				},
			},
			expectedErr: "queue settings has invalid configuration: queue size must be positive",
		},
		{
			name: "valid config",
			cfg: &Config{
				LogFormat:        "json",
				MetricFormat:     "carbon2",
				CompressEncoding: "gzip",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout:  defaultTimeout,
					Endpoint: "test_endpoint",
				},
			},
			expectedErr: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			err := tc.cfg.Validate()

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				require.NotNil(t, err)
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
