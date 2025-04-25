// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

// MockResolver implements the Resolver interface for testing
type MockResolver struct {
	mock.Mock
}

func (m *MockResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	args := m.Called(ctx, hostname)
	return args.String(0), args.Error(1)
}

func (m *MockResolver) Reverse(ctx context.Context, ip string) (string, error) {
	args := m.Called(ctx, ip)
	return args.String(0), args.Error(1)
}

func (m *MockResolver) Name() string {
	args := m.Called()
	return args.String(0)
}

func TestNewChainResolver(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name      string
		resolvers []Resolver
	}{
		{
			name:      "Empty netResolvers",
			resolvers: []Resolver{},
		},
		{
			name: "Single resolver",
			resolvers: []Resolver{
				&MockResolver{},
			},
		},
		{
			name: "Multiple netResolvers",
			resolvers: []Resolver{
				&MockResolver{},
				&MockResolver{},
				&MockResolver{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chainResolver := NewChainResolver(tt.resolvers, logger)
			assert.NotNil(t, chainResolver)
			assert.Equal(t, "chain", chainResolver.Name())
			assert.Equal(t, len(tt.resolvers), len(chainResolver.resolvers))
		})
	}
}

func TestChainResolver_Resolve(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	tests := []struct {
		name           string
		hostname       string
		setupResolvers func() []Resolver
		expectedIP     string
		expectError    bool
		expectedError  error
	}{
		{
			name:     "First resolver succeeds",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Name").Return("mock1")
				r1.On("Resolve", ctx, "example.com").Return("192.168.1.10", nil)

				// Second resolver should not be called
				r2 := new(MockResolver)

				return []Resolver{r1, r2}
			},
			expectedIP:  "192.168.1.10",
			expectError: false,
		},
		{
			name:     "First resolver fails with a standard error, second succeeds",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Resolve", ctx, "example.com").Return("", errors.New("first resolver error"))

				r2 := new(MockResolver)
				r2.On("Name").Return("mock2")
				r2.On("Resolve", ctx, "example.com").Return("192.168.1.20", nil)

				return []Resolver{r1, r2}
			},
			expectedIP:  "192.168.1.20",
			expectError: false,
		},
		{
			name:     "First resolver returns ErrNoResolution, chain stops",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Name").Return("mock1")
				r1.On("Resolve", ctx, "example.com").Return("", ErrNoResolution)

				// ErrNoResolution is treated as success
				// Second resolver should not be called
				r2 := new(MockResolver)

				return []Resolver{r1, r2}
			},
			expectedIP:  "",
			expectError: false,
		},
		{
			name:     "All netResolvers fail",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Resolve", ctx, "example.com").Return("", errors.New("first resolver error"))

				r2 := new(MockResolver)
				r2.On("Resolve", ctx, "example.com").Return("", errors.New("second resolver error"))

				return []Resolver{r1, r2}
			},
			expectedIP:    "",
			expectError:   true,
			expectedError: errors.New("second resolver error"),
		},
		{
			name:     "ErrNotInHostFiles should continue to next resolver",
			hostname: "example.com",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Resolve", ctx, "example.com").Return("", ErrNotInHostFiles)

				// Second resolver should be called
				r2 := new(MockResolver)
				r2.On("Name").Return("mock2")
				r2.On("Resolve", ctx, "example.com").Return("192.168.1.20", nil)

				return []Resolver{r1, r2}
			},
			expectedIP:  "192.168.1.20",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvers := tt.setupResolvers()
			chainResolver := NewChainResolver(resolvers, logger)

			ip, err := chainResolver.Resolve(ctx, tt.hostname)

			// Verify resolver mock expectations
			for _, r := range resolvers {
				mockResolver, ok := r.(*MockResolver)
				if ok {
					mockResolver.AssertExpectations(t)
				}
			}

			if tt.expectError {
				assert.Equal(t, tt.expectedError, err)
				assert.Empty(t, ip)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIP, ip)
			}
		})
	}
}

func TestChainResolver_Reverse(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	tests := []struct {
		name             string
		ip               string
		setupResolvers   func() []Resolver
		expectedHostname string
		expectError      bool
		expectedError    error
	}{
		{
			name: "First resolver succeeds",
			ip:   "192.168.1.10",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Name").Return("mock1")
				r1.On("Reverse", ctx, "192.168.1.10").Return("example.com", nil)

				// Second resolver should not be called
				r2 := new(MockResolver)

				return []Resolver{r1, r2}
			},
			expectedHostname: "example.com",
			expectError:      false,
		},
		{
			name: "First resolver fails with standard error, second succeeds",
			ip:   "192.168.1.10",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Reverse", ctx, "192.168.1.10").Return("", errors.New("first resolver error"))

				r2 := new(MockResolver)
				r2.On("Name").Return("mock2")
				r2.On("Reverse", ctx, "192.168.1.10").Return("example.com", nil)

				return []Resolver{r1, r2}
			},
			expectedHostname: "example.com",
			expectError:      false,
		},
		{
			name: "First resolver returns ErrNoResolution, chain stops",
			ip:   "192.168.1.10",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Name").Return("mock1")
				r1.On("Reverse", ctx, "192.168.1.10").Return("", ErrNoResolution)

				// ErrNoResolution is treated as success
				// Second resolver should not be called
				r2 := new(MockResolver)

				return []Resolver{r1, r2}
			},
			expectedHostname: "",
			expectError:      false,
		},
		{
			name: "All resolvers fail",
			ip:   "192.168.1.10",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Reverse", ctx, "192.168.1.10").Return("", errors.New("first resolver error"))

				r2 := new(MockResolver)
				r2.On("Reverse", ctx, "192.168.1.10").Return("", errors.New("second resolver error"))

				return []Resolver{r1, r2}
			},
			expectedHostname: "",
			expectError:      true,
			expectedError:    errors.New("second resolver error"),
		},
		{
			name: "ErrNotInHostFiles should continue to next resolver",
			ip:   "192.168.1.10",
			setupResolvers: func() []Resolver {
				r1 := new(MockResolver)
				r1.On("Reverse", ctx, "192.168.1.10").Return("", ErrNotInHostFiles)

				// Second resolver should be called since
				r2 := new(MockResolver)
				r2.On("Name").Return("mock2")
				r2.On("Reverse", ctx, "192.168.1.10").Return("example.com", nil)

				return []Resolver{r1, r2}
			},
			expectedHostname: "example.com",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvers := tt.setupResolvers()
			chainResolver := NewChainResolver(resolvers, logger)

			hostname, err := chainResolver.Reverse(ctx, tt.ip)

			// Verify resolver mock expectations
			for _, r := range resolvers {
				mockResolver, ok := r.(*MockResolver)
				if ok {
					mockResolver.AssertExpectations(t)
				}
			}

			if tt.expectError {
				assert.Equal(t, tt.expectedError, err)
				assert.Empty(t, hostname)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedHostname, hostname)
			}
		})
	}
}
