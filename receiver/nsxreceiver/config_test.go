// Copyright  The OpenTelemetry Authors
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

package nsxreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestID(t *testing.T) {
	config := createDefaultConfig()
	require.NotEmpty(t, config.ID())
}

func TestMetricValidation(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	cases := []struct {
		desc          string
		cfg           *Config
		expectedError error
	}{
		{
			desc: "default config",
			cfg:  defaultConfig,
		},
		{
			desc: "not valid scheme",
			cfg: &Config{
				MetricsConfig: &MetricsConfig{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "wss://not-supported-websockets",
					},
				},
			},
			expectedError: errors.New("url scheme must be http or https"),
		},
		{
			desc: "unparseable url",
			cfg: &Config{
				MetricsConfig: &MetricsConfig{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "\x00",
					},
				},
			},
			expectedError: errors.New("parse"),
		},
		{
			desc: "username not provided",
			cfg: &Config{
				MetricsConfig: &MetricsConfig{
					Password: "password",
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "http://localhost",
					},
				},
			},
			expectedError: errors.New("username not provided"),
		},
		{
			desc: "password not provided",
			cfg: &Config{
				MetricsConfig: &MetricsConfig{
					Username: "otelu",
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "http://localhost",
					},
				},
			},
			expectedError: errors.New("password not provided"),
		},
	}
	for _, tc := range cases {
		err := tc.cfg.Validate()
		if tc.expectedError != nil {
			require.ErrorContains(t, err, tc.expectedError.Error())
		} else {
			require.NoError(t, err)
		}
	}
}
