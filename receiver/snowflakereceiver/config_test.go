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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/multierr"
)

func TestValidateConfig(t *testing.T) {
    t.Parallel()

    var(
        missingUsernameError  = errors.New("You must provide a valid snowflake username")
        missingPasswordError  = errors.New("You must provide a password for the snowflake username")
        missingAccountError   = errors.New("You must provide a valid account name")
        missingWarehouseError = errors.New("You must provide a valid warehouse name")
    )
    var multierror error 

    multierror = multierr.Append(multierror, missingPasswordError)
    multierror = multierr.Append(multierror, missingWarehouseError)
    
    tests := []struct {
        desc   string 
        expect error
        conf   Config
    }{
        {
            desc: "Missing username, all else present",
            expect: missingUsernameError,
            conf: Config{
                Username: "",
                Password: "password",
                Account: "account",
                Warehouse: "warehouse",
            },
        },
        {
            desc: "Missing password, all else present",
            expect: missingPasswordError,
            conf: Config{
                Username: "username",
                Password: "",
                Account: "account",
                Warehouse: "warehouse",
            },
        },
        {
            desc: "Missing account, all else present",
            expect: missingAccountError,
            conf: Config{
                Username: "username",
                Password: "password",
                Account: "",
                Warehouse: "warehouse",
            },
        },
        {
            desc: "Missing warehouse, all else present",
            expect: missingWarehouseError,
            conf: Config{
                Username: "username",
                Password: "password",
                Account: "account",
                Warehouse: "",
            },
        },
        {
            desc: "Missing multiple, check multierror",
            expect: multierror,
            conf: Config{
                Username: "username",
                Password: "",
                Account: "account",
                Warehouse: "",
            },
        },
    }
    
    for _, test := range tests {
        err := test.conf.Validate()
        require.ErrorIs(t, err, test.expect)
    }
}
