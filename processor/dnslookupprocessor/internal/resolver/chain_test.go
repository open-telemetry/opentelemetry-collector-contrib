// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/testutil"
)

func TestNewChainResolver(t *testing.T) {
	tests := []struct {
		name      string
		resolvers []Resolver
	}{
		{
			name:      "Empty resolvers",
			resolvers: []Resolver{},
		},
		{
			name: "Single resolver",
			resolvers: []Resolver{
				&testutil.MockResolver{},
			},
		},
		{
			name: "Multiple resolvers",
			resolvers: []Resolver{
				&testutil.MockResolver{},
				&testutil.MockResolver{},
				&testutil.MockResolver{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chainResolver := NewChainResolver(tt.resolvers)
			assert.NotNil(t, chainResolver)
			assert.Len(t, tt.resolvers, len(chainResolver.resolvers))
		})
	}
}

func TestChainResolver_resolveInSequence(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		hostname       string
		setupResolvers func() []Resolver
		expectedIPs    []string
		expectError    bool
		expectedError  error
	}{
		{
			name:     "First resolver succeeds",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(testutil.MockResolver)
				r1.On("Resolve", ctx, "example.com").Return([]string{"192.168.1.10"}, nil)

				// Second resolver should not be called
				r2 := new(testutil.MockResolver)

				return []Resolver{r1, r2}
			},
			expectedIPs: []string{"192.168.1.10"},
			expectError: false,
		},
		{
			name:     "First resolver fails with an error, second succeeds",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(testutil.MockResolver)
				r1.On("Resolve", ctx, "example.com").Return(nil, errors.New("first resolver error"))

				r2 := new(testutil.MockResolver)
				r2.On("Resolve", ctx, "example.com").Return([]string{"192.168.1.20"}, nil)

				return []Resolver{r1, r2}
			},
			expectedIPs: []string{"192.168.1.20"},
			expectError: false,
		},
		{
			name:     "First resolver returns [], chain stops",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(testutil.MockResolver)
				r1.On("Resolve", ctx, "example.com").Return([]string{}, nil)

				// Empty slice is a valid result that no need to try the next resolver
				// Second resolver should not be called
				r2 := new(testutil.MockResolver)

				return []Resolver{r1, r2}
			},
			expectedIPs: []string{},
			expectError: false,
		},
		{
			name:     "All resolvers fail, populate last error",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(testutil.MockResolver)
				r1.On("Resolve", ctx, "example.com").Return(nil, errors.New("first resolver error"))

				r2 := new(testutil.MockResolver)
				r2.On("Resolve", ctx, "example.com").Return(nil, errors.New("second resolver error"))

				return []Resolver{r1, r2}
			},
			expectedIPs:   nil,
			expectError:   true,
			expectedError: errors.New("second resolver error"),
		},
		{
			name:     "no resolution and no error should try the next resolver",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(testutil.MockResolver)
				r1.On("Resolve", ctx, "example.com").Return(nil, nil)

				// Second resolver should be called
				r2 := new(testutil.MockResolver)
				r2.On("Resolve", ctx, "example.com").Return([]string{"192.168.1.20"}, nil)

				return []Resolver{r1, r2}
			},
			expectedIPs: []string{"192.168.1.20"},
			expectError: false,
		},
		{
			name:     "All resolvers give no resolution and no error",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(testutil.MockResolver)
				r1.On("Resolve", ctx, "example.com").Return(nil, nil)

				// Second resolver should be called
				r2 := new(testutil.MockResolver)
				r2.On("Resolve", ctx, "example.com").Return(nil, nil)

				return []Resolver{r1, r2}
			},
			expectedIPs: nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvers := tt.setupResolvers()
			chainResolver := NewChainResolver(resolvers)

			ips, err := chainResolver.Resolve(ctx, tt.hostname)

			// Verify resolver mock expectations
			for _, r := range resolvers {
				mockResolver, ok := r.(*testutil.MockResolver)
				if ok {
					mockResolver.AssertExpectations(t)
				}
			}

			if tt.expectError {
				assert.Equal(t, tt.expectedError, err)
				assert.Empty(t, ips)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.expectedIPs, ips)
			}
		})
	}
}
