// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFloat64RateCalculator(t *testing.T) {
	mKey := NewKey("rate", nil)
	initTime := time.Now()
	c := newFloat64RateCalculator()
	r, ok := c.Calculate(mKey, float64(50), initTime)
	assert.False(t, ok)
	assert.Equal(t, float64(0), r)

	nextTime := initTime.Add(100 * time.Millisecond)
	r, ok = c.Calculate(mKey, float64(100), nextTime)
	assert.True(t, ok)
	assert.InDelta(t, 0.5, r, 0.1)
	require.NoError(t, c.Shutdown())
}

func TestFloat64RateCalculatorWithTooFrequentUpdate(t *testing.T) {
	mKey := NewKey("rate", nil)
	initTime := time.Now()
	c := newFloat64RateCalculator()
	r, ok := c.Calculate(mKey, float64(50), initTime)
	assert.False(t, ok)
	assert.Equal(t, float64(0), r)

	nextTime := initTime
	for i := 0; i < 10; i++ {
		nextTime = nextTime.Add(5 * time.Millisecond)
		r, ok = c.Calculate(mKey, float64(105), nextTime)
		assert.False(t, ok)
		assert.Equal(t, float64(0), r)
	}

	nextTime = nextTime.Add(5 * time.Millisecond)
	r, ok = c.Calculate(mKey, float64(105), nextTime)
	assert.True(t, ok)
	assert.InDelta(t, 1, r, 0.1)
	require.NoError(t, c.Shutdown())
}

func newFloat64RateCalculator() MetricCalculator {
	return NewMetricCalculator(func(prev *MetricValue, val any, timestampMs time.Time) (any, bool) {
		if prev != nil {
			deltaTimestampMs := timestampMs.Sub(prev.Timestamp).Milliseconds()
			deltaValue := val.(float64) - prev.RawValue.(float64)
			if deltaTimestampMs > 50*time.Millisecond.Milliseconds() && deltaValue >= 0 {
				return deltaValue / float64(deltaTimestampMs), true
			}
		}
		return float64(0), false
	})
}

func TestFloat64DeltaCalculator(t *testing.T) {
	mKey := NewKey("delta", nil)
	initTime := time.Now()
	c := NewFloat64DeltaCalculator()

	testCases := []float64{0.1, 0.1, 0.5, 1.3, 1.9, 2.5, 5, 24.2, 103}
	for i, f := range testCases {
		r, ok := c.Calculate(mKey, f, initTime)
		assert.Equal(t, i > 0, ok)
		if i == 0 {
			assert.Equal(t, float64(0), r)
		} else {
			assert.InDelta(t, f-testCases[i-1], r, f/10)
		}
	}
	require.NoError(t, c.Shutdown())
}

func TestFloat64DeltaCalculatorWithDecreasingValues(t *testing.T) {
	mKey := NewKey("delta", nil)
	initTime := time.Now()
	c := NewFloat64DeltaCalculator()

	testCases := []float64{108, 106, 56.2, 28.8, 10, 10, 3, -1, -100}
	for i, f := range testCases {
		r, ok := c.Calculate(mKey, f, initTime)
		assert.Equal(t, i > 0, ok)
		if ok {
			assert.Equal(t, testCases[i]-testCases[i-1], r)
		}
	}
	require.NoError(t, c.Shutdown())
}

func TestMapWithExpiryAdd(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	value1 := rand.Float64()
	store.Lock()
	store.Set(Key{MetricMetadata: "key1"}, MetricValue{RawValue: value1})
	val, ok := store.Get(Key{MetricMetadata: "key1"})
	store.Unlock()
	assert.True(t, ok)
	assert.Equal(t, value1, val.RawValue)

	store.Lock()
	defer store.Unlock()
	val, ok = store.Get(Key{MetricMetadata: "key2"})
	assert.False(t, ok)
	assert.Nil(t, val)
	require.NoError(t, store.Shutdown())
}

