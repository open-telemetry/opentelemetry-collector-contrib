// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestStartsWith(t *testing.T) {
	type testCase struct {
		name          string
		value         any
		prefixes      []string
		caseSensitive bool
		want          bool
		expectErr     bool
	}

	emptyStr := ""
	testStr := "this is a test string"
	tests := []testCase{
		{
			name:          "nil value",
			value:         nil,
			prefixes:      []string{"this"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "empty string",
			value:         emptyStr,
			prefixes:      []string{"this"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "empty prefixes",
			value:         testStr,
			prefixes:      []string{},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "single val match",
			value:         testStr,
			prefixes:      []string{"this"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "single val no match",
			value:         testStr,
			prefixes:      []string{"xyz"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "two prefixes first match",
			value:         testStr,
			prefixes:      []string{"this", "xyz"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "two prefixes second match",
			value:         testStr,
			prefixes:      []string{"xyz", "this"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "two prefixes no match",
			value:         testStr,
			prefixes:      []string{"xyz", "abc"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "case sensitivity default",
			value:         testStr,
			prefixes:      []string{"THIS"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "partial match",
			value:         testStr,
			prefixes:      []string{"th"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "prefix longer than string",
			value:         testStr,
			prefixes:      []string{"this is a test string with extra words"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "not a prefix",
			value:         testStr,
			prefixes:      []string{"test"},
			caseSensitive: true,
			want:          false,
		},
		// Case insensitive tests
		{
			name:          "case insensitive match uppercase prefix",
			value:         testStr,
			prefixes:      []string{"THIS"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive match mixed case prefix",
			value:         testStr,
			prefixes:      []string{"ThIs"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive no match",
			value:         testStr,
			prefixes:      []string{"XYZ"},
			caseSensitive: false,
			want:          false,
		},
		{
			name:          "case insensitive multiple prefixes",
			value:         testStr,
			prefixes:      []string{"abc", "THIS", "xyz"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive partial match",
			value:         testStr,
			prefixes:      []string{"TH"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "uppercase string with lowercase prefix",
			value:         "THIS IS A TEST STRING",
			prefixes:      []string{"this"},
			caseSensitive: false,
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, err := createStartsWithFunction[any](
				ottl.FunctionContext{},
				&StartsWithArguments[any]{
					Target: &ottl.StandardStringGetter[any]{
						Getter: func(context.Context, any) (any, error) {
							return tt.value, nil
						},
					},
					Prefixes:      tt.prefixes,
					CaseSensitive: tt.caseSensitive,
				},
			)

			require.NoError(t, err)

			result, err := expressionFunc(t.Context(), nil)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, result)
		})
	}
}
