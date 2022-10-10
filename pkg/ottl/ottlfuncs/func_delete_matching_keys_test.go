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

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_deleteMatchingKeys(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	target := &ottl.StandardGetSetter[pcommon.Map]{
		Getter: func(ctx pcommon.Map) interface{} {
			return ctx
		},
	}

	tests := []struct {
		name    string
		target  ottl.Getter[pcommon.Map]
		pattern string
		want    func(pcommon.Map)
	}{
		{
			name:    "delete everything",
			target:  target,
			pattern: "test.*",
			want: func(expectedMap pcommon.Map) {
				expectedMap.EnsureCapacity(3)
			},
		},
		{
			name:    "delete attributes that end in a number",
			target:  target,
			pattern: "\\d$",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name:    "delete nothing",
			target:  target,
			pattern: "not a matching pattern",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := DeleteMatchingKeys(tt.target, tt.pattern)
			require.NoError(t, err)
			exprFunc(scenarioMap)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_deleteMatchingKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			return ctx
		},
	}

	exprFunc, err := DeleteMatchingKeys[interface{}](target, "anything")
	require.NoError(t, err)
	exprFunc(input)

	assert.Equal(t, pcommon.NewValueInt(1), input)
}

func Test_deleteMatchingKeys_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			return ctx
		},
	}

	exprFunc, err := DeleteMatchingKeys[interface{}](target, "anything")
	require.NoError(t, err)
	assert.Nil(t, exprFunc(nil))
}

func Test_deleteMatchingKeys_invalid_pattern(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			t.Errorf("nothing should be received in this scenario")
			return nil
		},
	}

	invalidRegexPattern := "*"
	_, err := DeleteMatchingKeys[interface{}](target, invalidRegexPattern)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error parsing regexp:")
}
