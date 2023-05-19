// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
