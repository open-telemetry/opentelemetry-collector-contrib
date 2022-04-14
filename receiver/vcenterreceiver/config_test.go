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

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestConfigValidation(t *testing.T) {
	cases := []struct {
		desc        string
		cfg         Config
		expectedErr error
	}{
		{
			desc: "empty endpoint",
			cfg: Config{
				MetricsConfig: &MetricsConfig{
					Endpoint: "",
				},
			},
			expectedErr: errors.New("no endpoint was provided"),
		},
		{
			desc: "with endpoint",
			cfg: Config{
				MetricsConfig: &MetricsConfig{
					Endpoint: "http://vcsa.some-host",
				},
			},
		},
		{
			desc: "not http or https",
			cfg: Config{
				MetricsConfig: &MetricsConfig{
					Endpoint: "ws://vcsa.some-host",
				},
			},
			expectedErr: errors.New("url scheme must be http or https"),
		},
		{
			desc: "unparseable URL",
			cfg: Config{
				MetricsConfig: &MetricsConfig{
					Endpoint:         "h" + string(rune(0x7f)),
					TLSClientSetting: configtls.TLSClientSetting{},
				},
			},
			expectedErr: errors.New("unable to parse url"),
		},
		{
			desc: "no username",
			cfg: Config{
				MetricsConfig: &MetricsConfig{
					Endpoint: "https://vcsa.some-host",
					Password: "otelp",
				},
			},
			expectedErr: errors.New("username not provided"),
		},
		{
			desc: "no password",
			cfg: Config{
				MetricsConfig: &MetricsConfig{
					Endpoint: "https://vcsa.some-host",
					Username: "otelu",
				},
			},
			expectedErr: errors.New("password not provided"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			}
		})
	}
}
