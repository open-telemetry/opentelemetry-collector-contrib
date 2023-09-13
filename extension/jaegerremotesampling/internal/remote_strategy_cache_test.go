// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/tilinna/clock"
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
	mock := clock.NewMock(testTime)
	ctx, cfn := mock.DeadlineContext(context.Background(), testTime.Add(3*time.Minute))
	defer cfn()

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
	mock.Add(20 * time.Millisecond)

	// the written item is still available
	result, ok = cache.get(ctx, "fooSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// advance time (just before end of TTL time range)
	mock.Add(30 * time.Millisecond)

	// the written item is still available
	result, ok = cache.get(ctx, "fooSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.False(t, ok)
	assert.Nil(t, result)

	// advance time (across TTL range)
	mock.Add(1 * time.Millisecond)

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
	mock := clock.NewMock(startTime)
	ctx, cfn := mock.DeadlineContext(context.Background(), startTime.Add(3*time.Minute))
	defer cfn()

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
	firstWriteTime := mock.Add(10 * time.Millisecond)
	cache.put(ctx, "barSvc", testStrategyResponseA)

	// whitebox assert for internal timestamp tracking
	assert.Equal(t, serviceStrategyCacheEntry{
		retrievedAt:      firstWriteTime,
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
	mock.Add(10 * time.Millisecond)

	// the written item is still available
	result, ok = cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseA, result)

	// perform a write for barSvc at startTime + 30ms (still within TTL, but we retain this more recent data)
	secondWriteTime := mock.Add(10 * time.Millisecond)
	cache.put(ctx, "barSvc", testStrategyResponseB)

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
	mock.Add(cacheTestItemTTL)

	result, ok = cache.get(ctx, "fooSvc")
	assert.False(t, ok)
	assert.Nil(t, result)
	result, ok = cache.get(ctx, "barSvc")
	assert.True(t, ok)
	assert.Equal(t, testStrategyResponseB, result)

	// advance time beyond the newer item's TTL
	mock.Add(1)

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
	go cache.periodicallyClearCache(context.Background(), time.Millisecond*1)

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
					if _, ok := cache.get(context.Background(), svcName); !ok {
						cache.put(context.Background(), svcName, &api_v2.SamplingStrategyResponse{})
					}
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
