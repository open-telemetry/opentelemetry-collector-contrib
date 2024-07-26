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

func Test_MurmurHash3(t *testing.T) {
	tests := []struct {
		name        string
		oArgs       ottl.Arguments
		expected    string
		createError string
		funcError   string
	}{
		{
			name: "string",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
			},
			expected: "dbc2a0c1ab26631a27b4c09fcf1fe683",
		},
		{
			name: "empty string",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "", nil
					},
				},
				Version: ottl.NewTestingOptional[string]("128"),
			},
			expected: "00000000000000000000000000000000",
		},
		{
			name: "string in v128",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
				Version: ottl.NewTestingOptional[string]("128"),
			},
			expected: "dbc2a0c1ab26631a27b4c09fcf1fe683",
		},
		{
			name: "string in v32",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
				Version: ottl.NewTestingOptional[string]("32"),
			},
			expected: "ce837619",
		},
		{
			name: "invalid version",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
				Version: ottl.NewTestingOptional[string]("66"),
			},
			createError: "invalid arguments: 66",
		},
		{
			name: "non-string",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return 10, nil
					},
				},
			},
			funcError: "expected string but got int",
		},
		{
			name: "nil",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return nil, nil
					},
				},
			},
			funcError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := createMurmurHash3Function[any](ottl.FunctionContext{}, tt.oArgs)
			if tt.createError != "" {
				require.ErrorContains(t, err, tt.createError)
				return
			}

			assert.NoError(t, err)

			result, err := exprFunc(nil, nil)
			if tt.funcError != "" {
				require.ErrorContains(t, err, tt.funcError)
				return
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}
