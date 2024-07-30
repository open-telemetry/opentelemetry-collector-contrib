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
		expected    any
		createError string
		funcError   string
	}{
		{
			name: "string in v32_hash",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
				Version: ottl.NewTestingOptional[string]("v32_hash"),
			},
			expected: int64(427197390),
		},
		{
			name: "string in v128_hash",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
			},
			expected: []int64{int64(1901405986810282715), int64(-8942425033498643417)},
		},
		{
			name: "empty string",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "", nil
					},
				},
				Version: ottl.NewTestingOptional[string]("v128_hex"),
			},
			expected: "00000000000000000000000000000000",
		},
		{
			name: "string in v128_hex",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
				Version: ottl.NewTestingOptional[string]("v128_hex"),
			},
			expected: "dbc2a0c1ab26631a27b4c09fcf1fe683",
		},
		{
			name: "string in v32_hex",
			oArgs: &MurmurHash3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
				Version: ottl.NewTestingOptional[string]("v32_hex"),
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
