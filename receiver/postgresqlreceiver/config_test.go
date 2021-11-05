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
	"testing"

	"github.com/stretchr/testify/require"
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
			cfg: &Config{
				SSLConfig: SSLConfig{
					SSLMode: "require",
				},
			},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing password",
			cfg: &Config{
				Username: "otel",
				SSLConfig: SSLConfig{
					SSLMode: "require",
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
				SSLConfig: SSLConfig{
					SSLMode: "require",
				},
			},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
			),
		},
		{
			desc: "bad SSL mode",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				SSLConfig: SSLConfig{
					SSLMode: "assume",
				},
			},
			expected: multierr.Combine(
				errors.New("SSL Mode 'assume' not supported, valid values are 'require', 'verify-ca', 'verify-full', 'disable'. The default is 'require'"),
			),
		},
		{
			desc: "no error",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				SSLConfig: SSLConfig{
					SSLMode: "require",
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
