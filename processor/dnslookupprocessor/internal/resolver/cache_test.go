// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/testutil"
)

func TestNewCacheResolver(t *testing.T) {
	mockResolver := new(testutil.MockResolver)

	tests := []struct {
		name            string
		nextResolver    Resolver
		hitCacheSize    int
		hitCacheTTL     time.Duration
		missCacheSize   int
		missCacheTTL    time.Duration
		expectError     bool
		expectHitCache  bool
		expectMissCache bool
	}{
		{
			name:            "Valid configuration with both caches",
			nextResolver:    mockResolver,
			hitCacheSize:    100,
			hitCacheTTL:     0,
			missCacheSize:   50,
			missCacheTTL:    0,
			expectError:     false,
			expectHitCache:  true,
			expectMissCache: true,
		},
		{
			name:            "Valid configuration with only hit cache",
			nextResolver:    mockResolver,
			hitCacheSize:    100,
			hitCacheTTL:     0,
			missCacheSize:   0,
			missCacheTTL:    0,
			expectError:     false,
			expectHitCache:  true,
			expectMissCache: false,
		},
		{
			name:            "Valid configuration with only miss cache",
			nextResolver:    mockResolver,
			hitCacheSize:    0,
			hitCacheTTL:     0,
			missCacheSize:   50,
			missCacheTTL:    0,
			expectError:     false,
			expectHitCache:  false,
			expectMissCache: true,
		},
		{
			name:            "Valid configuration with no caches",
			nextResolver:    mockResolver,
			hitCacheSize:    0,
			hitCacheTTL:     0,
			missCacheSize:   0,
			missCacheTTL:    0,
			expectError:     false,
			expectHitCache:  false,
			expectMissCache: false,
		},
		{
			name:            "No next resolver",
			nextResolver:    nil,
			hitCacheSize:    100,
			hitCacheTTL:     0,
			missCacheSize:   50,
			missCacheTTL:    0,
			expectError:     true,
			expectHitCache:  false,
			expectMissCache: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewCacheResolver(
				tt.nextResolver,
				tt.hitCacheSize,
				tt.hitCacheTTL,
				tt.missCacheSize,
				tt.missCacheTTL,
			)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resolver)
			} else {
				assert.NoError(t, err)

				if tt.expectHitCache {
					assert.NotNil(t, resolver.hitCache)
				} else {
					assert.Nil(t, resolver.hitCache)
				}

				if tt.expectMissCache {
					assert.NotNil(t, resolver.missCache)
				} else {
					assert.Nil(t, resolver.missCache)
				}
			}
		})
	}
}

func TestCacheResolver_resolveWithCache(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                  string
		hostname              string
		hitCacheSize          int
		missCacheSize         int
		cacheTTL              time.Duration
		setupMock             func(*testutil.MockResolver)
		expectedIP            []string
		expectError           bool
		expectedError         error
		prePopulateCache      map[string][]string
		prePopulateMiss       map[string]struct{}
		expectedHitCacheSize  int
		expectedMissCacheSize int
	}{
		{
			name:          "First lookup miss. Add to hit cache",
			hostname:      "example.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *testutil.MockResolver) {
				m.On("Resolve", ctx, "example.com").Return([]string{"192.168.1.1"}, nil).Once()
			},
			expectedIP:            []string{"192.168.1.1"},
			expectError:           false,
			expectedHitCacheSize:  1,
			expectedMissCacheSize: 0,
		},
		{
			name:          "Lookup with hit cache",
			hostname:      "cached.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(_ *testutil.MockResolver) {
			},
			prePopulateCache: map[string][]string{
				"cached.com": {"10.0.0.1"},
			},
			expectedIP:            []string{"10.0.0.1"},
			expectError:           false,
			expectedHitCacheSize:  1,
			expectedMissCacheSize: 0,
		},
		{
			name:          "Lookup with miss cache",
			hostname:      "notfound.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(_ *testutil.MockResolver) {
			},
			prePopulateMiss: map[string]struct{}{
				"notfound.com": {},
			},
			expectedIP:            nil,
			expectError:           false,
			expectedHitCacheSize:  0,
			expectedMissCacheSize: 1,
		},
		{
			name:          "Cache disabled, always goes to resolver",
			hostname:      "nocache.com",
			hitCacheSize:  0,
			missCacheSize: 0,
			cacheTTL:      0,
			setupMock: func(m *testutil.MockResolver) {
				m.On("Resolve", ctx, "nocache.com").Return([]string{"172.16.0.1"}, nil).Once()
			},
			expectedIP:            []string{"172.16.0.1"},
			expectError:           false,
			expectedHitCacheSize:  0,
			expectedMissCacheSize: 0,
		},
		{
			name:          "No resolution and no error. Add to miss cache",
			hostname:      "somewhere.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *testutil.MockResolver) {
				m.On("Resolve", ctx, "somewhere.com").Return(nil, nil).Once()
			},
			expectedIP:            nil,
			expectError:           false,
			expectedHitCacheSize:  0,
			expectedMissCacheSize: 1,
		},
		{
			name:          "Empty slice. Add to miss cache",
			hostname:      "somewhere.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *testutil.MockResolver) {
				m.On("Resolve", ctx, "somewhere.com").Return([]string{}, nil).Once()
			},
			expectedIP:            nil,
			expectError:           false,
			expectedHitCacheSize:  0,
			expectedMissCacheSize: 1,
		},
		{
			name:          "Error from resolver is populated. No cache update",
			hostname:      "error.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *testutil.MockResolver) {
				m.On("Resolve", ctx, "error.com").Return(nil, errors.New("dns error")).Once()
			},
			expectedIP:            nil,
			expectError:           true,
			expectedError:         errors.New("dns error"),
			expectedHitCacheSize:  0,
			expectedMissCacheSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResolver := new(testutil.MockResolver)
			tt.setupMock(mockResolver)

			resolver, err := NewCacheResolver(
				mockResolver,
				tt.hitCacheSize,
				tt.cacheTTL,
				tt.missCacheSize,
				tt.cacheTTL,
			)
			require.NoError(t, err)

			// Pre-populate hit cache
			if tt.prePopulateCache != nil && resolver.hitCache != nil {
				for k, v := range tt.prePopulateCache {
					resolver.hitCache.Add(k, v)
				}
			}

			// Pre-populate miss cache
			if tt.prePopulateMiss != nil && resolver.missCache != nil {
				for k := range tt.prePopulateMiss {
					resolver.missCache.Add(k, struct{}{})
				}
			}

			ips, err := resolver.Resolve(ctx, tt.hostname)

			mockResolver.AssertExpectations(t)

			if tt.expectError {
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.ElementsMatch(t, tt.expectedIP, ips)

			if tt.hitCacheSize > 0 {
				assert.Equal(t, tt.expectedHitCacheSize, resolver.hitCache.Len())
			}

			if tt.missCacheSize > 0 {
				assert.Equal(t, tt.expectedMissCacheSize, resolver.missCache.Len())
			}
		})
	}
}

