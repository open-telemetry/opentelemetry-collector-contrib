// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name        string
		config      Config
		expectedErr string
	}{
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
				Logs: LogsConfig{
					TLS: &configtls.TLSServerSetting{
						TLSSetting: configtls.TLSSetting{
							CertFile: "some_cert_file",
							KeyFile:  "some_key_file",
						},
					},
				},
			},
			expectedErr: errNoEndpoint.Error(),
		},
		{
			name: "Invalid endpoint",
			config: Config{
				Logs: LogsConfig{
					Endpoint: "9999",
					TLS: &configtls.TLSServerSetting{
						TLSSetting: configtls.TLSSetting{
							CertFile: "some_cert_file",
							KeyFile:  "some_key_file",
						},
					},
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

			loaded, err := cm.Sub(component.NewIDWithName(typeStr, tc.name).String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(loaded, cfg))
			require.Equal(t, tc.expectedConfig, cfg)
			require.NoError(t, component.ValidateConfig(cfg))
		})
	}
}
