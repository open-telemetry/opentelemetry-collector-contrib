// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type firstErrorProvider struct {
	cnt int
	mu  sync.Mutex
}

func (p *firstErrorProvider) Retrieve(_ context.Context, key string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cnt++
	if p.cnt == 1 {
		return "", errors.New("first error")
	}
	p.cnt = 0
	return key, nil
}

// alwaysErrorProvider always returns an error and counts how many times Retrieve was called.
type alwaysErrorProvider struct {
	calls int
}

func (p *alwaysErrorProvider) Retrieve(_ context.Context, _ string) (string, error) {
	p.calls++
	return "", errors.New("always fails")
}

// TestCacheableProviderRateLimit verifies that after the call limit is reached and the
// cooldown expires, the very next failed call triggers a new cooldown immediately rather
// than allowing another burst of `limit` calls through.
func TestCacheableProviderRateLimit(t *testing.T) {
	provider := &alwaysErrorProvider{}
	cp := NewCacheableProvider(provider, time.Hour, 3).(*CacheableProvider)

	// Exhaust the limit: 3 calls should reach the provider.
	for range 3 {
		_, _ = cp.Retrieve(t.Context(), "key")
	}
	require.Equal(t, 3, provider.calls, "all 3 calls should reach the provider")

	// Next call should be rate-limited (cooldown is 1 hour).
	_, err := cp.Retrieve(t.Context(), "key")
	require.ErrorContains(t, err, "rate limited")
	require.Equal(t, 3, provider.calls, "rate-limited call should not reach provider")

	// Simulate cooldown expiry.
	cp.resetTime = time.Now().Add(-time.Second)

	// After cooldown expires, one retry should be allowed.
	_, err = cp.Retrieve(t.Context(), "key")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "rate limited", "first call after cooldown should reach provider")
	require.Equal(t, 4, provider.calls, "one retry after cooldown")

	// The next call should be rate-limited again immediately — not allowed another
	// burst of `limit` calls.
	_, err = cp.Retrieve(t.Context(), "key")
	require.ErrorContains(t, err, "rate limited",
		"second call after cooldown should be rate-limited, not allowed through")
}

func TestCacheableProvider(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		retry int
	}{
		{
			name:  "limit 1",
			limit: 1,
			retry: 4,
		},
		{
			name:  "limit 0",
			limit: 0,
			retry: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new cacheable provider
			provider := NewCacheableProvider(&firstErrorProvider{}, 0*time.Nanosecond, tt.limit)

			var p string
			var err error
			for i := 0; i < tt.retry; i++ {
				p, err = provider.Retrieve(t.Context(), "key")
				if err == nil {
					break
				}
				require.Error(t, err, "first error")
			}
			require.NoError(t, err, "no error")
			require.Equal(t, "key", p, "value is key")
		})
	}
}
