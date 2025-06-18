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
			name:     "First resolver returns ErrNoResolution, chain stops",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(testutil.MockResolver)
				r1.On("Resolve", ctx, "example.com").Return(nil, ErrNoResolution)

				// ErrNoResolution is a valid result that no need to try next resolver
				// Second resolver should not be called
				r2 := new(testutil.MockResolver)

				return []Resolver{r1, r2}
			},
			expectedIPs:   nil,
			expectError:   true,
			expectedError: ErrNoResolution,
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
			name:     "ErrNotInHostFiles should try the next resolver",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(testutil.MockResolver)
				r1.On("Resolve", ctx, "example.com").Return(nil, ErrNotInHostFiles)

				// Second resolver should be called
				r2 := new(testutil.MockResolver)
				r2.On("Resolve", ctx, "example.com").Return([]string{"192.168.1.20"}, nil)

				return []Resolver{r1, r2}
			},
			expectedIPs: []string{"192.168.1.20"},
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

func TestChainResolver_Close(t *testing.T) {
	t.Run("Successful close", func(t *testing.T) {
		mock1 := new(testutil.MockResolver)
		mock1.On("Close").Return(nil).Once()

		mock2 := new(testutil.MockResolver)
		mock2.On("Close").Return(nil).Once()

		chainResolver := NewChainResolver([]Resolver{mock1, mock2})

		err := chainResolver.Close()
		assert.NoError(t, err)

		mock1.AssertExpectations(t)
		mock2.AssertExpectations(t)
	})

	t.Run("Close with errors", func(t *testing.T) {
		errorMock1 := new(testutil.MockResolver)
		errorMock1.On("Close").Return(errors.New("error 1")).Once()

		errorMock2 := new(testutil.MockResolver)
		errorMock2.On("Close").Return(errors.New("error 2")).Once()

		chainResolver := NewChainResolver([]Resolver{errorMock1, errorMock2})

		err := chainResolver.Close()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error 1")
		assert.Contains(t, err.Error(), "error 2")

		errorMock1.AssertExpectations(t)
		errorMock2.AssertExpectations(t)
	})
}
