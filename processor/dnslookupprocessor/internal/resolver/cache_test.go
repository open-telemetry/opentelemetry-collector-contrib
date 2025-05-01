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
	"go.uber.org/zap/zaptest"
)

func TestNewCacheResolver(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockResolver := new(MockResolver)
	mockResolver.On("Name").Return("mock_resolver")

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
				logger,
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

func TestCacheResolver_Resolve(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	tests := []struct {
		name             string
		hostname         string
		hitCacheSize     int
		missCacheSize    int
		cacheTTL         time.Duration
		setupMock        func(*MockResolver)
		expectedIP       string
		expectError      bool
		expectedError    error
		prePopulateCache map[string]string
		prePopulateMiss  map[string]struct{}
	}{
		{
			name:          "First lookup, cache miss",
			hostname:      "example.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *MockResolver) {
				m.On("Resolve", ctx, "example.com").Return("192.168.1.1", nil).Once()
			},
			expectedIP:  "192.168.1.1",
			expectError: false,
		},
		{
			name:          "Lookup with hit cache",
			hostname:      "cached.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(_ *MockResolver) {
			},
			expectedIP:  "10.0.0.1",
			expectError: false,
			prePopulateCache: map[string]string{
				"cached.com": "10.0.0.1",
			},
		},
		{
			name:          "Lookup with miss cache",
			hostname:      "notfound.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(_ *MockResolver) {
			},
			expectedIP:  "",
			expectError: false,
			prePopulateMiss: map[string]struct{}{
				"notfound.com": {},
			},
		},
		{
			name:          "Cache disabled, always goes to resolver",
			hostname:      "nocache.com",
			hitCacheSize:  0,
			missCacheSize: 0,
			cacheTTL:      0,
			setupMock: func(m *MockResolver) {
				m.On("Resolve", ctx, "nocache.com").Return("172.16.0.1", nil).Once()
			},
			expectedIP:  "172.16.0.1",
			expectError: false,
		},
		{
			name:          "Error from resolver",
			hostname:      "error.com",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *MockResolver) {
				m.On("Resolve", ctx, "error.com").Return("", errors.New("dns error")).Once()
			},
			expectedIP:    "",
			expectError:   true,
			expectedError: errors.New("dns error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResolver := new(MockResolver)
			tt.setupMock(mockResolver)

			resolver, err := NewCacheResolver(
				mockResolver,
				tt.hitCacheSize,
				tt.cacheTTL,
				tt.missCacheSize,
				tt.cacheTTL,
				logger,
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

			ip, err := resolver.Resolve(ctx, tt.hostname)

			mockResolver.AssertExpectations(t)

			if tt.expectError {
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

func TestCacheResolver_Reverse(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	tests := []struct {
		name             string
		ip               string
		hitCacheSize     int
		missCacheSize    int
		cacheTTL         time.Duration
		setupMock        func(*MockResolver)
		expectedHostname string
		expectError      bool
		expectedError    error
		prePopulateCache map[string]string
		prePopulateMiss  map[string]struct{}
	}{
		{
			name:          "First lookup, cache miss",
			ip:            "192.168.1.1",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *MockResolver) {
				m.On("Reverse", ctx, "192.168.1.1").Return("example.com", nil).Once()
			},
			expectedHostname: "example.com",
			expectError:      false,
		},
		{
			name:          "Lookup with hit cache",
			ip:            "10.0.0.1",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(_ *MockResolver) {
			},
			expectedHostname: "cached.com",
			expectError:      false,
			prePopulateCache: map[string]string{
				"10.0.0.1": "cached.com",
			},
		},
		{
			name:          "Lookup with miss cache",
			ip:            "1.1.1.1",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(_ *MockResolver) {
			},
			expectedHostname: "",
			expectError:      false,
			prePopulateMiss: map[string]struct{}{
				"1.1.1.1": {},
			},
		},
		{
			name:          "Cache disabled, always goes to resolver",
			ip:            "172.16.0.1",
			hitCacheSize:  0,
			missCacheSize: 0,
			cacheTTL:      0,
			setupMock: func(m *MockResolver) {
				m.On("Reverse", ctx, "172.16.0.1").Return("nocache.com", nil).Once()
			},
			expectedHostname: "nocache.com",
			expectError:      false,
		},
		{
			name:          "Error from resolver",
			ip:            "169.254.0.1",
			hitCacheSize:  10,
			missCacheSize: 10,
			cacheTTL:      0,
			setupMock: func(m *MockResolver) {
				m.On("Reverse", ctx, "169.254.0.1").Return("", errors.New("dns error")).Once()
			},
			expectedHostname: "",
			expectError:      true,
			expectedError:    errors.New("dns error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResolver := new(MockResolver)
			tt.setupMock(mockResolver)

			resolver, err := NewCacheResolver(
				mockResolver,
				tt.hitCacheSize,
				tt.cacheTTL,
				tt.missCacheSize,
				tt.cacheTTL,
				logger,
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

			assert.Equal(t, tt.expectedHostname, hostname)
		})
	}
}

func TestCacheResolver_Close(t *testing.T) {
	logger := zaptest.NewLogger(t)

	mockResolver := new(MockResolver)
	mockResolver.On("Close").Return(nil).Once()

	resolver, err := NewCacheResolver(
		mockResolver,
		10,
		0,
		10,
		0,
		logger,
	)
	require.NoError(t, err)

	// Add data to the caches
	resolver.hitCache.Add("test.com", "192.168.1.1")
	resolver.missCache.Add("nonexistent.com", struct{}{})

	// Close the resolver
	err = resolver.Close()
	assert.NoError(t, err)
	mockResolver.AssertExpectations(t)

	// Check that the caches are empty
	_, hitFound := resolver.hitCache.Get("test.com")
	assert.False(t, hitFound, "Hit cache should be empty after Close()")
	_, missFound := resolver.missCache.Get("nonexistent.com")
	assert.False(t, missFound, "Miss cache should be empty after Close()")
}
