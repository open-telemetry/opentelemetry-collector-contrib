// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package samplingstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCounterStore_AddMergesAdditively(t *testing.T) {
	m := NewMemoryCounterStore()
	ctx := t.Context()

	// Two writers (instances) into the same sampler and bucket.
	require.NoError(t, m.AddCounts(ctx, "rule-a", 100, map[string]float64{"svc-1": 10, "svc-2": 5}))
	require.NoError(t, m.AddCounts(ctx, "rule-a", 100, map[string]float64{"svc-1": 7, "svc-3": 2}))

	merged, err := m.ReadCounts(ctx, "rule-a", 100)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"svc-1": 17, "svc-2": 5, "svc-3": 2}, merged)
}

func TestMemoryCounterStore_BucketsAndSamplersAreIsolated(t *testing.T) {
	m := NewMemoryCounterStore()
	ctx := t.Context()

	require.NoError(t, m.AddCounts(ctx, "rule-a", 100, map[string]float64{"svc-1": 10}))
	require.NoError(t, m.AddCounts(ctx, "rule-a", 101, map[string]float64{"svc-1": 3}))
	require.NoError(t, m.AddCounts(ctx, "rule-b", 100, map[string]float64{"svc-1": 99}))

	merged, err := m.ReadCounts(ctx, "rule-a", 100)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"svc-1": 10}, merged)
	merged, err = m.ReadCounts(ctx, "rule-a", 101)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"svc-1": 3}, merged)
}

func TestMemoryCounterStore_UnwrittenBucketReadsEmpty(t *testing.T) {
	m := NewMemoryCounterStore()
	merged, err := m.ReadCounts(t.Context(), "rule-a", 42)
	require.NoError(t, err)
	assert.Empty(t, merged)
	assert.NotNil(t, merged)
}

func TestMemoryCounterStore_ReadReturnsCallerOwnedCopy(t *testing.T) {
	m := NewMemoryCounterStore()
	ctx := t.Context()
	require.NoError(t, m.AddCounts(ctx, "rule-a", 100, map[string]float64{"svc-1": 10}))

	first, err := m.ReadCounts(ctx, "rule-a", 100)
	require.NoError(t, err)
	first["svc-1"] = 0
	delete(first, "svc-1")

	second, err := m.ReadCounts(ctx, "rule-a", 100)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"svc-1": 10}, second, "mutating a read result must not affect the store")
}

func TestMemoryCounterStore_DoesNotRetainCallerMap(t *testing.T) {
	m := NewMemoryCounterStore()
	ctx := t.Context()
	counts := map[string]float64{"svc-1": 10}
	require.NoError(t, m.AddCounts(ctx, "rule-a", 100, counts))
	counts["svc-1"] = 999

	merged, err := m.ReadCounts(ctx, "rule-a", 100)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"svc-1": 10}, merged, "mutating the input map must not affect the store")
}

func TestMemoryCounterStore_PrunesOldBuckets(t *testing.T) {
	m := NewMemoryCounterStore()
	ctx := t.Context()
	require.NoError(t, m.AddCounts(ctx, "rule-a", 100, map[string]float64{"svc-1": 1}))
	require.NoError(t, m.AddCounts(ctx, "rule-a", 100+retainedBuckets, map[string]float64{"svc-1": 1}))

	merged, err := m.ReadCounts(ctx, "rule-a", 100)
	require.NoError(t, err)
	assert.Empty(t, merged, "buckets older than the retention margin are dropped")
}
