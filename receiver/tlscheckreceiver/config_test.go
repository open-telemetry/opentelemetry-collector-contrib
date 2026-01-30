// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

func TestValidate(t *testing.T) {
	// Create a temporary certificate file for testing
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-cert-*.pem")
	require.NoError(t, err)

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	t.Cleanup(func() {
		tmpFile.Close()
		os.RemoveAll(tmpDir)
	})

	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing targets",
			cfg: &Config{
				Targets:          []*CertificateTarget{},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errMissingTargets,
		},
		{
			desc: "valid endpoint config",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						TCPAddrConfig: confignet.TCPAddrConfig{
							Endpoint: "opentelemetry.io:443",
							DialerConfig: confignet.DialerConfig{
								Timeout: 3 * time.Second,
							},
						},
					},
					{
						TCPAddrConfig: confignet.TCPAddrConfig{
							Endpoint: "opentelemetry.io:8080",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "valid file path config",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						FilePath: tmpFile.Name(),
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "mixed valid config",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						TCPAddrConfig: confignet.TCPAddrConfig{
							Endpoint: "opentelemetry.io:443",
						},
					},
					{
						FilePath: tmpFile.Name(),
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "both endpoint and file path",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						TCPAddrConfig: confignet.TCPAddrConfig{
							Endpoint: "example.com:443",
						},
						FilePath: tmpFile.Name(),
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("cannot specify both endpoint and file_path"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, actualErr, tc.expectedErr.Error())
			} else {
				require.NoError(t, actualErr)
			}
		})
	}
}

func TestValidateForDiscovery(t *testing.T) {
	tests := []struct {
		name               string
		cfg                *Config
		rawCfg             map[string]any
		discoveredEndpoint string
		expectedErr        bool
		expectedErrString  string
	}{
		{
			name: "endpoint and discovered endpoint match",
			cfg:  &Config{},
			rawCfg: map[string]any{
				"targets": []any{
					map[string]any{
						"endpoint": "test-endpoint.com:443",
					},
				},
			},
			discoveredEndpoint: "test-endpoint.com:443",
			expectedErr:        false,
		},
		{
			name: "endpoint and discovered endpoint don't match",
			cfg:  &Config{},
			rawCfg: map[string]any{
				"targets": []any{
					map[string]any{
						"endpoint": "test-endpoint.com:443",
					},
				},
			},
			discoveredEndpoint: "wrong-endpoint",
			expectedErr:        true,
			expectedErrString:  "targets[0].endpoint \"test-endpoint.com:443\" does not match discoveredEndpoint \"wrong-endpoint\"",
		},
		{
			name: "rawCfg does not contain a []any slice",
			cfg:  &Config{},
			rawCfg: map[string]any{
				"targets": "endpoint.com:443",
			},
			discoveredEndpoint: "endpoint.com:443",
			expectedErr:        false,
		},
		{
			name: "Target endpoint is not a map[string]any",
			cfg:  &Config{},
			rawCfg: map[string]any{
				"targets": []any{
					"just-a-string",
				},
			},
			discoveredEndpoint: "endpoint.com:443",
			expectedErr:        false,
		},
		{
			name: "Target has no endpoint key",
			cfg:  &Config{},
			rawCfg: map[string]any{
				"targets": []any{
					map[string]any{
						"other-key": "value",
					},
				},
			},
			discoveredEndpoint: "endpoint.com:443",
			expectedErr:        false,
		},
		{
			name: "endpoint value is not a string",
			cfg:  &Config{},
			rawCfg: map[string]any{
				"targets": []any{
					map[string]any{
						"endpoint": 123,
					},
				},
			},
			discoveredEndpoint: "endpoint.com:443",
			expectedErr:        true,
			expectedErrString:  "targets[0].endpoint: expected string",
		},
		{
			name: "Multi target, second element is not matched",
			cfg:  &Config{},
			rawCfg: map[string]any{
				"targets": []any{
					map[string]any{
						"endpoint": "endpoint.com:443",
					},
					map[string]any{
						"endpoint": "wrong-endpoint",
					},
				},
			},
			discoveredEndpoint: "endpoint.com:443",
			expectedErr:        true,
			expectedErrString:  "targets[1].endpoint \"wrong-endpoint\" does not match discoveredEndpoint \"endpoint.com:443\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.ValidateForDiscovery(tt.rawCfg, tt.discoveredEndpoint)
			if tt.expectedErr {
				require.Error(t, err, "require error")
				require.EqualError(t, err, tt.expectedErrString, "error does not match")
			} else {
				require.NoError(t, err, "no error required")
			}
		})
	}
}
