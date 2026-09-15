// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package samplingstate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/samplingstate"

import (
	"context"
	"maps"
	"sync"
)

// retainedBuckets bounds how many interval buckets MemoryCounterStore keeps per sampler.
// The sync loop reads the bucket it just wrote, so anything older is garbage;
// a small margin tolerates a reader that lags a tick.
const retainedBuckets = 3

// MemoryCounterStore is the in-process Store used when no extension is configured. It
// gives a single instance the exact per-instance behavior (this instance's
// counts are the merged counts) through the same code path the shared
// backends use.
type MemoryCounterStore struct {
	mu sync.Mutex
	// buckets maps samplerID -> bucket index -> merged counts.
	buckets map[string]map[int64]map[string]float64
}

var _ CounterStore = (*MemoryCounterStore)(nil)

// NewMemoryCounterStore returns an empty in-process store.
func NewMemoryCounterStore() *MemoryCounterStore {
	return &MemoryCounterStore{buckets: make(map[string]map[int64]map[string]float64)}
}

// AddCounts implements Store.
func (m *MemoryCounterStore) AddCounts(_ context.Context, samplerID string, bucket int64, counts map[string]float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	perSampler := m.buckets[samplerID]
	if perSampler == nil {
		perSampler = make(map[int64]map[string]float64)
		m.buckets[samplerID] = perSampler
	}
	merged := perSampler[bucket]
	if merged == nil {
		merged = make(map[string]float64, len(counts))
		perSampler[bucket] = merged
	}
	for k, v := range counts {
		merged[k] += v
	}
	for b := range perSampler {
		if b <= bucket-retainedBuckets {
			delete(perSampler, b)
		}
	}
	return nil
}

// ReadCounts implements Store.
func (m *MemoryCounterStore) ReadCounts(_ context.Context, samplerID string, bucket int64) (map[string]float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	merged := m.buckets[samplerID][bucket]
	out := make(map[string]float64, len(merged))
	maps.Copy(out, merged)
	return out, nil
}
