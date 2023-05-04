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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_limit(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	target := &ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name   string
		target ottl.PMapGetter[pcommon.Map]
		limit  int64
		keep   []string
		want   func(pcommon.Map)
	}{
		{
			name:   "limit to 1",
			target: target,
			limit:  int64(1),
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name:   "limit to zero",
			target: target,
			limit:  int64(0),
			want: func(expectedMap pcommon.Map) {
				expectedMap.EnsureCapacity(input.Len())
			},
		},
		{
			name:   "limit nothing",
			target: target,
			limit:  int64(100),
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "limit exact",
			target: target,
			limit:  int64(3),
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "keep one key",
			target: target,
			limit:  int64(2),
			keep:   []string{"test3"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "keep same # of keys as limit",
			target: target,
			limit:  int64(2),
			keep:   []string{"test", "test3"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "keep not existing key",
			target: target,
			limit:  int64(1),
			keep:   []string{"te"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name:   "keep not-/existing keys",
			target: target,
			limit:  int64(2),
			keep:   []string{"te", "test3"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := limit(tt.target, tt.limit, tt.keep)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioMap)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_limit_validation(t *testing.T) {
	tests := []struct {
		name   string
		target ottl.PMapGetter[interface{}]
		keep   []string
		limit  int64
	}{
		{
			name:   "limit less than zero",
			target: &ottl.StandardTypeGetter[interface{}, pcommon.Map]{},
			limit:  int64(-1),
		},
		{
			name:   "limit less than # of keep attrs",
			target: &ottl.StandardTypeGetter[interface{}, pcommon.Map]{},
			keep:   []string{"test", "test"},
			limit:  int64(1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := limit(tt.target, tt.limit, tt.keep)
			assert.Error(t, err)
		})
	}
}

func Test_limit_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := limit[interface{}](target, 1, []string{})
	assert.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_limit_get_nil(t *testing.T) {
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := limit[interface{}](target, 1, []string{})
	assert.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}
