// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_SanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
	}{
		{
			name:     "simple path",
			value:    "/api/users",
			expected: "/api/users",
		},
		{
			name:     "path with integer ID",
			value:    "/api/users/12345",
			expected: "/api/users/{int}",
		},
		{
			name:     "path with object ID",
			value:    "/api/items/507f1f77bcf86cd799439011",
			expected: "/api/items/{objectId}",
		},
		{
			name:     "path with UUID",
			value:    "/api/sessions/550e8400-e29b-41d4-a716-446655440000",
			expected: "/api/sessions/{uuid}",
		},
		{
			name:     "path with token",
			value:    "/files/download/XyZABcDeFg1234",
			expected: "/files/download/{token}",
		},
		{
			name:     "multiple IDs in path",
			value:    "/api/users/12345/posts/67890",
			expected: "/api/users/{int}/posts/{int}",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "",
		},
		{
			name:     "single slash",
			value:    "/",
			expected: "/",
		},
		{
			name:     "mixed patterns",
			value:    "/api/users/12345/items/507f1f77bcf86cd799439011",
			expected: "/api/users/{int}/items/{objectId}",
		},
		{
			name:     "pattern already in path",
			value:    "/api/users/{int}",
			expected: "/api/users/{int}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := SanitizeURL[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_SanitizeURLError(t *testing.T) {
	tests := []struct {
		name          string
		value         any
		expectedError string
	}{
		{
			name:          "non-string",
			value:         123,
			expectedError: "expected string but got int",
		},
		{
			name:          "nil",
			value:         nil,
			expectedError: "expected string but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := SanitizeURL[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			assert.NoError(t, err)
			_, err = exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Test_SanitizeURLFactory(t *testing.T) {
	tests := []struct {
		name          string
		args          ottl.Arguments
		expectedError string
	}{
		{
			name:          "invalid args",
			args:          &AppendArguments[any]{},
			expectedError: "SanitizeURLFactory args must be of type *SanitizeURLArguments[K]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := createSanitizeURLFunction[any](ottl.FunctionContext{}, tt.args)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
