// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
}

func newFloat64RateCalculator() MetricCalculator {
	return NewMetricCalculator(func(prev *MetricValue, val interface{}, timestampMs time.Time) (interface{}, bool) {
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
}

func TestMapWithExpiryAdd(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	value1 := rand.Float64()
	store.Set(Key{MetricMetadata: "key1"}, MetricValue{RawValue: value1})
	val, ok := store.Get(Key{MetricMetadata: "key1"})
	assert.Equal(t, true, ok)
	assert.Equal(t, value1, val.RawValue)

	val, ok = store.Get(Key{MetricMetadata: "key2"})
	assert.Equal(t, false, ok)
	assert.True(t, val == nil)
}

func TestMapWithExpiryCleanup(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	value1 := rand.Float64()
	store.Set(Key{MetricMetadata: "key1"}, MetricValue{RawValue: value1, Timestamp: time.Now()})

	store.CleanUp(time.Now())
	val, ok := store.Get(Key{MetricMetadata: "key1"})
	assert.Equal(t, true, ok)
	assert.Equal(t, value1, val.RawValue.(float64))
	assert.Equal(t, 1, store.Size())

	time.Sleep(time.Second)
	store.CleanUp(time.Now())
	val, ok = store.Get(Key{MetricMetadata: "key1"})
	assert.Equal(t, false, ok)
	assert.True(t, val == nil)
	assert.Equal(t, 0, store.Size())
}

func TestMapWithExpiryConcurrency(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	store.Set(Key{MetricMetadata: "sum"}, MetricValue{RawValue: 0})

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
