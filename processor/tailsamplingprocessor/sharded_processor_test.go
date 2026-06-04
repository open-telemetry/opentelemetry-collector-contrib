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

func TestShardedProcessorSingleShardFallback(t *testing.T) {
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		NumShards:        1,
		PolicyCfgs:       testPolicy,
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	_, ok := p.(*tailSamplingSpanProcessor)
	require.True(t, ok, "expected tailSamplingSpanProcessor when NumShards <= 1")
}

func TestShardedProcessorZeroShardsFallback(t *testing.T) {
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		NumShards:        0,
		PolicyCfgs:       testPolicy,
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	_, ok := p.(*tailSamplingSpanProcessor)
	require.True(t, ok, "expected tailSamplingSpanProcessor when NumShards is 0")
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

	// Verify shard assignment is based on the lower 8 bytes (little-endian)
	id := pcommon.TraceID{}
	binary.LittleEndian.PutUint64(id[8:], 0)
	assert.Equal(t, uint32(0), sp.traceIDToShard(id))

	binary.LittleEndian.PutUint64(id[8:], 1)
	assert.Equal(t, uint32(1), sp.traceIDToShard(id))

	binary.LittleEndian.PutUint64(id[8:], 8)
	assert.Equal(t, uint32(0), sp.traceIDToShard(id))

	binary.LittleEndian.PutUint64(id[8:], 15)
	assert.Equal(t, uint32(7), sp.traceIDToShard(id))
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
