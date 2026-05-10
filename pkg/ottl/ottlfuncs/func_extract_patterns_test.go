// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_extractPatterns(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return `a=b c=d`, nil
		},
	}
	tests := []struct {
		name    string
		target  ottl.StringGetter[any]
		pattern ottl.StringGetter[any]
		want    func(pcommon.Map)
	}{
		{
			name:   "extract patterns",
			target: target,
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return `^a=(?P<a>\w+)\s+c=(?P<c>\w+)$`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("a", "b")
				expectedMap.PutStr("c", "d")
			},
		},
		{
			name:   "no pattern found",
			target: target,
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return `^a=(?P<a>\w+)$`, nil
				},
			},
			want: func(_ pcommon.Map) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := extractPatterns(tt.target, tt.pattern)
			require.NoError(t, err)

			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)

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
		pattern ottl.StringGetter[any]
	}{
		{
			name: "bad regex",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "foobar", nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "(", nil
				},
			},
		},
		{
			name: "no named capture group",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "foobar", nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "(.*)", nil
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := extractPatterns[any](tt.target, tt.pattern)
			require.NoError(t, err)
			assert.NotNil(t, exprFunc)
			_, err = exprFunc(t.Context(), nil)
			assert.Error(t, err)
		})
	}
}

func Test_extractPatternsLiteralPatternWithoutNamedCaptureFailsAtParse(t *testing.T) {
	parser, err := ottl.NewParser[any](
		StandardConverters[any](),
		func(path ottl.Path[any]) (ottl.GetSetter[any], error) {
			require.Equal(t, "body", path.Name())
			return &ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "foobar", nil
				},
				Setter: func(context.Context, any, any) error {
					return nil
				},
			}, nil
		},
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)

	_, err = parser.ParseValueExpression(`ExtractPatterns(body, "(.*)")`)
	require.ErrorContains(t, err, "at least 1 named capture group must be supplied in the given regex")
}

func Test_extractPatterns_bad_input(t *testing.T) {
	tests := []struct {
		name    string
		target  ottl.StringGetter[any]
		pattern ottl.StringGetter[any]
	}{
		{
			name: "target is non-string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return 123, nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "(?P<line>.*)", nil
				},
			},
		},
		{
			name: "target is nil",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return nil, nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "(?P<line>.*)", nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := extractPatterns[any](tt.target, tt.pattern)
			require.NoError(t, err)

			result, err := exprFunc(nil, nil)
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func Test_newRegexLiteralPrefilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pattern     string
		wantNil     bool
		matching    string
		nonMatching string
	}{
		{
			name:        "static literal before named capture",
			pattern:     `GIT_SHA=(?P<git_sha>\w+)`,
			matching:    "GIT_SHA=abc123",
			nonMatching: "OTHER=abc123",
		},
		{
			name:        "keeps required literal after optional atom",
			pattern:     `foo?bar(?P<value>\w+)`,
			matching:    "fobar123",
			nonMatching: "fooqux123",
		},
		{
			name:    "skips top level alternation",
			pattern: `foo(?P<a>\w+)|bar(?P<b>\w+)`,
			wantNil: true,
		},
		{
			name:    "skips case insensitive patterns",
			pattern: `(?i)git_sha=(?P<git_sha>\w+)`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefilter := newRegexLiteralPrefilter(tt.pattern)
			if tt.wantNil {
				require.Nil(t, prefilter)
				return
			}
			require.NotNil(t, prefilter)
			require.True(t, prefilter(tt.matching))
			require.False(t, prefilter(tt.nonMatching))
		})
	}
}

func Test_extractPatternsLiteralPrefilterReturnsEmptyMap(t *testing.T) {
	parser, err := ottl.NewParser[any](
		StandardConverters[any](),
		func(path ottl.Path[any]) (ottl.GetSetter[any], error) {
			require.Equal(t, "body", path.Name())
			return &ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "OTHER=abc123", nil
				},
				Setter: func(context.Context, any, any) error {
					return nil
				},
			}, nil
		},
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)

	expr, err := parser.ParseValueExpression(`ExtractPatterns(body, "GIT_SHA=(?P<git_sha>\\w+)")`)
	require.NoError(t, err)

	result, err := expr.Eval(t.Context(), nil)
	require.NoError(t, err)

	resultMap, ok := result.(pcommon.Map)
	require.True(t, ok)
	require.Equal(t, 0, resultMap.Len())
}
