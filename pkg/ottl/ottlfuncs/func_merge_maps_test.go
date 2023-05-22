// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_MergeMaps(t *testing.T) {

	input := pcommon.NewMap()
	input.PutStr("attr1", "value1")

	targetGetter := &ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name     string
		source   ottl.PMapGetter[pcommon.Map]
		strategy string
		want     func(pcommon.Map)
	}{
		{
			name: "Upsert no conflicting keys",
			source: ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
				Getter: func(ctx context.Context, _ pcommon.Map) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: UPSERT,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "Upsert conflicting key",
			source: ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
				Getter: func(ctx context.Context, _ pcommon.Map) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("attr1", "value3")
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: UPSERT,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value3")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "Insert no conflicting keys",
			source: ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
				Getter: func(ctx context.Context, _ pcommon.Map) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: INSERT,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "Insert conflicting key",
			source: ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
				Getter: func(ctx context.Context, _ pcommon.Map) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("attr1", "value3")
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: INSERT,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "Update no conflicting keys",
			source: ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
				Getter: func(ctx context.Context, _ pcommon.Map) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: UPDATE,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
			},
		},
		{
			name: "Update conflicting key",
			source: ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
				Getter: func(ctx context.Context, _ pcommon.Map) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("attr1", "value3")
					return m, nil
				},
			},
			strategy: UPDATE,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value3")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := mergeMaps[pcommon.Map](targetGetter, tt.source, tt.strategy)
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), scenarioMap)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_MergeMaps_bad_target(t *testing.T) {
	input := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return 1, nil
		},
	}

	exprFunc, err := mergeMaps[interface{}](target, input, "insert")
	assert.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_MergeMaps_bad_input(t *testing.T) {
	input := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return 1, nil
		},
	}
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := mergeMaps[interface{}](target, input, "insert")
	assert.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}
