// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aerospikereceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		name     string
		config   *Config
		expected error
	}{
		{
			name: "blank endpoint",
			config: &Config{
				Endpoint: "",
			},
			expected: errEmptyEndpoint,
		},
		{
			name: "missing port",
			config: &Config{
				Endpoint: "localhost",
			},
			expected: errBadEndpoint,
		},
		{
			name: "bad endpoint",
			config: &Config{
				Endpoint: "x;;ef;s;d:::ss:23423423423423423",
			},
			expected: errBadEndpoint,
		},
		{
			name: "missing host",
			config: &Config{
				Endpoint: ":3001",
			},
			expected: errBadEndpoint,
		},
		{
			name: "negative port",
			config: &Config{
				Endpoint: "localhost:-2",
			},
			expected: errBadPort,
		},
		{
			name: "bad port",
			config: &Config{
				Endpoint: "localhost:9999999999999999999",
			},
			expected: errBadPort,
		},
		{
			name: "negative timeout",
			config: &Config{
				Endpoint: "localhost:3000",
				Timeout:  -1 * time.Second,
			},
			expected: errNegativeTimeout,
		},
		{
			name: "password but no username",
			config: &Config{
				Endpoint: "localhost:3000",
				Username: "",
				Password: "secret",
			},
			expected: errEmptyUsername,
		},
		{
			name: "username but no password",
			config: &Config{
				Endpoint: "localhost:3000",
				Username: "ro_user",
			},
			expected: errEmptyPassword,
		},
		{
			name: "bad TLS config",
			config: &Config{
				Endpoint: "localhost:3000",
				TLSName:  "tls1",
				TLS: &configtls.TLSClientSetting{
					Insecure: false,
					TLSSetting: configtls.TLSSetting{
						CAFile: "BADCAFILE",
					},
				},
			},
			expected: errFailedTLSLoad,
		},
		{
			name: "empty tls name",
			config: &Config{
				Endpoint: "localhost:3000",
				TLSName:  "",
				TLS: &configtls.TLSClientSetting{
					Insecure:   false,
					TLSSetting: configtls.TLSSetting{},
				},
			},
			expected: errEmptyEndpointTLSName,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			require.ErrorIs(t, err, tc.expected)
		})
	}
}
