// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_isInCIDR(t *testing.T) {
	tests := []struct {
		name     string
		target   any
		networks []string
		result   any
	}{
		{
			name:     "a included IP string",
			target:   "192.0.2.1",
			networks: []string{"192.0.24.1/24", "192.0.2.1/24"},
			result:   true,
		},
		{
			name:     "a not included IP string",
			target:   "195.0.2.1",
			networks: []string{"192.0.2.1/24"},
			result:   false,
		},
		{
			name:     "non IP string",
			target:   "hello world",
			networks: []string{"192.0.2.1/24"},
			result:   false,
		},
		{
			name:     "empty string",
			target:   "",
			networks: []string{"192.0.2.1/24"},
			result:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isInCIDR[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) { return tt.target, nil },
			},
				tt.networks)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.result, result)
		})
	}
}

func Test_isInCIDR_Error(t *testing.T) {
	tests := []struct {
		name          string
		target        any
		networks      []string
		result        any
		err           bool
		expectedError string
	}{
		{
			name:          "non-string",
			target:        10,
			networks:      []string{"192.0.2.1/24"},
			expectedError: "expected string but got int",
		},
		{
			name:          "Addresses is not a valid address",
			target:        "192.0.0.1",
			networks:      []string{"192.0.2/24"},
			expectedError: "invalid CIDR address: 192.0.2/24",
		},
		{
			name:          "nil",
			target:        nil,
			networks:      []string{"192.0.2.1/24"},
			expectedError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isInCIDR[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) { return tt.target, nil },
			},
				tt.networks)
			_, err := exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
