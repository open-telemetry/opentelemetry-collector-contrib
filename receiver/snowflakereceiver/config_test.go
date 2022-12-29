// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snowflakereceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	var multierror error

	multierror = multierr.Append(multierror, errMissingPassword)
	multierror = multierr.Append(multierror, errMissingWarehouse)

	tests := []struct {
		desc   string
		expect error
		conf   Config
	}{
		{
			desc:   "Missing username all else present",
			expect: errMissingUsername,
			conf: Config{
				Username:  "",
				Password:  "password",
				Account:   "account",
				Warehouse: "warehouse",
			},
		},
		{
			desc:   "Missing password all else present",
			expect: errMissingPassword,
			conf: Config{
				Username:  "username",
				Password:  "",
				Account:   "account",
				Warehouse: "warehouse",
			},
		},
		{
			desc:   "Missing account all else present",
			expect: errMissingAccount,
			conf: Config{
				Username:  "username",
				Password:  "password",
				Account:   "",
				Warehouse: "warehouse",
			},
		},
		{
			desc:   "Missing warehouse all else present",
			expect: errMissingWarehouse,
			conf: Config{
				Username:  "username",
				Password:  "password",
				Account:   "account",
				Warehouse: "",
			},
		},
		{
			desc:   "Missing multiple check multierror",
			expect: multierror,
			conf: Config{
				Username:  "username",
				Password:  "",
				Account:   "account",
				Warehouse: "",
			},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			err := test.conf.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), test.expect.Error())
		})
	}
}
