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

func Test_keepKeys(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	target := &ottl.StandardPMapGetter[pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (any, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name   string
		target ottl.PMapGetter[pcommon.Map]
		keys   []string
		want   func(pcommon.Map)
	}{
		{
			name:   "keep one",
			target: target,
			keys:   []string{"test"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name:   "keep two",
			target: target,
			keys:   []string{"test", "test2"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
			},
		},
		{
			name:   "keep none",
			target: target,
			keys:   []string{},
			want:   func(expectedMap pcommon.Map) {},
		},
		{
			name:   "no match",
			target: target,
			keys:   []string{"no match"},
			want:   func(expectedMap pcommon.Map) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc := keepKeys(tt.target, tt.keys)

			_, err := exprFunc(nil, scenarioMap)
			assert.Nil(t, err)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_keepKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	keys := []string{"anything"}

	exprFunc := keepKeys[any](target, keys)

	_, err := exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_keepKeys_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	keys := []string{"anything"}

	exprFunc := keepKeys[any](target, keys)
	_, err := exprFunc(nil, nil)
	assert.Error(t, err)
}
