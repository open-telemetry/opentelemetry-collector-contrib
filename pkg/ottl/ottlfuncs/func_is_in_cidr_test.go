// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_isInCIDR(t *testing.T) {
	tests := []struct {
		name     string
		target   any
		networks ottl.StringLikeSliceGetter[any]
		result   any
	}{
		{
			name:   "an included IP string",
			target: "192.0.2.1",
			networks: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"192.0.24.0/24", "192.0.2.0/24"}, nil
				},
			},
			result: true,
		},
		{
			name:   "a not included IP string",
			target: "195.0.2.1",
			networks: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"192.0.2.0/24"}, nil
				},
			},
			result: false,
		},
		{
			name:   "non IP string",
			target: "hello world",
			networks: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"192.0.2.0/24"}, nil
				},
			},
			result: false,
		},
		{
			name:   "empty string",
			target: "",
			networks: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"192.0.2.0/24"}, nil
				},
			},
			result: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) { return tt.target, nil },
			}, tt.networks)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.result, result)
		})
	}
}

func Test_isInCIDR_Error(t *testing.T) {
	tests := []struct {
		name          string
		target        any
		networks      ottl.StringLikeSliceGetter[any]
		result        any
		err           bool
		expectedError string
	}{
		{
			name:   "non-string",
			target: 10,
			networks: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"192.0.2.0/24"}, nil
				},
			},
			expectedError: "expected string but got int",
		},
		{
			name:   "dynamic network is not a valid CIDR",
			target: "192.0.0.1",
			networks: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"192.0.2/24"}, nil
				},
			},
			expectedError: "invalid CIDR address: 192.0.2/24",
		},
		{
			name:   "nil",
			target: nil,
			networks: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"192.0.2.0/24"}, nil
				},
			},
			expectedError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) { return tt.target, nil },
			}, tt.networks)
			require.NoError(t, err)
			_, err = exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Test_isInCIDR_literalNetworks(t *testing.T) {
	literalNetworks := func(networks ...string) ottl.StringLikeSliceGetter[any] {
		g, err := ottl.NewTestingLiteralGetter[any, []string](true, ottl.StandardStringLikeSliceGetter[any]{
			Getter: func(context.Context, any) (any, error) { return networks, nil },
		})
		require.NoError(t, err)
		return g
	}

	t.Run("single literal network", func(t *testing.T) {
		exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "10.1.2.3", nil },
		}, literalNetworks("10.0.0.0/8"))
		require.NoError(t, err)
		result, err := exprFunc(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("multiple literals networks", func(t *testing.T) {
		exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.168.0.1", nil },
		}, literalNetworks("10.0.0.0/8", "192.168.0.0/16"))
		require.NoError(t, err)
		result, err := exprFunc(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("invalid literal network", func(t *testing.T) {
		_, err := isInCIDR[any](ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.0.1", nil },
		}, literalNetworks("192.0.2/24"))
		assert.ErrorContains(t, err, "invalid CIDR address")
	})
}
