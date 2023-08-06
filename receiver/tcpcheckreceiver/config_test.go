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

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/multierr"
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
			desc: "missing endpoint",
			cfg: &Config{
				NetAddr: confignet.NetAddr{
					Transport: "tcp",
				},
			},
			expectedErr: multierr.Combine(errMissingEndpoint),
		},
		{
			desc: "missing transport",
			cfg: &Config{
				NetAddr: confignet.NetAddr{
					Endpoint: "localhost:1234",
				},
			},
			expectedErr: multierr.Combine(
				error(nil),
			),
		},
		{
			desc: "missing endpoint and unknown transport",
			cfg: &Config{
				NetAddr: confignet.NetAddr{
					Transport: "gopherpro",
				},
			},
			expectedErr: multierr.Combine(
				errMissingEndpoint,
				errUnknownTransport,
			),
		},
		{
			desc: "unknown transport",
			cfg: &Config{
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:2222",
					Transport: "gopherpro",
				},
			},
			expectedErr: multierr.Combine(
				errUnknownTransport,
			),
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
