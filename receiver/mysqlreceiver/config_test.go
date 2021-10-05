// Copyright  OpenTelemetry Authors
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

package mysqlreceiver

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
			cfg:  &Config{},
			expected: multierr.Combine(
				errors.New(errNoUsername),
				errors.New(errNoPassword),
			),
		},
		{
			desc: "missing password",
			cfg: &Config{
				Username: "otel",
			},
			expected: multierr.Combine(
				errors.New(errNoPassword),
			),
		},
		{
			desc: "missing username",
			cfg: &Config{
				Password: "otel",
			},
			expected: multierr.Combine(
				errors.New(errNoUsername),
			),
		},
		{
			desc: "no error",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
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
