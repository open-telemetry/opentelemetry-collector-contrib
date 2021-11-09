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

package postgresqlreceiver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/multierr"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc     string
		cfg      *Config
		expected error
	}{
		{
			desc: "missing username and password",
			cfg:  &Config{},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
				errors.New(ErrNoPassword),
				errors.New(ErrTransportsSupported),
			),
		},
		{
			desc: "missing password",
			cfg: &Config{
				Username: "otel",
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:5432",
					Transport: "tcp",
				},
			},
			expected: multierr.Combine(
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing username",
			cfg: &Config{
				Password: "otel",
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:5432",
					Transport: "tcp",
				},
			},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
			),
		},
		{
			desc: "bad endpoint",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				NetAddr: confignet.NetAddr{
					Endpoint:  "open-telemetry",
					Transport: "tcp",
				},
			},
			expected: multierr.Combine(
				errors.New(ErrHostPort),
			),
		},
		{
			desc: "bad transport",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:5432",
					Transport: "teacup",
				},
			},
			expected: multierr.Combine(
				errors.New(ErrTransportsSupported),
			),
		},
		{
			desc: "unsupported SSL params",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:5432",
					Transport: "unix",
				},
				TLSClientSetting: configtls.TLSClientSetting{
					ServerName: "notlocalhost",
					TLSSetting: configtls.TLSSetting{
						MinVersion: "1.0",
						MaxVersion: "1.7",
					},
				},
			},
			expected: multierr.Combine(
				fmt.Errorf(ErrNotSupported, "ServerName"),
				fmt.Errorf(ErrNotSupported, "MaxVersion"),
				fmt.Errorf(ErrNotSupported, "MinVersion"),
			),
		},
		{
			desc: "no error",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:5432",
					Transport: "tcp",
				},
			},
			expected: nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			actual := tC.cfg.Validate()
			require.Equal(t, tC.expected, actual)
		})
	}
}
