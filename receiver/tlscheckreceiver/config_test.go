// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
			desc: "invalid endpoint",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						TCPAddrConfig: confignet.TCPAddrConfig{
							Endpoint: "bad-endpoint:12efg",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: provided port is not a number: 12efg", errInvalidEndpoint),
		},
		{
			desc: "endpoint with scheme",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						TCPAddrConfig: confignet.TCPAddrConfig{
							Endpoint: "https://example.com:443",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("endpoint contains a scheme, which is not allowed: https://example.com:443"),
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
		{
			desc: "relative file path",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						FilePath: "cert.pem",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("file path must be absolute: cert.pem"),
		},
		{
			desc: "nonexistent file",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						FilePath: filepath.Join(tmpDir, "nonexistent.pem"),
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("certificate file does not exist"),
		},
		{
			desc: "directory instead of file",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						FilePath: tmpDir,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("path is a directory, not a file: %s", tmpDir),
		},
		{
			desc: "port out of range",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						TCPAddrConfig: confignet.TCPAddrConfig{
							Endpoint: "example.com:67000",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: provided port is out of valid range (1-65535): 67000", errInvalidEndpoint),
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
