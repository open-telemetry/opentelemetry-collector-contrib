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

func Test_replacePattern(t *testing.T) {
	input := pcommon.NewValueStr("application passwd=sensitivedtata otherarg=notsensitive key1 key2")

	target := &ottl.StandardGetSetter[pcommon.Value]{
		Getter: func(ctx pcommon.Value) interface{} {
			return ctx.Str()
		},
		Setter: func(ctx pcommon.Value, val interface{}) {
			ctx.SetStr(val.(string))
		},
	}

	tests := []struct {
		name        string
		target      ottl.GetSetter[pcommon.Value]
		pattern     string
		replacement string
		want        func(pcommon.Value)
	}{
		{
			name:        "replace regex match",
			target:      target,
			pattern:     `passwd\=[^\s]*(\s?)`,
			replacement: "passwd=*** ",
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=*** otherarg=notsensitive key1 key2")
			},
		},
		{
			name:        "no regex match",
			target:      target,
			pattern:     `nomatch\=[^\s]*(\s?)`,
			replacement: "shouldnotbeinoutput",
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=sensitivedtata otherarg=notsensitive key1 key2")
			},
		},
		{
			name:        "multiple regex match",
			target:      target,
			pattern:     `key[^\s]*(\s?)`,
			replacement: "**** ",
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=sensitivedtata otherarg=notsensitive **** **** ")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())

			exprFunc, err := ReplacePattern(tt.target, tt.pattern, tt.replacement)
			require.NoError(t, err)
			assert.Nil(t, exprFunc(scenarioValue))

			expected := pcommon.NewValueStr("")
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_replacePattern_bad_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			return ctx
		},
		Setter: func(ctx interface{}, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	exprFunc, err := ReplacePattern[interface{}](target, "regexp", "{replacement}")
	require.NoError(t, err)
	assert.Nil(t, exprFunc(input))
	assert.Equal(t, pcommon.NewValueInt(1), input)
}

func Test_replacePattern_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			return ctx
		},
		Setter: func(ctx interface{}, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	exprFunc, err := ReplacePattern[interface{}](target, `nomatch\=[^\s]*(\s?)`, "{anything}")
	require.NoError(t, err)
	assert.Nil(t, exprFunc(nil))
}

func Test_replacePatterns_invalid_pattern(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			t.Errorf("nothing should be received in this scenario")
			return nil
		},
		Setter: func(ctx interface{}, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	invalidRegexPattern := "*"
	_, err := ReplacePattern[interface{}](target, invalidRegexPattern, "{anything}")
	require.Error(t, err)
	assert.ErrorContains(t, err, "error parsing regexp:")
}
