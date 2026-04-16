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
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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

type staticProvider struct{ value string }

func (p *staticProvider) Retrieve(_ context.Context, _ string) (string, error) {
	return p.value, nil
}

func collectSum(t *testing.T, reader sdkmetric.Reader, name string) int64 {
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			var total int64
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
			return total
		}
	}
	return 0
}

func TestCacheableProviderCounters(t *testing.T) {
	tests := []struct {
		name       string
		retrievals []string
		wantHits   int64
		wantMisses int64
	}{
		{
			name:       "first retrieval is a miss",
			retrievals: []string{"key"},
			wantHits:   0,
			wantMisses: 1,
		},
		{
			name:       "second retrieval for same key is a hit",
			retrievals: []string{"key", "key"},
			wantHits:   1,
			wantMisses: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := sdkmetric.NewManualReader()
			mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			t.Cleanup(func() { assert.NoError(t, mp.Shutdown(t.Context())) })

			meter := mp.Meter("test")
			hitCounter, err := meter.Int64Counter("cache.hits")
			require.NoError(t, err)
			missCounter, err := meter.Int64Counter("cache.misses")
			require.NoError(t, err)

			cp := NewCacheableProvider(&staticProvider{value: "result"}, 0, 5).(*CacheableProvider)
			cp.hitCounter = hitCounter
			cp.missCounter = missCounter

			for _, key := range tt.retrievals {
				_, err := cp.Retrieve(t.Context(), key)
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantHits, collectSum(t, reader, "cache.hits"))
			assert.Equal(t, tt.wantMisses, collectSum(t, reader, "cache.misses"))
		})
	}
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
