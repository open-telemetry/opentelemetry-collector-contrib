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

func Test_keepMatchingKeys(t *testing.T) {
	in := pcommon.NewMap()
	in.PutStr("foo", "bar")
	in.PutStr("foo1", "bar")
	in.PutInt("foo2", 3)

	target := &ottl.StandardPMapGetter[pcommon.Map]{
		Getter: func(_ context.Context, tCtx pcommon.Map) (any, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name      string
		target    ottl.PMapGetter[pcommon.Map]
		pattern   string
		want      func() *pcommon.Map
		wantError bool
	}{
		{
			name:    "keep everything that ends with a number",
			target:  target,
			pattern: "\\d$",
			want: func() *pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo1", "bar")
				m.PutInt("foo2", 3)
				return &m
			},
		},
		{
			name:    "keep nothing",
			target:  target,
			pattern: "bar.*",
			want: func() *pcommon.Map {
				m := pcommon.NewMap()
				// add and remove something to have an empty map instead of nil
				m.PutStr("k", "")
				m.Remove("k")
				return &m
			},
		},
		{
			name:    "keep everything",
			target:  target,
			pattern: "foo.*",
			want: func() *pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo", "bar")
				m.PutStr("foo1", "bar")
				m.PutInt("foo2", 3)
				return &m
			},
		},
		{
			name:    "invalid pattern",
			target:  target,
			pattern: "*",
			want: func() *pcommon.Map {
				return nil
			},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			in.CopyTo(scenarioMap)

			exprFunc, err := keepMatchingKeys(tt.target, tt.pattern)

			if tt.wantError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			assert.NoError(t, err)

			assert.Equal(t, *tt.want(), scenarioMap)
		})
	}
}

func Test_keepMatchingKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := keepMatchingKeys[any](target, "anything")
	assert.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_keepMatchingKeys_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := keepMatchingKeys[any](target, "anything")
	assert.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}
