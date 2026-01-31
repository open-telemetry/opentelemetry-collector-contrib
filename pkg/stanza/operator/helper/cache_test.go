// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cacheFactory struct {
	name string
	make func(uint16) Cache
}

var cacheFactories = []cacheFactory{
	{name: "memory", make: func(size uint16) Cache { return NewMemoryCache(size, 0) }},
	{name: "syncmap", make: func(size uint16) Cache { return NewSyncMapCache(size, 0) }},
}

func TestNewMemoryCache(t *testing.T) {
	cases := []struct {
		name       string
		maxSize    uint16
		expectSize int
	}{
		{"size-50", 50, 50},
		{"size-1024", 1024, 1024},
	}

	for _, factory := range cacheFactories {
		for _, tc := range cases {
			t.Run(factory.name+"-"+tc.name, func(t *testing.T) {
				output := factory.make(tc.maxSize)
				defer output.Stop()
				require.Equal(t, tc.expectSize, int(output.MaxSize()), "keys channel should have cap of expected size")
			})
		}
	}
}

func TestMemoryCacheGetAndAdd(t *testing.T) {
	input := map[string]any{
		"key": "value",
		"map-value": map[string]any{
			"x":   "y",
			"dev": "stanza",
		},
	}

	for _, factory := range cacheFactories {
		t.Run(factory.name, func(t *testing.T) {
			c := factory.make(3)
			defer c.Stop()
			for key, value := range input {
				added := c.Add(key, value)
				require.True(t, added, "expected add to return true")
				out := c.Get(key)
				require.NotNil(t, out, "expected to get value from cache immediately after adding it")
				require.Equal(t, value, out, "expected value to equal the value that was added to the cache")
			}

			for expectKey, expectItem := range input {
				actual := c.Get(expectKey)
				require.NotNil(t, actual)
				require.Equal(t, expectItem, actual)
			}
		})
	}
}

// A full cache should replace the oldest element with the new element.
func TestMemoryCacheFIFOEviction(t *testing.T) {
	maxSize := 10

	for _, factory := range cacheFactories {
		t.Run(factory.name, func(t *testing.T) {
			c := factory.make(uint16(maxSize))
			defer c.Stop()

			for i := 0; i <= int(c.MaxSize()); i++ {
				str := strconv.Itoa(i)
				c.Add(str, i)
			}

			for i := 11; i <= 20; i++ {
				str := strconv.Itoa(i)
				c.Add(str, i)

				removedKey := strconv.Itoa(i - 10)
				x := c.Get(removedKey)
				require.Nil(t, x, "expected key %s to have been removed", removedKey)
			}
		})
	}
}

func TestMemoryCacheCopy(t *testing.T) {
	for _, factory := range cacheFactories {
		t.Run(factory.name, func(t *testing.T) {
			m := factory.make(5)
			defer m.Stop()

			m.Add("key1", "value1")
			m.Add("key2", "value2")
			m.Add("key3", "value3")

			copy := m.Copy()
			require.Len(t, copy, 3)
			require.Equal(t, "value1", copy["key1"])
			require.Equal(t, "value2", copy["key2"])
			require.Equal(t, "value3", copy["key3"])

			m.Add("key4", "value4")
			require.Len(t, copy, 3, "copy should not be affected by changes to original")
		})
	}
}

func TestMemoryCacheMaxSize(t *testing.T) {
	for _, factory := range cacheFactories {
		t.Run(factory.name, func(t *testing.T) {
			m := factory.make(100)
			defer m.Stop()
			require.Equal(t, uint16(100), m.MaxSize())
		})
	}
}

func TestNewStartedAtomicLimiter(t *testing.T) {
	cases := []struct {
		name     string
		max      uint64
		interval uint64
	}{
		{"default", 0, 0},
		{"max", 30, 0},
		{"interval", 0, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newStartedAtomicLimiter(tc.max, tc.interval)
			require.Equal(t, tc.max, l.max)
			defer l.stop()
			if tc.interval == 0 {
				tc.interval = 5
			}
			require.Equal(t, float64(tc.interval), l.interval.Seconds())
			require.Equal(t, uint64(0), l.currentCount())
		})
	}
}

// Start a limiter with a max of 3 and ensure throttling begins.
func TestLimiter(t *testing.T) {
	const maxVal = uint64(3)

	l := newStartedAtomicLimiter(maxVal, 120)
	require.NotNil(t, l)
	require.Equal(t, maxVal, l.max)
	defer l.stop()

	require.False(t, l.throttled(), "new limiter should not be throttling")
	require.Equal(t, uint64(0), l.currentCount())

	var i uint64
	for i = 1; i < maxVal; i++ {
		l.increment()
		require.Equal(t, i, l.currentCount())
		require.False(t, l.throttled())
	}

	l.increment()
	require.True(t, l.throttled())
}

func TestThrottledLimiter(t *testing.T) {
	const maxVal = uint64(3)

	count := &atomic.Uint64{}
	count.Add(maxVal + 1)
	l := atomicLimiter{
		max:      maxVal,
		count:    count,
		interval: 1,
		done:     make(chan struct{}),
	}

	require.True(t, l.throttled())

	l.init()
	defer l.stop()
	wait := 2 * l.interval
	time.Sleep(time.Second * wait)
	require.False(t, l.throttled())
	require.Equal(t, uint64(0), l.currentCount())
}

func TestThrottledCache(t *testing.T) {
	for _, factory := range cacheFactories {
		t.Run(factory.name, func(t *testing.T) {
			c := factory.make(3)
			defer c.Stop()
			for i := 1; i <= 8; i++ {
				key := strconv.Itoa(i)
				value := i
				_ = c.Add(key, value)
			}
		})
	}
}

func TestMemoryCacheConcurrentAccess(t *testing.T) {
	for _, factory := range cacheFactories {
		t.Run(factory.name, func(t *testing.T) {
			m := factory.make(100)
			defer m.Stop()

			var wg sync.WaitGroup
			numGoroutines := 10
			numOps := 100

			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for j := 0; j < numOps; j++ {
						key := strconv.Itoa(id*numOps + j)
						m.Add(key, key+"_value")
					}
				}(i)
			}

			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for j := 0; j < numOps; j++ {
						key := strconv.Itoa(id*numOps + j)
						_ = m.Get(key)
					}
				}(i)
			}

			wg.Wait()
		})
	}
}
