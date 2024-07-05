// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestInitExporterInvalidConfiguration(t *testing.T) {
	testcases := []struct {
		name          string
		cfg           *Config
		expectedError error
	}{
		{
			name:          "unexpected log format",
			expectedError: errors.New("unexpected log format: test_format"),
			cfg: &Config{
				LogFormat:    "test_format",
				MetricFormat: "otlp",
				ClientConfig: confighttp.ClientConfig{
					Timeout:  defaultTimeout,
					Endpoint: "test_endpoint",
				},
			},
		},
		{
			name:          "unexpected metric format",
			expectedError: errors.New("unexpected metric format: test_format"),
			cfg: &Config{
				LogFormat:    "json",
				MetricFormat: "test_format",
				ClientConfig: confighttp.ClientConfig{
					Timeout:     defaultTimeout,
					Endpoint:    "test_endpoint",
					Compression: "gzip",
				},
			},
		},
		{
			name:          "unsupported Carbon2 metrics format",
			expectedError: errors.New("support for the carbon2 metric format was removed, please use prometheus or otlp instead"),
			cfg: &Config{
				LogFormat:    "json",
				MetricFormat: "carbon2",
				ClientConfig: confighttp.ClientConfig{
					Timeout:     defaultTimeout,
					Endpoint:    "test_endpoint",
					Compression: "gzip",
				},
			},
		},
		{
			name:          "unsupported Graphite metrics format",
			expectedError: errors.New("support for the graphite metric format was removed, please use prometheus or otlp instead"),
			cfg: &Config{
				LogFormat:    "json",
				MetricFormat: "graphite",
				ClientConfig: confighttp.ClientConfig{
					Timeout:     defaultTimeout,
					Endpoint:    "test_endpoint",
					Compression: "gzip",
				},
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := component.ValidateConfig(tc.cfg)

			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigInvalidTimeout(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError error
		cfg           *Config
	}{
		{
			name:          "over the limit timeout",
			expectedError: errors.New("timeout must be between 1 and 55 seconds, got 56s"),
			cfg: &Config{
				ClientConfig: confighttp.ClientConfig{
					Timeout: 56 * time.Second,
				},
			},
		},
		{
			name:          "less than 1 timeout",
			expectedError: errors.New("timeout must be between 1 and 55 seconds, got 0s"),
			cfg: &Config{
				ClientConfig: confighttp.ClientConfig{
					Timeout: 0 * time.Second,
				},
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()

			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
