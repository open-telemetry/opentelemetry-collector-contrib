// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestContains(t *testing.T) {
	type testCase struct {
		name          string
		value         any
		patterns      []string
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
			patterns:      []string{"test"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "empty string",
			value:         emptyStr,
			patterns:      []string{"test"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "empty patterns",
			value:         testStr,
			patterns:      []string{},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "single val match",
			value:         testStr,
			patterns:      []string{"test"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "single val no match",
			value:         testStr,
			patterns:      []string{"xyz"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "two patterns first match",
			value:         testStr,
			patterns:      []string{"test", "xyz"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "two patterns second match",
			value:         testStr,
			patterns:      []string{"xyz", "test"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "two patterns no match",
			value:         testStr,
			patterns:      []string{"xyz", "abc"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "three patterns first match",
			value:         testStr,
			patterns:      []string{"test", "xyz", "abc"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "three patterns second match",
			value:         testStr,
			patterns:      []string{"xyz", "test", "abc"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "three patterns third match",
			value:         testStr,
			patterns:      []string{"xyz", "abc", "test"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "three patterns no match",
			value:         testStr,
			patterns:      []string{"xyz", "abc", "def"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "case sensitivity default",
			value:         testStr,
			patterns:      []string{"TEST"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "partial match at beginning",
			value:         testStr,
			patterns:      []string{"this"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "partial match in middle",
			value:         testStr,
			patterns:      []string{"is a"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "partial match at end",
			value:         testStr,
			patterns:      []string{"string"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "multiple matches",
			value:         testStr,
			patterns:      []string{"this", "test", "string"},
			caseSensitive: true,
			want:          true,
		},
		// Case insensitive tests
		{
			name:          "case insensitive match uppercase pattern",
			value:         testStr,
			patterns:      []string{"TEST"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive match mixed case pattern",
			value:         testStr,
			patterns:      []string{"TeSt"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive no match",
			value:         testStr,
			patterns:      []string{"XYZ"},
			caseSensitive: false,
			want:          false,
		},
		{
			name:          "case insensitive multiple patterns",
			value:         testStr,
			patterns:      []string{"ABC", "TEST", "XYZ"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive beginning match",
			value:         testStr,
			patterns:      []string{"THIS"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive middle match",
			value:         testStr,
			patterns:      []string{"IS A"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive end match",
			value:         testStr,
			patterns:      []string{"STRING"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "uppercase string with lowercase patterns",
			value:         "THIS IS A TEST STRING",
			patterns:      []string{"test"},
			caseSensitive: false,
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, err := createContainsFunction[any](
				ottl.FunctionContext{},
				&ContainsArguments[any]{
					Target: &ottl.StandardStringGetter[any]{
						Getter: func(context.Context, any) (any, error) {
							return tt.value, nil
						},
					},
					Patterns:      tt.patterns,
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
