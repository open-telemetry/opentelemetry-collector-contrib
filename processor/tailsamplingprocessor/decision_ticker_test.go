// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

func TestDecisionTickDelay(t *testing.T) {
	period := time.Second
	base := 50 * time.Millisecond
	numShards := 8

	for i := range numShards {
		got := decisionTickDelay(i, numShards, period, base)
		want := base + time.Duration(i)*period/time.Duration(numShards)
		assert.Equal(t, want, got, "shard %d", i)
	}

	// 8 shards / 1s → 125ms spacing between consecutive shards.
	assert.Equal(t, 125*time.Millisecond, decisionTickDelay(1, 8, period, 0)-decisionTickDelay(0, 8, period, 0))
	assert.Equal(t, 125*time.Millisecond, decisionTickDelay(7, 8, period, 0)-decisionTickDelay(6, 8, period, 0))

	assert.Zero(t, decisionTickDelay(0, 0, period, base))
	assert.Zero(t, decisionTickDelay(0, 1, 0, base))
}

func TestAssignDecisionTickDelays(t *testing.T) {
	period := time.Second
	base := 100 * time.Millisecond
	shards := make([]*tailSamplingSpanProcessor, 4)
	for i := range shards {
		shards[i] = &tailSamplingSpanProcessor{tickerFrequency: period}
	}
	shards[2].fixedFirstTickDelay = true
	shards[2].firstTickDelay = 5 * time.Millisecond

	assignDecisionTickDelays(shards, base, period)

	assert.Equal(t, base, shards[0].firstTickDelay)
	assert.Equal(t, base+period/4, shards[1].firstTickDelay)
	assert.Equal(t, 5*time.Millisecond, shards[2].firstTickDelay, "fixed delay must be preserved")
	assert.Equal(t, base+3*period/4, shards[3].firstTickDelay)
}

func TestRandomDecisionTickBase(t *testing.T) {
	period := 200 * time.Millisecond
	for range 50 {
		base := randomDecisionTickBase(period)
		assert.GreaterOrEqual(t, base, time.Duration(0))
		assert.Less(t, base, period)
	}
	assert.Zero(t, randomDecisionTickBase(0))
}

func TestStaggeredDecisionTickerStopBeforeFirstTick(t *testing.T) {
	tk := newStaggeredDecisionTicker(time.Hour, time.Second)
	done := make(chan struct{})
	go func() {
		tk.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop blocked with pending first-tick timer")
	}
	// Second Stop must be a no-op.
	tk.Stop()
}

func TestStaggeredDecisionTickerZeroDelayMatchesTicker(t *testing.T) {
	period := 20 * time.Millisecond
	tk := newStaggeredDecisionTicker(0, period)
	defer tk.Stop()

	start := time.Now()
	select {
	case <-tk.C:
	case <-time.After(time.Second):
		t.Fatal("expected first tick")
	}
	// With zero delay the first tick should arrive after about one period,
	// not immediately.
	assert.GreaterOrEqual(t, time.Since(start), period/2)
}

func TestDecisionTickStaggerDoesNotBlockIngest(t *testing.T) {
	next := new(consumertest.TracesSink)
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        100,
		NumShards:        1,
		PolicyCfgs:       testPolicy,
		Options: []Option{
			withFirstTickDelay(time.Hour),
			withTickerFrequency(time.Second),
		},
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), next, cfg)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, p.Shutdown(t.Context()))
	})

	start := time.Now()
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTracesWithID(uInt64ToTraceID(1))))
	assert.Less(t, time.Since(start), 2*time.Second, "ingest must not wait on the first decision-tick delay")
	assert.Zero(t, next.SpanCount(), "decision tick must not fire during long first-tick delay")
}

func TestShardedProcessorDecisionTickStagger(t *testing.T) {
	cfg := Config{
		SamplingStrategy:    samplingStrategyTraceComplete,
		DecisionWait:        defaultTestDecisionWait,
		NumTraces:           400,
		NumShards:           8,
		DecisionTickStagger: time.Second,
		PolicyCfgs:          testPolicy,
		Options: []Option{
			withTickerFrequency(time.Second),
		},
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)
	sp := p.(*shardedProcessor)
	require.Len(t, sp.shards, 8)

	base := sp.shards[0].firstTickDelay
	for i, shard := range sp.shards {
		assert.Equal(t, decisionTickDelay(i, 8, time.Second, base), shard.firstTickDelay, "shard %d", i)
	}
	for i := 1; i < len(sp.shards); i++ {
		assert.Equal(t, 125*time.Millisecond, sp.shards[i].firstTickDelay-sp.shards[i-1].firstTickDelay)
	}
	assert.Less(t, base, time.Second)
}

func TestDecisionTickStaggerDisabled(t *testing.T) {
	cfg := Config{
		SamplingStrategy:    samplingStrategyTraceComplete,
		DecisionWait:        defaultTestDecisionWait,
		NumTraces:           400,
		NumShards:           8,
		DecisionTickStagger: 0,
		PolicyCfgs:          testPolicy,
		Options: []Option{
			withTickerFrequency(time.Second),
		},
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)
	for i, shard := range p.(*shardedProcessor).shards {
		assert.Zero(t, shard.firstTickDelay, "shard %d should not be staggered when set to 0", i)
	}
}

func TestDecisionTickStaggerCustomWindow(t *testing.T) {
	window := 400 * time.Millisecond
	cfg := Config{
		SamplingStrategy:    samplingStrategyTraceComplete,
		DecisionWait:        defaultTestDecisionWait,
		NumTraces:           400,
		NumShards:           4,
		DecisionTickStagger: window,
		PolicyCfgs:          testPolicy,
		Options: []Option{
			withTickerFrequency(time.Second),
		},
	}

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)
	sp := p.(*shardedProcessor)
	base := sp.shards[0].firstTickDelay
	for i := 1; i < len(sp.shards); i++ {
		assert.Equal(t, window/4, sp.shards[i].firstTickDelay-sp.shards[i-1].firstTickDelay)
	}
	assert.Less(t, base, window)
}
