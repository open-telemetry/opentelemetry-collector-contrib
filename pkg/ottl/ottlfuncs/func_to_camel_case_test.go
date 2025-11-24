// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_toCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[any]
		expected any
	}{
		{
			name: "simple",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "simple_string", nil
				},
			},
			expected: "SimpleString",
		},
		{
			name: "already camel",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "SimpleString", nil
				},
			},
			expected: "SimpleString",
		},
		{
			name: "hyphens",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "simple-string", nil
				},
			},
			expected: "SimpleString",
		},
		{
			name: "empty string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "", nil
				},
			},
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := toCamelCase(tt.target)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_toCamelCaseRuntimeError(t *testing.T) {
	tests := []struct {
		name          string
		target        ottl.StringGetter[any]
		expectedError string
	}{
		{
			name: "non-string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return 10, nil
				},
			},
			expectedError: "expected string but got int",
		},
		{
			name: "nil",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return nil, nil
				},
			},
			expectedError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := toCamelCase[any](tt.target)
			_, err := exprFunc(t.Context(), nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
