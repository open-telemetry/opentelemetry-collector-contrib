// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

func TestShardedProcessorCreation(t *testing.T) {
	cfg := Config{
		SamplingStrategy:        samplingStrategyTraceComplete,
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               400,
		ExpectedNewTracesPerSec: 100,
		NumShards:               4,
		PolicyCfgs:              testPolicy,
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	sp, ok := p.(*shardedProcessor)
	require.True(t, ok, "expected shardedProcessor when NumShards > 1")
	assert.Equal(t, uint32(4), sp.numShards)
	assert.Len(t, sp.shards, 4)

	for _, shard := range sp.shards {
		assert.Equal(t, uint64(100), shard.cfg.NumTraces)
	}
}

// shard0 returns the first (or only) shard of a processor created by
// newTracesProcessor, for tests that inspect shard internals.
func shard0(p processor.Traces) *tailSamplingSpanProcessor {
	return p.(*shardedProcessor).shards[0]
}

func TestShardedProcessorSingleShard(t *testing.T) {
	// NumShards of 1 and 0 (unset) walk the same code path as NumShards > 1,
	// producing a shardedProcessor with a single shard whose config is not
	// divided.
	for _, numShards := range []uint32{0, 1} {
		cfg := Config{
			SamplingStrategy:        samplingStrategyTraceComplete,
			DecisionWait:            defaultTestDecisionWait,
			NumTraces:               defaultNumTraces,
			ExpectedNewTracesPerSec: 10,
			NumShards:               numShards,
			PolicyCfgs:              testPolicy,
		}

		p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
		require.NoError(t, err)

		sp, ok := p.(*shardedProcessor)
		require.True(t, ok, "expected shardedProcessor for NumShards = %d", numShards)
		assert.Equal(t, uint32(1), sp.numShards)
		require.Len(t, sp.shards, 1)
		assert.Equal(t, uint64(defaultNumTraces), sp.shards[0].cfg.NumTraces)
		assert.Equal(t, uint64(10), sp.shards[0].cfg.ExpectedNewTracesPerSec)
	}
}

func TestShardedProcessorDisabledDecisionCacheStaysDisabled(t *testing.T) {
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        400,
		NumShards:        4,
		PolicyCfgs:       testPolicy,
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	for _, shard := range p.(*shardedProcessor).shards {
		assert.Zero(t, shard.cfg.DecisionCache.SampledCacheSize,
			"disabled sampled cache must not be enabled by shard division")
		assert.Zero(t, shard.cfg.DecisionCache.NonSampledCacheSize,
			"disabled non-sampled cache must not be enabled by shard division")
	}
}

func TestDividePolicyRates(t *testing.T) {
	cfgs := []PolicyCfg{
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "rate", Type: RateLimiting,
				RateLimitingCfg: RateLimitingCfg{SpansPerSecond: 1000, BurstCapacity: 100},
			},
		},
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "bytes", Type: BytesLimiting,
				BytesLimitingCfg: BytesLimitingCfg{BytesPerSecond: 4000},
			},
		},
		{
			sharedPolicyCfg: sharedPolicyCfg{Name: "and", Type: And},
			AndCfg: AndCfg{SubPolicyCfg: []AndSubPolicyCfg{{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "and-rate", Type: RateLimiting,
					RateLimitingCfg: RateLimitingCfg{SpansPerSecond: 400},
				},
			}}},
		},
		{
			sharedPolicyCfg: sharedPolicyCfg{Name: "composite", Type: Composite},
			CompositeCfg: CompositeCfg{
				MaxTotalSpansPerSecond: 800,
				RateAllocation:         []RateAllocationCfg{{Policy: "sub", Percent: 50}},
				SubPolicyCfg: []CompositeSubPolicyCfg{{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "sub", Type: RateLimiting,
						RateLimitingCfg: RateLimitingCfg{SpansPerSecond: 40},
					},
				}},
			},
		},
		{
			// Limit smaller than the shard count must not round down to 0,
			// which would sample nothing.
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "small", Type: RateLimiting,
				RateLimitingCfg: RateLimitingCfg{SpansPerSecond: 2},
			},
		},
	}

	divided := dividePolicyRates(cfgs, 4)

	assert.Equal(t, int64(250), divided[0].RateLimitingCfg.SpansPerSecond)
	assert.Equal(t, int64(100), divided[0].RateLimitingCfg.BurstCapacity,
		"explicit burst capacity must not be divided: it caps the size of a single admissible trace")
	assert.Equal(t, int64(1000), divided[1].BytesLimitingCfg.BytesPerSecond)
	assert.Equal(t, int64(8000), divided[1].BytesLimitingCfg.BurstCapacity,
		"unset burst capacity must be pinned to 2x the configured rate, not default to 2x the divided rate")
	assert.Equal(t, int64(100), divided[2].AndCfg.SubPolicyCfg[0].RateLimitingCfg.SpansPerSecond)
	assert.Equal(t, int64(200), divided[3].CompositeCfg.MaxTotalSpansPerSecond)
	assert.Equal(t, int64(10), divided[3].CompositeCfg.SubPolicyCfg[0].RateLimitingCfg.SpansPerSecond)
	assert.Equal(t, int64(50), divided[3].CompositeCfg.RateAllocation[0].Percent, "percentage allocations must not be divided")
	assert.Equal(t, int64(1), divided[4].RateLimitingCfg.SpansPerSecond)

	// The input must not be mutated: SetSamplingPolicy callers and the parent
	// config retain the undivided values.
	assert.Equal(t, int64(1000), cfgs[0].RateLimitingCfg.SpansPerSecond)
	assert.Equal(t, int64(0), cfgs[1].BytesLimitingCfg.BurstCapacity)
	assert.Equal(t, int64(400), cfgs[2].AndCfg.SubPolicyCfg[0].RateLimitingCfg.SpansPerSecond)
	assert.Equal(t, int64(800), cfgs[3].CompositeCfg.MaxTotalSpansPerSecond)
	assert.Equal(t, int64(40), cfgs[3].CompositeCfg.SubPolicyCfg[0].RateLimitingCfg.SpansPerSecond)

	// A single shard needs no division and returns the input unchanged.
	assert.Equal(t, cfgs, dividePolicyRates(cfgs, 1))
}

