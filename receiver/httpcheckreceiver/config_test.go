// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
)

func TestConfig_Validate(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing endpoint",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: confighttp.ClientConfig{},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				errMissingEndpoint,
			),
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: confighttp.ClientConfig{
							Endpoint: "invalid://endpoint:  12efg",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "invalid config with multiple targets",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: confighttp.ClientConfig{
							Endpoint: "https://localhost:80",
						},
					},
					{
						ClientConfig: confighttp.ClientConfig{
							Endpoint: "invalid://endpoint:  12efg",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "missing scheme",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: confighttp.ClientConfig{
							Endpoint: "www.opentelemetry.io/docs",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "www.opentelemetry.io/docs": invalid URI for request`),
			),
		},
		{
			desc: "valid config",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: confighttp.ClientConfig{
							Endpoint: "https://opentelemetry.io",
						},
					},
					{
						ClientConfig: confighttp.ClientConfig{
							Endpoint: "https://opentelemetry.io:80/docs",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.EqualError(t, actualErr, tc.expectedErr.Error())
			} else {
				require.NoError(t, actualErr)
			}

		})
	}
}

func TestSequenceConfig_Validate(t *testing.T) {
	testCases := []struct {
		name        string
		sequenceCfg *sequenceConfig
		expectedErr error
	}{
		{
			name: "Valid sequence name and one step",
			sequenceCfg: &sequenceConfig{
				Name: "valid-sequence",
				Steps: []*sequenceStep{
					{
						ClientConfig: confighttp.ClientConfig{Endpoint: "http://example.com"},
						Method:       "GET",
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "Missing sequence name",
			sequenceCfg: &sequenceConfig{
				Steps: []*sequenceStep{
					{
						ClientConfig: confighttp.ClientConfig{Endpoint: "http://example.com"},
						Method:       "GET",
					},
				},
			},
			expectedErr: errors.New("sequence name must be specified"),
		},
		{
			name: "No steps",
			sequenceCfg: &sequenceConfig{
				Name:  "test-sequence",
				Steps: []*sequenceStep{},
			},
			expectedErr: errors.New("sequence must have at least one step"),
		},
		{
			name: "Invalid endpoint format",
			sequenceCfg: &sequenceConfig{
				Name: "test_sequence",
				Steps: []*sequenceStep{
					{
						ClientConfig: confighttp.ClientConfig{
							Endpoint: "invalid.endpoint",
						},
						Method: "GET",
					},
				},
			},
			expectedErr: fmt.Errorf("invalid endpoint in step 1 of sequence test_sequence: parse \"invalid.endpoint\": invalid URI for request"),
		},
		{
			name: "Missing method",
			sequenceCfg: &sequenceConfig{
				Name: "test_sequence",
				Steps: []*sequenceStep{
					{
						ClientConfig: confighttp.ClientConfig{Endpoint: "https://example.com"},
					},
				},
			},
			expectedErr: fmt.Errorf("missing method in step 1 of sequence test_sequence"),
		},
		{
			name: "Mixed valid and invalid steps",
			sequenceCfg: &sequenceConfig{
				Name: "mixed_sequence",
				Steps: []*sequenceStep{
					{
						ClientConfig: confighttp.ClientConfig{Endpoint: "https://valid-endpoint.com"},
						Method:       "GET",
					},
					{
						ClientConfig: confighttp.ClientConfig{},
						Method:       "POST",
					},
					{
						ClientConfig: confighttp.ClientConfig{Endpoint: "invalid.endpoint"},
						Method:       "PUT",
					},
				},
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("missing endpoint in step 2 of sequence mixed_sequence"),
				fmt.Errorf("invalid endpoint in step 3 of sequence mixed_sequence: parse \"invalid.endpoint\": invalid URI for request"),
			),
		},
		{
			name: "Multiple errors in single step",
			sequenceCfg: &sequenceConfig{
				Name: "test-sequence",
				Steps: []*sequenceStep{
					{
						ClientConfig: confighttp.ClientConfig{},
					},
				},
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("missing endpoint in step 1 of sequence test-sequence"),
				fmt.Errorf("missing method in step 1 of sequence test-sequence"),
			),
		},
		{
			name: "Valid sequence with multiple steps",
			sequenceCfg: &sequenceConfig{
				Name: "test-sequence",
				Steps: []*sequenceStep{
					{
						ClientConfig: confighttp.ClientConfig{Endpoint: "https://example.com/step1"},
						Method:       "GET",
					},
					{
						ClientConfig: confighttp.ClientConfig{Endpoint: "https://example.com/step2"},
						Method:       "POST",
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sequenceCfg.Validate()
			if tc.expectedErr != nil {
				assert.Error(t, err)
				if err != nil {
					assert.Contains(t, tc.expectedErr.Error(), err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
