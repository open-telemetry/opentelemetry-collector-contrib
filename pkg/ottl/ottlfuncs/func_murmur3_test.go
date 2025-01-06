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

func Test_Murmur3(t *testing.T) {
	tests := []struct {
		name      string
		oArgs     *Murmur3Arguments[any]
		variant   murmur3Variant
		expected  any
		funcError string
	}{
		{
			name: "string in Murmur3Hash",
			oArgs: &Murmur3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
			},
			variant:  Murmur3Sum32,
			expected: int64(427197390),
		},
		{
			name: "string in Murmur3Hash128",
			oArgs: &Murmur3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
			},
			variant:  Murmur3Sum128,
			expected: []int64{int64(1901405986810282715), int64(-8942425033498643417)},
		},
		{
			name: "empty string",
			oArgs: &Murmur3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "", nil
					},
				},
			},
			variant:  Murmur3Hex128,
			expected: "00000000000000000000000000000000",
		},
		{
			name: "string in Murmur3Hex128",
			oArgs: &Murmur3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
			},
			variant:  Murmur3Hex128,
			expected: "dbc2a0c1ab26631a27b4c09fcf1fe683",
		},
		{
			name: "string in Murmur3Hex",
			oArgs: &Murmur3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
			},
			variant:  Murmur3Hex32,
			expected: "ce837619",
		},
		{
			name: "invalid variant",
			oArgs: &Murmur3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "Hello World", nil
					},
				},
			},
			variant:   66,
			funcError: "unknown murmur3 variant: 66",
		},
		{
			name: "non-string",
			oArgs: &Murmur3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return 10, nil
					},
				},
			},
			variant:   Murmur3Sum32,
			funcError: "expected string but got int",
		},
		{
			name: "nil",
			oArgs: &Murmur3Arguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return nil, nil
					},
				},
			},
			variant:   Murmur3Sum32,
			funcError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := createMurmur3Function[any](tt.oArgs, tt.variant)
			assert.NoError(t, err)
			result, err := exprFunc(context.Background(), nil)
			if tt.funcError != "" {
				require.ErrorContains(t, err, tt.funcError)
				return
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}
