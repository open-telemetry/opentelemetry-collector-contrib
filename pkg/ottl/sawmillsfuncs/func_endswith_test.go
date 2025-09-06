// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestEndsWith(t *testing.T) {
	type testCase struct {
		name          string
		value         any
		suffixes      []string
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
			suffixes:      []string{"string"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "empty string",
			value:         emptyStr,
			suffixes:      []string{"string"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "empty suffixes",
			value:         testStr,
			suffixes:      []string{},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "single val match",
			value:         testStr,
			suffixes:      []string{"string"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "single val no match",
			value:         testStr,
			suffixes:      []string{"xyz"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "two suffixes first match",
			value:         testStr,
			suffixes:      []string{"string", "xyz"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "two suffixes second match",
			value:         testStr,
			suffixes:      []string{"xyz", "string"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "two suffixes no match",
			value:         testStr,
			suffixes:      []string{"xyz", "abc"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "case sensitivity default",
			value:         testStr,
			suffixes:      []string{"STRING"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "partial match",
			value:         testStr,
			suffixes:      []string{"ing"},
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "suffix longer than string",
			value:         testStr,
			suffixes:      []string{"extra words this is a test string"},
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "not a suffix",
			value:         testStr,
			suffixes:      []string{"this"},
			caseSensitive: true,
			want:          false,
		},
		// Case insensitive tests
		{
			name:          "case insensitive match uppercase suffix",
			value:         testStr,
			suffixes:      []string{"STRING"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive match mixed case suffix",
			value:         testStr,
			suffixes:      []string{"StRiNg"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive no match",
			value:         testStr,
			suffixes:      []string{"XYZ"},
			caseSensitive: false,
			want:          false,
		},
		{
			name:          "case insensitive multiple suffixes",
			value:         testStr,
			suffixes:      []string{"abc", "STRING", "xyz"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive partial match",
			value:         testStr,
			suffixes:      []string{"ING"},
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "uppercase string with lowercase suffix",
			value:         "THIS IS A TEST STRING",
			suffixes:      []string{"string"},
			caseSensitive: false,
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, err := createEndsWithFunction[any](
				ottl.FunctionContext{},
				&EndsWithArguments[any]{
					Target: &ottl.StandardStringGetter[any]{
						Getter: func(context.Context, any) (any, error) {
							return tt.value, nil
						},
					},
					Suffixes:      tt.suffixes,
					CaseSensitive: tt.caseSensitive,
				},
			)

			require.NoError(t, err)

			result, err := expressionFunc(context.Background(), nil)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, result)
		})
	}
}
