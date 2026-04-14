// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingProvider signals when Retrieve is called and blocks until released.
// Used to verify that the lock is not held during the HTTP call.
type blockingProvider struct {
	started chan struct{}
	release chan struct{}
}

func (p *blockingProvider) Retrieve(_ context.Context, key string) (string, error) {
	p.started <- struct{}{}
	<-p.release
	return key, nil
}

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

// TestCacheableProviderConcurrentFetch verifies that the lock is not held during the
// HTTP call. If it were, only one goroutine could be inside provider.Retrieve at a time.
// The test uses a blockingProvider that signals when it has been entered and then blocks.
// With correct behavior, two goroutines can both be inside the HTTP call simultaneously.
func TestCacheableProviderConcurrentFetch(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})

	cp := NewCacheableProvider(&blockingProvider{started: started, release: release}, time.Minute, 10)

	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			_, _ = cp.Retrieve(t.Context(), "key")
		})
	}

	// Both goroutines must reach the HTTP call concurrently.
	// If the mutex were held during the HTTP call, the second goroutine would block
	// on acquiring the lock and never reach provider.Retrieve until the first completes.
	for range 2 {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("goroutine blocked waiting for lock — mutex is held during HTTP call")
		}
	}

	close(release)
	wg.Wait()

	// After both complete, the result should be cached.
	v, err := cp.Retrieve(t.Context(), "key")
	assert.NoError(t, err)
	assert.Equal(t, "key", v)
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