func TestShardedProcessorStartShutdown(t *testing.T) {
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        100,
		NumShards:        4,
		PolicyCfgs:       testPolicy,
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestShardedProcessorDeterministicRouting(t *testing.T) {
	sp := &shardedProcessor{numShards: 4}

	id1 := uInt64ToTraceID(100)
	id2 := uInt64ToTraceID(100)

	assert.Equal(t, sp.traceIDToShard(id1), sp.traceIDToShard(id2),
		"same trace ID must route to same shard")

	shardCounts := make(map[uint32]int)
	for i := range 100 {
		id := uInt64ToTraceID(uint64(i))
		shardCounts[sp.traceIDToShard(id)]++
	}
	// Sequential (non-random) trace IDs must still spread over every shard;
	// this guards against routing regressions that concentrate traffic on
	// one shard, such as taking raw ID bytes modulo numShards.
	require.Len(t, shardCounts, int(sp.numShards), "all shards must receive traces")
	for shard, count := range shardCounts {
		assert.Positive(t, count, "shard %d received no traces", shard)
	}
}

func TestShardedProcessorSamplesTraces(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        200,
		NumShards:        2,
		PolicyCfgs:       testPolicy,
		Options: []Option{
			withTickerFrequency(time.Millisecond),
		},
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	numTraces := 10
	for i := range numTraces {
		require.NoError(t, p.ConsumeTraces(t.Context(), simpleTracesWithID(uInt64ToTraceID(uint64(i)))))
	}

	require.Eventually(t, func() bool {
		return nextConsumer.SpanCount() == numTraces
	}, 5*time.Second, 10*time.Millisecond,
		"expected %d spans, got %d", numTraces, nextConsumer.SpanCount())
}

func TestShardedProcessorConcurrentConsume(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        1000,
		NumShards:        4,
		PolicyCfgs:       testPolicy,
		Options: []Option{
			withTickerFrequency(time.Millisecond),
		},
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	numGoroutines := 8
	tracesPerGoroutine := 25
	totalTraces := numGoroutines * tracesPerGoroutine

	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	for g := range numGoroutines {
		go func(offset int) {
			defer wg.Done()
			for i := range tracesPerGoroutine {
				id := uInt64ToTraceID(uint64(offset*tracesPerGoroutine + i))
				_ = p.ConsumeTraces(t.Context(), simpleTracesWithID(id))
			}
		}(g)
	}
	wg.Wait()

	require.Eventually(t, func() bool {
		return nextConsumer.SpanCount() == totalTraces
	}, 5*time.Second, 10*time.Millisecond,
		"expected %d spans, got %d", totalTraces, nextConsumer.SpanCount())
}

func TestShardedProcessorMultiTraceResourceSpans(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        200,
		NumShards:        3,
		PolicyCfgs:       testPolicy,
		Options: []Option{
			withTickerFrequency(time.Millisecond),
		},
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Build a single batch containing spans from multiple traces in one ResourceSpans
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	scope := rs.ScopeSpans().AppendEmpty()
	numTraces := 6
	for i := range numTraces {
		span := scope.Spans().AppendEmpty()
		span.SetTraceID(uInt64ToTraceID(uint64(i)))
		span.SetSpanID(uInt64ToSpanID(uint64(i)))
	}

	require.NoError(t, p.ConsumeTraces(t.Context(), traces))

	require.Eventually(t, func() bool {
		return nextConsumer.SpanCount() == numTraces
	}, 5*time.Second, 10*time.Millisecond,
		"expected %d spans, got %d", numTraces, nextConsumer.SpanCount())
}

func TestShardedProcessorCapabilities(t *testing.T) {
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        100,
		NumShards:        2,
		PolicyCfgs:       testPolicy,
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	assert.False(t, p.Capabilities().MutatesData)
}

func TestTraceIDToShard(t *testing.T) {
	sp := &shardedProcessor{numShards: 8}

	seen := make(map[uint32]int)
	id := pcommon.TraceID{}
	for i := range uint64(1000) {
		// Worst-case skew: IDs identical except for a counter in one byte
		// range, as produced by SDKs embedding timestamps or constants.
		binary.BigEndian.PutUint64(id[8:], i)
		shard := sp.traceIDToShard(id)
		assert.Less(t, shard, sp.numShards)
		seen[shard]++
	}

	// The mixing hash must spread even highly regular IDs over all shards.
	require.Len(t, seen, int(sp.numShards), "all shards must be used")
	for shard, count := range seen {
		// With uniform routing each shard expects 125 of 1000; allow wide
		// tolerance since the hash is deterministic, not random.
		assert.Greater(t, count, 50, "shard %d is severely underloaded", shard)
	}
}

func TestShardedProcessorMinNumTraces(t *testing.T) {
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        2,
		NumShards:        8,
		PolicyCfgs:       testPolicy,
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	sp := p.(*shardedProcessor)
	for _, shard := range sp.shards {
		assert.GreaterOrEqual(t, shard.cfg.NumTraces, uint64(1),
			"per-shard NumTraces must be at least 1")
	}
}