func TestCacheResolver_Reverse(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                  string
		ip                    string
		hitCacheSize          int
		missCacheSize         int
		cacheTTL              time.Duration
		setupMock             func(*testutil.MockResolver)
		prePopulateCache      map[string][]string
		prePopulateMiss       map[string]struct{}
		expectedHostnames     []string
		expectError           bool
		expectedError         error
		expectedHitCacheSize  int
		expectedMissCacheSize int
	}{
		{
			name:          "First lookup miss. Add to hit cache",
			ip:            "192.168.1.1",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *testutil.MockResolver) {
				m.On("Reverse", ctx, "192.168.1.1").Return([]string{"example.com"}, nil).Once()
			},
			expectedHostnames:     []string{"example.com"},
			expectError:           false,
			expectedHitCacheSize:  1,
			expectedMissCacheSize: 0,
		},
		{
			name:          "Lookup with hit cache",
			ip:            "10.0.0.1",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(_ *testutil.MockResolver) {
			},
			prePopulateCache: map[string][]string{
				"10.0.0.1": {"cached.com"},
			},
			expectedHostnames:     []string{"cached.com"},
			expectError:           false,
			expectedHitCacheSize:  1,
			expectedMissCacheSize: 0,
		},
		{
			name:          "Lookup with miss cache",
			ip:            "1.1.1.1",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(_ *testutil.MockResolver) {
			},
			prePopulateMiss: map[string]struct{}{
				"1.1.1.1": {},
			},
			expectedHostnames:     nil,
			expectError:           false,
			expectedHitCacheSize:  0,
			expectedMissCacheSize: 1,
		},
		{
			name:          "Error from resolver",
			ip:            "169.254.0.1",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *testutil.MockResolver) {
				m.On("Reverse", ctx, "169.254.0.1").Return(nil, errors.New("dns error")).Once()
			},
			expectedHostnames:     nil,
			expectError:           true,
			expectedError:         errors.New("dns error"),
			expectedHitCacheSize:  0,
			expectedMissCacheSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResolver := new(testutil.MockResolver)
			tt.setupMock(mockResolver)

			resolver, err := NewCacheResolver(
				mockResolver,
				tt.hitCacheSize,
				tt.cacheTTL,
				tt.missCacheSize,
				tt.cacheTTL,
			)
			require.NoError(t, err)

			// Pre-populate hit cache
			if tt.prePopulateCache != nil && resolver.hitCache != nil {
				for k, v := range tt.prePopulateCache {
					resolver.hitCache.Add(k, v)
				}
			}

			// Pre-populate miss cache
			if tt.prePopulateMiss != nil && resolver.missCache != nil {
				for k := range tt.prePopulateMiss {
					resolver.missCache.Add(k, struct{}{})
				}
			}

			hostname, err := resolver.Reverse(ctx, tt.ip)

			mockResolver.AssertExpectations(t)

			if tt.expectError {
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.ElementsMatch(t, tt.expectedHostnames, hostname)

			if tt.hitCacheSize > 0 {
				assert.Equal(t, tt.expectedHitCacheSize, resolver.hitCache.Len())
			}

			if tt.missCacheSize > 0 {
				assert.Equal(t, tt.expectedMissCacheSize, resolver.missCache.Len())
			}
		})
	}
}
