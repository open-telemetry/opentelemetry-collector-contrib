// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name        string
		config      Config
		expectedErr string
	}{
		{
			name: "Valid config",
			config: Config{
				Logs: LogsConfig{
					Endpoint: "0.0.0.0:9999",
				},
			},
		},
		{
			name: "Valid config with tls",
			config: Config{
				Logs: LogsConfig{
					Endpoint: "0.0.0.0:9999",
					TLS: &configtls.TLSServerSetting{
						TLSSetting: configtls.TLSSetting{
							CertFile: "some_cert_file",
							KeyFile:  "some_key_file",
						},
					},
				},
			},
		},
		{
			name: "missing endpoint",
			config: Config{
				Logs: LogsConfig{},
			},
			expectedErr: errNoEndpoint.Error(),
		},
		{
			name: "Invalid endpoint",
			config: Config{
				Logs: LogsConfig{
					Endpoint: "9999",
				},
			},
			expectedErr: "failed to split endpoint into 'host:port' pair",
		},
		{
			name: "TLS config missing key",
			config: Config{
				Logs: LogsConfig{
					Endpoint: "0.0.0.0:9999",
					TLS: &configtls.TLSServerSetting{
						TLSSetting: configtls.TLSSetting{
							CertFile: "some_cert_file",
						},
					},
				},
			},
			expectedErr: errNoKey.Error(),
		},
		{
			name: "TLS config missing cert",
			config: Config{
				Logs: LogsConfig{
					Endpoint: "0.0.0.0:9999",
					TLS: &configtls.TLSServerSetting{
						TLSSetting: configtls.TLSSetting{
							KeyFile: "some_key_file",
						},
					},
				},
			},
			expectedErr: errNoCert.Error(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cases := []struct {
		name           string
		expectedConfig component.Config
	}{
		{
			name: "",
			expectedConfig: &Config{
				Logs: LogsConfig{
					Endpoint: "0.0.0.0:12345",
					TLS: &configtls.TLSServerSetting{
						TLSSetting: configtls.TLSSetting{
							CertFile: "some_cert_file",
							KeyFile:  "some_key_file",
						},
					},
					Secret:         "1234567890abcdef1234567890abcdef",
					TimestampField: "EdgeStartTimestamp",
					Attributes: map[string]string{
						"ClientIP":         "http_request.client_ip",
						"ClientRequestURI": "http_request.uri",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			loaded, err := cm.Sub(component.NewIDWithName(metadata.Type, tc.name).String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(loaded, cfg))
			require.Equal(t, tc.expectedConfig, cfg)
			require.NoError(t, component.ValidateConfig(cfg))
		})
	}
}
