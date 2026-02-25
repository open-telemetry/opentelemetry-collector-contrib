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
	"go.opentelemetry.io/collector/config/configopaque"
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
		{
			desc: "valid jks file_format with password",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						FilePath:   tmpFile.Name(),
						FileFormat: FileFormatJKS,
						Password:   configopaque.String("changeit"),
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "valid pkcs12 file_format with empty password",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						FilePath:   tmpFile.Name(),
						FileFormat: FileFormatPKCS12,
						Password:   configopaque.String(""),
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "valid auto file_format (zero value)",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						FilePath:   tmpFile.Name(),
						FileFormat: FileFormatAuto,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "valid pem file_format explicit",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						FilePath:   tmpFile.Name(),
						FileFormat: FileFormatPEM,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "invalid file_format value",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						FilePath:   tmpFile.Name(),
						FileFormat: FileFormat("der"),
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errInvalidFileFormat,
		},
		{
			desc: "endpoint with file_format set",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: "example.com:443"},
						FileFormat:    FileFormatJKS,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New(`"file_format" cannot be set when "file_path" is empty (endpoint-based target)`),
		},
		{
			desc: "endpoint with password set",
			cfg: &Config{
				Targets: []*CertificateTarget{
					{
						TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: "example.com:443"},
						Password:      configopaque.String("secret"),
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New(`"password" cannot be set when "file_path" is empty (endpoint-based target)`),
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

func TestResolveFileFormat(t *testing.T) {
	testCases := []struct {
		format   FileFormat
		path     string
		expected FileFormat
	}{
		{FileFormatAuto, "/etc/certs/server.jks", FileFormatJKS},
		{FileFormatAuto, "/etc/certs/server.p12", FileFormatPKCS12},
		{FileFormatAuto, "/etc/certs/server.pfx", FileFormatPKCS12},
		{FileFormatAuto, "/etc/certs/server.pem", FileFormatPEM},
		{FileFormatAuto, "/etc/certs/server.crt", FileFormatPEM},
		{"", "/etc/certs/server.jks", FileFormatJKS},
		{"", "/etc/certs/server.p12", FileFormatPKCS12},
		{"", "/etc/certs/server.pem", FileFormatPEM},
		// Explicit format overrides extension
		{FileFormatJKS, "/etc/certs/server.pem", FileFormatJKS},
		{FileFormatPEM, "/etc/certs/server.p12", FileFormatPEM},
		{FileFormatPKCS12, "/etc/certs/server.jks", FileFormatPKCS12},
	}

	for _, tc := range testCases {
		t.Run(string(tc.format)+"_"+tc.path, func(t *testing.T) {
			got := resolveFileFormat(tc.format, tc.path)
			require.Equal(t, tc.expected, got)
		})
	}
}
