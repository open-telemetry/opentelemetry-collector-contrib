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
		networks []ottl.StringGetter[any]
		result   any
	}{
		{
			name:   "an included IP string",
			target: "192.0.2.1",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.24.0/24", nil },
				},
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			result: true,
		},
		{
			name:   "a not included IP string",
			target: "195.0.2.1",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			result: false,
		},
		{
			name:   "non IP string",
			target: "hello world",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			result: false,
		},
		{
			name:   "empty string",
			target: "",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
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
		networks      []ottl.StringGetter[any]
		result        any
		err           bool
		expectedError string
	}{
		{
			name:   "non-string",
			target: 10,
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
				},
			},
			expectedError: "expected string but got int",
		},
		{
			name:   "dynamic network is not a valid CIDR",
			target: "192.0.0.1",
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2/24", nil },
				},
			},
			expectedError: "invalid CIDR address: 192.0.2/24",
		},
		{
			name:   "nil",
			target: nil,
			networks: []ottl.StringGetter[any]{
				ottl.StandardStringGetter[any]{
					Getter: func(context.Context, any) (any, error) { return "192.0.2.0/24", nil },
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
	literalOne, err := ottl.NewTestingLiteralGetter(true, ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) { return "10.0.0.0/8", nil },
	})
	require.NoError(t, err)
	literalTwo, err := ottl.NewTestingLiteralGetter(true, ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) { return "192.168.0.0/16", nil },
	})
	require.NoError(t, err)

	t.Run("single literal network", func(t *testing.T) {
		exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "10.1.2.3", nil },
		}, []ottl.StringGetter[any]{literalOne})
		require.NoError(t, err)
		result, err := exprFunc(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("multiple literals networks", func(t *testing.T) {
		exprFunc, err := isInCIDR[any](ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.168.0.1", nil },
		}, []ottl.StringGetter[any]{literalOne, literalTwo})
		require.NoError(t, err)
		result, err := exprFunc(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("invalid literal network", func(t *testing.T) {
		invalidLiteral, err := ottl.NewTestingLiteralGetter(true, ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.2/24", nil },
		})
		require.NoError(t, err)

		_, err = isInCIDR[any](ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return "192.0.0.1", nil },
		}, []ottl.StringGetter[any]{invalidLiteral})
		assert.ErrorContains(t, err, "invalid CIDR address")
	})
}
