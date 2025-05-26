// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Luhn(t *testing.T) {
	noErrorTests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "valid number string",
			value:    "17893729974",
			expected: true,
		},
		{
			name:     "valid number string with spaces",
			value:    "1789 3729 974",
			expected: true,
		},
		{
			name:     "empty string",
			value:    "",
			expected: false,
		},
		{
			name:     "single digit",
			value:    "0",
			expected: true,
		},
		{
			name:     "valid number",
			value:    17893729974,
			expected: true,
		},
		{
			name:     "invalid number string",
			value:    "17893729975",
			expected: false,
		},
	}

	for _, tt := range noErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isValidLuhnFunc[any](&ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}

	errorTests := []struct {
		name     string
		value    any
		errorStr string
	}{
		{
			name:     "not a number string",
			value:    "test",
			errorStr: "invalid",
		},
		{
			name:     "false",
			value:    false,
			errorStr: "invalid",
		},
		{
			name:     "float values are not allowed",
			value:    30.3,
			errorStr: "invalid",
		},
		{
			name:     "nil",
			value:    nil,
			errorStr: "invalid",
		},
		{
			name:     "some struct",
			value:    struct{}{},
			errorStr: "invalid",
		},
	}
	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isValidLuhnFunc[any](&ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.errorStr)
			assert.Nil(t, result)
		})
	}
}