func TestMapWithExpiryCleanup(t *testing.T) {
	// This test is meant to explicitly test the CleanUp method. We do not need to use NewMapWithExpiry().
	// Instead, manually create a Map Object, sleep, and then call cleanup to ensure that entries are erased.
	// The sweep method is tested in a later unit test.
	// Explicitly testing CleanUp allows us to avoid test race conditions when the sweep ticker may not fire within
	// the allotted sleep time.
	store := &MapWithExpiry{
		ttl:     time.Millisecond,
		entries: make(map[any]*MetricValue),
		lock:    &sync.Mutex{},
	}
	value1 := rand.Float64()
	store.Lock()
	store.Set(Key{MetricMetadata: "key1"}, MetricValue{RawValue: value1, Timestamp: time.Now()})

	val, ok := store.Get(Key{MetricMetadata: "key1"})

	assert.True(t, ok)
	assert.Equal(t, value1, val.RawValue.(float64))
	assert.Equal(t, 1, store.Size())
	store.Unlock()

	time.Sleep(time.Millisecond * 2)
	store.CleanUp(time.Now())
	store.Lock()
	val, ok = store.Get(Key{MetricMetadata: "key1"})
	assert.False(t, ok)
	assert.Nil(t, val)
	assert.Equal(t, 0, store.Size())
	store.Unlock()
}

func TestMapWithExpiryConcurrency(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	store.Lock()
	store.Set(Key{MetricMetadata: "sum"}, MetricValue{RawValue: 0})
	store.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for i := 0; i < 30; i++ {
			store.Lock()
			sum, _ := store.Get(Key{MetricMetadata: "sum"})
			newSum := MetricValue{
				RawValue: sum.RawValue.(int) + 1,
			}
			store.Set(Key{MetricMetadata: "sum"}, newSum)
			store.Unlock()
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 30; i++ {
			store.Lock()
			sum, _ := store.Get(Key{MetricMetadata: "sum"})
			newSum := MetricValue{
				RawValue: sum.RawValue.(int) - 1,
			}
			store.Set(Key{MetricMetadata: "sum"}, newSum)
			store.Unlock()
		}
		wg.Done()
	}()
	wg.Wait()
	sum, _ := store.Get(Key{MetricMetadata: "sum"})
	assert.Equal(t, 0, sum.RawValue.(int))
	require.NoError(t, store.Shutdown())
}

type mockKey struct {
	name  string
	index int64
}

func TestMapKeyEquals(t *testing.T) {
	labelMap1 := make(map[string]string)
	labelMap1["k1"] = "v1"
	labelMap1["k2"] = "v2"

	labelMap2 := make(map[string]string)
	labelMap2["k2"] = "v2"
	labelMap2["k1"] = "v1"

	mKey1 := NewKey("name", labelMap1)
	mKey2 := NewKey("name", labelMap2)
	assert.Equal(t, mKey1, mKey2)

	mKey1 = NewKey(mockKey{
		name:  "name",
		index: 1,
	}, labelMap1)
	mKey2 = NewKey(mockKey{
		name:  "name",
		index: 1,
	}, labelMap2)
	assert.Equal(t, mKey1, mKey2)
}

func TestMapKeyNotEqualOnName(t *testing.T) {
	labelMap1 := make(map[string]string)
	labelMap1["k1"] = "v1"
	labelMap1["k2"] = "v2"

	labelMap2 := make(map[string]string)
	labelMap2["k2"] = "v2"
	labelMap2["k1"] = "v1"

	mKey1 := NewKey("name1", labelMap1)
	mKey2 := NewKey("name2", labelMap2)
	assert.NotEqual(t, mKey1, mKey2)

	mKey1 = NewKey(mockKey{
		name:  "name",
		index: 1,
	}, labelMap1)
	mKey2 = NewKey(mockKey{
		name:  "name",
		index: 2,
	}, labelMap2)
	assert.NotEqual(t, mKey1, mKey2)

	mKey2 = NewKey(mockKey{
		name:  "name0",
		index: 1,
	}, labelMap2)
	assert.NotEqual(t, mKey1, mKey2)
}

func TestSweep(t *testing.T) {
	sweepEvent := make(chan time.Time)
	closed := &atomic.Bool{}

	onSweep := func(now time.Time) {
		sweepEvent <- now
	}

	mwe := &MapWithExpiry{
		ttl:      1 * time.Millisecond,
		lock:     &sync.Mutex{},
		doneChan: make(chan struct{}),
	}

	start := time.Now()
	go func() {
		mwe.sweep(onSweep)
		closed.Store(true)
		close(sweepEvent)
	}()

	for i := 1; i <= 2; i++ {
		sweepTime := <-sweepEvent
		tickTime := time.Since(start) + mwe.ttl*time.Duration(i)
		require.False(t, closed.Load())
		assert.LessOrEqual(t, mwe.ttl, tickTime)
		assert.LessOrEqual(t, time.Since(sweepTime), mwe.ttl)
	}
	require.NoError(t, mwe.Shutdown())
	for range sweepEvent { // nolint
	}
	assert.True(t, closed.Load(), "Sweeper did not terminate.")
}
