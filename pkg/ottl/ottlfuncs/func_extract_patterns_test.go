// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_extractPatterns(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return `a=b c=d`, nil
		},
	}
	tests := []struct {
		name    string
		target  ottl.StringGetter[any]
		pattern string
		want    func(pcommon.Map)
	}{
		{
			name:    "extract patterns",
			target:  target,
			pattern: `^a=(?P<a>\w+)\s+c=(?P<c>\w+)$`,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("a", "b")
				expectedMap.PutStr("c", "d")
			},
		},
		{
			name:    "no pattern found",
			target:  target,
			pattern: `^a=(?P<a>\w+)$`,
			want:    func(_ pcommon.Map) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := extractPatterns(tt.target, tt.pattern)
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)

			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected.Len(), resultMap.Len())
			for k := range expected.All() {
				ev, _ := expected.Get(k)
				av, _ := resultMap.Get(k)
				assert.Equal(t, ev, av)
			}
		})
	}
}

func Test_extractPatterns_validation(t *testing.T) {
	tests := []struct {
		name    string
		target  ottl.StringGetter[any]
		pattern string
	}{
		{
			name: "bad regex",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "foobar", nil
				},
			},
			pattern: "(",
		},
		{
			name: "no named capture group",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "foobar", nil
				},
			},
			pattern: "(.*)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := extractPatterns[any](tt.target, tt.pattern)
			assert.Error(t, err)
			assert.Nil(t, exprFunc)
		})
	}
}

func Test_extractPatterns_bad_input(t *testing.T) {
	tests := []struct {
		name    string
		target  ottl.StringGetter[any]
		pattern string
	}{
		{
			name: "target is non-string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 123, nil
				},
			},
			pattern: "(?P<line>.*)",
		},
		{
			name: "target is nil",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			pattern: "(?P<line>.*)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := extractPatterns[any](tt.target, tt.pattern)
			assert.NoError(t, err)

			result, err := exprFunc(nil, nil)
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}
