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

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"testing"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/configtls"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component/componenttest"
)

// check that OTel Collector patterns are implemented
func TestCheckConfig(t *testing.T) {
	t.Parallel()
	if err := componenttest.CheckConfigStruct(&Config{}); err != nil {
		t.Fatal(err)
	}
}

// test the validate function for config
func TestValidate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "invalid endpoint",
			cfg: &Config{
				TLSCertsClientSettings: configtls.TLSCertsClientSettings{
					Endpoint: "badendpoint . cuz peanuts:2222",
				},
			},
			expectedErr: multierr.Combine(
				errInvalidEndpoint,
			),
		},
		{
			desc: "invalid local cert path",
			cfg: &Config{
				TLSCertsClientSettings: configtls.TLSCertsClientSettings{
					Endpoint:      defaultEndpoint,
					LocalCertPath: "cert.cerx",
				},
			},
			expectedErr: multierr.Combine(
				errInvalidCertPath,
			),
		},
		{
			desc: "no error with endpoint and local cert path",
			cfg: &Config{
				TLSCertsClientSettings: configtls.TLSCertsClientSettings{
					Endpoint:      defaultEndpoint,
					LocalCertPath: "cert.cerx",
				},
			},
			expectedErr: error(nil),
		},
		{
			desc: "no error with local_cert_path",
			cfg: &Config{
				TLSCertsClientSettings: configtls.TLSCertsClientSettings{
					Endpoint:      defaultEndpoint,
					LocalCertPath: "cert.cer",
				},
			},
			expectedErr: error(nil),
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
