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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDefault(t *testing.T) {
	testCases := []struct {
		name      string
		expectErr bool
		param     Parameter
	}{
		{
			"ValidStringDefault",
			false,
			Parameter{
				Type:    "string",
				Default: "test",
			},
		},
		{
			"InvalidStringDefault",
			true,
			Parameter{
				Type:    "string",
				Default: 5,
			},
		},
		{
			"ValidIntDefault",
			false,
			Parameter{
				Type:    "int",
				Default: 5,
			},
		},
		{
			"InvalidStringDefault",
			true,
			Parameter{
				Type:    "int",
				Default: "test",
			},
		},
		{
			"ValidBoolDefault",
			false,
			Parameter{
				Type:    "bool",
				Default: true,
			},
		},
		{
			"InvalidBoolDefault",
			true,
			Parameter{
				Type:    "bool",
				Default: "test",
			},
		},
		{
			"ValidStringsDefault",
			false,
			Parameter{
				Type:    "strings",
				Default: []interface{}{"test"},
			},
		},
		{
			"InvalidStringsDefault",
			true,
			Parameter{
				Type:    "strings",
				Default: []interface{}{5},
			},
		},
		{
			"ValidEnumDefault",
			false,
			Parameter{
				Type:        "enum",
				ValidValues: []string{"test"},
				Default:     "test",
			},
		},
		{
			"InvalidEnumDefault",
			true,
			Parameter{
				Type:        "enum",
				ValidValues: []string{"test"},
				Default:     "invalid",
			},
		},
		{
			"NonStringEnumDefault",
			true,
			Parameter{
				Type:        "enum",
				ValidValues: []string{"test"},
				Default:     5,
			},
		},
		{
			"InvalidTypeDefault",
			true,
			Parameter{
				Type:    "float",
				Default: 5,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.param.validateDefault()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateValue(t *testing.T) {
	testCases := []struct {
		name      string
		expectErr bool
		param     Parameter
		value     interface{}
	}{
		{
			"ValidString",
			false,
			Parameter{
				Type:    "string",
				Default: "test",
			},
			"string",
		},
		{
			"InvalidString",
			true,
			Parameter{
				Type:    "string",
				Default: "test",
			},
			5,
		},
		{
			"ValidInt",
			false,
			Parameter{
				Type:    "int",
				Default: 5,
			},
			5,
		},
		{
			"InvalidInt",
			true,
			Parameter{
				Type:    "int",
				Default: 5,
			},
			"test",
		},
		{
			"ValidBool",
			false,
			Parameter{
				Type:    "bool",
				Default: true,
			},
			false,
		},
		{
			"InvalidBool",
			true,
			Parameter{
				Type:    "bool",
				Default: true,
			},
			"test",
		},
		{
			"ValidStringsAsInterface",
			false,
			Parameter{
				Type:    "strings",
				Default: []interface{}{"test"},
			},
			[]interface{}{"test"},
		},
		{
			"ValidStrings",
			false,
			Parameter{
				Type:    "strings",
				Default: []interface{}{"test"},
			},
			[]string{"test"},
		},
		{
			"InvalidStringsAsInterface",
			true,
			Parameter{
				Type:    "strings",
				Default: []interface{}{"test"},
			},
			[]interface{}{5},
		},
		{
			"InvalidStrings",
			true,
			Parameter{
				Type:    "strings",
				Default: []interface{}{"test"},
			},
			[]int{5},
		},
		{
			"ValidEnum",
			false,
			Parameter{
				Type:        "enum",
				ValidValues: []string{"test"},
				Default:     "test",
			},
			"test",
		},
		{
			"InvalidEnumValue",
			true,
			Parameter{
				Type:        "enum",
				ValidValues: []string{"test"},
				Default:     "test",
			},
			"missing",
		},
		{
			"InvalidEnumtype",
			true,
			Parameter{
				Type:        "enum",
				ValidValues: []string{"test"},
				Default:     "test",
			},
			5,
		},
		{
			"InvalidType",
			true,
			Parameter{
				Type:    "float",
				Default: 5,
			},
			5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.param.validateValue(tc.value)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
