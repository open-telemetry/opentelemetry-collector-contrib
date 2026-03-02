// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotesource

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/jaegertracing/jaeger-idl/proto-gen/api_v2"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

const cacheTestItemTTL = 50 * time.Millisecond

var testStrategyResponseA = &api_v2.SamplingStrategyResponse{
	StrategyType: api_v2.SamplingStrategyType_PROBABILISTIC,
	OperationSampling: &api_v2.PerOperationSamplingStrategies{
		DefaultSamplingProbability: 0.1337,
	},
}

var testStrategyResponseB = &api_v2.SamplingStrategyResponse{
	StrategyType: api_v2.SamplingStrategyType_PROBABILISTIC,
	OperationSampling: &api_v2.PerOperationSamplingStrategies{
		DefaultSamplingProbability: 0.001,
		PerOperationStrategies: []*api_v2.OperationSamplingStrategy{
			{
				Operation: "always-sampled-op",
				ProbabilisticSampling: &api_v2.ProbabilisticSamplingStrategy{
					SamplingRate: 1.0,
				},
			},
			{
				Operation: "never-sampled-op",
				ProbabilisticSampling: &api_v2.ProbabilisticSamplingStrategy{
					SamplingRate: 0,
				},
			},
		},
	},
}

func Test_serviceStrategyCache_ReadWriteSequence(t *testing.T) {
	testTime := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
	mock := clockwork.NewFakeClockAt(testTime)
	_ = mock.After(3 * time.Minute)

	ctx, cancel := context.WithCancel(clockwork.AddToContext(t.Context(), mock))
	defer cancel()

	cache := newServiceStrategyCache(cacheTestItemTTL).(*serviceStrategyTTLCache)
	defer func() {
		assert.NoError(t, cache.Close())
	}()

	// initial read returns nothing
	result, ok := cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// perform a write for fooSvc at testTime
	firstWriteTime := mock.Now()
	cache.put(ctx, "fooSvc", testStrategyResponseA)

	// whitebox assert for internal timestamp tracking (we don't want a caching bug manifesting as stale data serving)
	// (post-write)
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      firstWriteTime,
		strategyResponse: testStrategyResponseA,
	}, cache.items["fooSvc"])

	// read without time advancing
	result, ok = cache.get(ctx, "fooSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// reading does not mutate internal cache state
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      firstWriteTime,
		strategyResponse: testStrategyResponseA,
	}, cache.items["fooSvc"])

	// advance time (still within TTL time range)
	mock.Advance(20 * time.Millisecond)

	// the written item is still available
	result, ok = cache.get(ctx, "fooSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// advance time (just before end of TTL time range)
	mock.Advance(30 * time.Millisecond)

	// the written item is still available
	result, ok = cache.get(ctx, "fooSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// advance time (across TTL range)
	mock.Advance(1 * time.Millisecond)

	// the (now stale) cached item is no longer available
	result, ok = cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      firstWriteTime,
		strategyResponse: testStrategyResponseA,
	}, cache.items["fooSvc"])
}

func Test_serviceStrategyCache_WritesUpdateTimestamp(t *testing.T) {
	startTime := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
	mock := clockwork.NewFakeClockAt(startTime)
	_ = mock.After(3 * time.Minute)

	ctx, cancel := context.WithCancel(clockwork.AddToContext(t.Context(), mock))
	defer cancel()

	cache := newServiceStrategyCache(cacheTestItemTTL).(*serviceStrategyTTLCache)
	defer func() {
		assert.NoError(t, cache.Close())
	}()

	// initial read returns nothing
	result, ok := cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// perform a write for barSvc at startTime + 10ms
	mock.Advance(10 * time.Millisecond)
	cache.put(ctx, "barSvc", testStrategyResponseA)

	// whitebox assert for internal timestamp tracking
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      mock.Now(),
		strategyResponse: testStrategyResponseA,
	}, cache.items["barSvc"])

	// read without time advancing
	result, ok = cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)

	// advance time (still within TTL time range)
	mock.Advance(10 * time.Millisecond)

	// the written item is still available
	result, ok = cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)

	// perform a write for barSvc at startTime + 30ms (still within TTL, but we retain this more recent data)
	mock.Advance(10 * time.Millisecond)
	cache.put(ctx, "barSvc", testStrategyResponseB)
	secondWriteTime := mock.Now()

	// whitebox assert for internal timestamp tracking (post-write, still-fresh cache entry replaced with newer data)
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      secondWriteTime,
		strategyResponse: testStrategyResponseB,
	}, cache.items["barSvc"])

	// the written item is still available
	result, ok = cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseB, result)

	// advance time (to end of what is now a new/full TTL for the new fresh item)
	mock.Advance(cacheTestItemTTL)

	result, ok = cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseB, result)

	// advance time beyond the newer item's TTL
	mock.Advance(1)

	// the (now stale) cached item is no longer available
	result, ok = cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// internal state for now-stale second written item is still maintained
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      secondWriteTime,
		strategyResponse: testStrategyResponseB,
	}, cache.items["barSvc"])
}

func Test_serviceStrategyCache_Concurrency(t *testing.T) {
	defer leaktest.CheckTimeout(t, time.Minute*3)

	cache := newServiceStrategyCache(cacheTestItemTTL).(*serviceStrategyTTLCache)
	defer func() {
		assert.NoError(t, cache.Close())
	}()

	// newServiceStrategyCache invokes this as well but with a practically-motivated period that is too long for tests.
	// We should at least exercise it for consideration by the race detector.
	// NB: We don't use a mock clock in this concurrency test case.
	go cache.periodicallyClearCache(t.Context(), time.Millisecond*1)

	numThreads := 4
	numIterationsPerThread := 32

	wg := sync.WaitGroup{}
	wg.Add(numThreads)
	for i := range numThreads {
		go func() {
			for range numIterationsPerThread {
				for _, svcName := range []string{
					fmt.Sprintf("thread-specific-service-%d", i),
					"contended-for-service",
				} {
					if _, ok := cache.get(t.Context(), svcName); !ok {
						cache.put(t.Context(), svcName, &api_v2.SamplingStrategyResponse{})
					}
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
