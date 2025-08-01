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
			name:     "simple path",
			value:    "/attach",
			expected: "/attach",
		},
		{
			name:     "simple numeric path",
			value:    "/123",
			expected: "/*",
		},
		{
			name:     "trailing numeric path",
			value:    "123/",
			expected: "*/",
		},
		{
			name:     "numeric with text path",
			value:    "123/ljgdflgjf",
			expected: "*/*",
		},
		{
			name:     "wildcard path",
			value:    "/**",
			expected: "/*",
		},
		{
			name:     "simple user path",
			value:    "/u/2",
			expected: "/u/*",
		},
		{
			name:     "versioned product path",
			value:    "/v1/products/2",
			expected: "/v1/products/*",
		},
		{
			name:     "versioned product path with longer id",
			value:    "/v1/products/22",
			expected: "/v1/products/*",
		},
		{
			name:     "versioned product path with alphanumeric",
			value:    "/v1/products/22j",
			expected: "/v1/products/*",
		},
		{
			name:     "multiple numeric ids",
			value:    "/products/1/org/3",
			expected: "/products/*/org/*",
		},
		{
			name:     "path with empty segment",
			value:    "/products//org/3",
			expected: "/products//org/*",
		},
		{
			name:     "k6 test runs path",
			value:    "/v1/k6-test-runs/1",
			expected: "/v1/k6-test-runs/*",
		},
		{
			name:     "complex user path spanish",
			value:    "/usuarios/fdklsd/j4elk/23993/trabajo/2",
			expected: "/usuarios/*/j4elk/*/trabajo/*",
		},
		{
			name:     "complex user path german",
			value:    "/Benutzer/fdklsd/j4elk/23993/Arbeit/2",
			expected: "/Benutzer/*/j4elk/*/Arbeit/*",
		},
		{
			name:     "complex user path french",
			value:    "/utilisateurs/fdklsd/j4elk/23993/tache/2",
			expected: "/utilisateurs/*/j4elk/*/tache/*",
		},
		{
			name:     "trailing slash",
			value:    "/products/",
			expected: "/products/",
		},
		{
			name:     "hyphenated path",
			value:    "/user-space/",
			expected: "/user-space/",
		},
		{
			name:     "underscored path",
			value:    "/user_space/",
			expected: "/user_space/",
		},
		{
			name:     "dotted path",
			value:    "/api/hello.world",
			expected: "/api/hello.world",
		},
		{
			name:     "multiple dots path",
			value:    "/api/hello.world.again",
			expected: "/api/hello.world.again",
		},
		{
			name:     "dotted segment path",
			value:    "/api.backup/hello.world",
			expected: "/api.backup/hello.world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := SanitizeURL[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			}, &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "*", nil
				},
			})
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_SanitizeURLWithCustomReplacement(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		replacement string
		expected    any
	}{
		{
			name:        "custom replacement single char",
			value:       "/products/123",
			replacement: "X",
			expected:    "/products/X",
		},
		{
			name:        "custom replacement multiple chars",
			value:       "/users/456/orders/789",
			replacement: "[ID]",
			expected:    "/users/[ID]/orders/[ID]",
		},
		{
			name:        "custom replacement with emoji",
			value:       "/api/abc123def",
			replacement: "🔢",
			expected:    "/api/🔢",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := SanitizeURL[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			}, &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.replacement, nil
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
			}, &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "*", nil
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
