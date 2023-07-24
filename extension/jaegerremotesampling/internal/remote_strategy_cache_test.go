// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/jaegertracing/jaeger/thrift-gen/sampling"
	"github.com/stretchr/testify/assert"
)

const cacheTestItemTTL = 50 * time.Millisecond

var testStrategyResponseA = &sampling.SamplingStrategyResponse{
	ProbabilisticSampling: &sampling.ProbabilisticSamplingStrategy{
		SamplingRate: 0.1337,
	},
}

var testStrategyResponseB = &sampling.SamplingStrategyResponse{
	OperationSampling: &sampling.PerOperationSamplingStrategies{
		DefaultSamplingProbability: 0.001,
		PerOperationStrategies: []*sampling.OperationSamplingStrategy{
			{
				Operation: "always-sampled-op",
				ProbabilisticSampling: &sampling.ProbabilisticSamplingStrategy{
					SamplingRate: 1.0,
				},
			},
			{
				Operation: "never-sampled-op",
				ProbabilisticSampling: &sampling.ProbabilisticSamplingStrategy{
					SamplingRate: 0,
				},
			},
		},
	},
}

func Test_serviceStrategyCache_ReadWriteSequence(t *testing.T) {
	cache := newServiceStrategyCache(cacheTestItemTTL).(*serviceStrategyTTLCache)
	defer func() {
		assert.NoError(t, cache.Close())
	}()

	// override clock with a test-controlled one
	testTime := time.Unix(1690221207, 0)
	cache.clockFn = func() time.Time {
		return testTime
	}

	// initial read returns nothing
	result, ok := cache.get("fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get("barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// perform a write for fooSvc at testTime
	firstWriteTime := testTime
	cache.put("fooSvc", testStrategyResponseA)

	// whitebox assert for internal timestamp tracking (we don't want a caching bug manifesting as stale data serving)
	// (post-write)
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      firstWriteTime,
		strategyResponse: testStrategyResponseA,
	}, cache.items["fooSvc"])

	// read without time advancing
	result, ok = cache.get("fooSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)
	result, ok = cache.get("barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// reading does not mutate internal cache state
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      firstWriteTime,
		strategyResponse: testStrategyResponseA,
	}, cache.items["fooSvc"])

	// advance time (still within TTL time range)
	testTime = testTime.Add(20 * time.Millisecond)

	// the written item is still available
	result, ok = cache.get("fooSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)
	result, ok = cache.get("barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// advance time (just before end of TTL time range)
	testTime = testTime.Add(30 * time.Millisecond)

	// the written item is still available

	result, ok = cache.get("fooSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)
	result, ok = cache.get("barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// advance time (across TTL range)
	testTime = testTime.Add(1 * time.Millisecond)

	// the (now stale) cached item is no longer available
	result, ok = cache.get("fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get("barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      firstWriteTime,
		strategyResponse: testStrategyResponseA,
	}, cache.items["fooSvc"])
}

func Test_serviceStrategyCache_WritesUpdateTimestamp(t *testing.T) {
	cache := newServiceStrategyCache(cacheTestItemTTL).(*serviceStrategyTTLCache)
	defer func() {
		assert.NoError(t, cache.Close())
	}()

	// override clock with a test-controlled one
	testTime := time.Now()
	cache.clockFn = func() time.Time {
		return testTime
	}

	// initial read returns nothing
	result, ok := cache.get("fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get("barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// perform a write for barSvc at testTime + 10ms
	testTime = testTime.Add(10 * time.Millisecond)
	firstWriteTime := testTime
	cache.put("barSvc", testStrategyResponseA)

	// whitebox assert for internal timestamp tracking
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      firstWriteTime,
		strategyResponse: testStrategyResponseA,
	}, cache.items["barSvc"])

	// read without time advancing
	result, ok = cache.get("fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get("barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)

	// advance time (still within TTL time range)
	testTime = testTime.Add(10 * time.Millisecond)

	// the written item is still available
	result, ok = cache.get("fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get("barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)

	// perform a write for barSvc at testTime + 30ms (still within TTL, but we retain this more recent data)
	testTime = testTime.Add(10 * time.Millisecond)
	secondWriteTime := testTime
	cache.put("barSvc", testStrategyResponseB)

	// whitebox assert for internal timestamp tracking (post-write, still-fresh cache entry replaced with newer data)
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      secondWriteTime,
		strategyResponse: testStrategyResponseB,
	}, cache.items["barSvc"])

	// the written item is still available
	result, ok = cache.get("fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get("barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseB, result)

	// advance time (to end of what is now a new/full TTL for the new fresh item)
	testTime = testTime.Add(cacheTestItemTTL)

	result, ok = cache.get("fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get("barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseB, result)

	// advance time beyond the newer item's TTL
	testTime = testTime.Add(1)

	// the (now stale) cached item is no longer available
	result, ok = cache.get("fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get("barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// internal state for now-stale second written item is still maintained
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      secondWriteTime,
		strategyResponse: testStrategyResponseB,
	}, cache.items["barSvc"])
}

func Test_serviceStrategyCache_Concurrency(t *testing.T) {
	defer leaktest.CheckTimeout(t, time.Minute*2)

	cache := newServiceStrategyCache(cacheTestItemTTL).(*serviceStrategyTTLCache)
	defer func() {
		assert.NoError(t, cache.Close())
	}()

	// newServiceStrategyCache invokes this as well but with a practically-motivated period that is too long for tests.
	// We should at least exercise it for consideration by the race detector.
	go cache.periodicallyClearCache(time.Millisecond * 1)

	numThreads := 4
	numIterationsPerThread := 32

	wg := sync.WaitGroup{}
	wg.Add(numThreads)
	for i := 0; i < numThreads; i++ {
		ii := i
		go func() {
			for j := 0; j < numIterationsPerThread; j++ {
				for _, svcName := range []string{
					fmt.Sprintf("thread-specific-service-%d", ii),
					"contended-for-service",
				} {
					if _, ok := cache.get(svcName); !ok {
						cache.put(svcName, &sampling.SamplingStrategyResponse{})
					}
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
