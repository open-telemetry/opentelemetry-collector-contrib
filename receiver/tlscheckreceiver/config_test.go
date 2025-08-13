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
