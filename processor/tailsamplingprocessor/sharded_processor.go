// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"context"
	"encoding/binary"
	"errors"
	"slices"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

// shardedProcessor wraps N independent tailSamplingSpanProcessor instances,
// routing traces to shards by trace ID. Each shard runs its own event loop
// goroutine, eliminating contention between trace ingestion and sampling
// decision evaluation under high load. It is the single code path for all
// configurations: num_shards of 1 (the default) simply creates one shard.
type shardedProcessor struct {
	tracer    trace.Tracer
	shards    []*tailSamplingSpanProcessor
	numShards uint32
}

// shardShared holds the dependencies all shards of one processor have in
// common, built once instead of per shard.
type shardShared struct {
	telemetry *metadata.TelemetryBuilder
	tracer    trace.Tracer
	// tracesOnMemory counts the traces held in memory across all shards so
	// the traces_on_memory gauge reports the processor-wide total instead of
	// per-shard values overwriting each other on the same time series.
	tracesOnMemory *atomic.Int64
}

func newShardedTracesProcessor(ctx context.Context, set processor.Settings, nextConsumer consumer.Traces, cfg Config) (*shardedProcessor, error) {
	numShards := max(1, cfg.NumShards)

	shardCfg := cfg
	if numShards > 1 {
		shardCfg.NumTraces = max(1, cfg.NumTraces/uint64(numShards))
		shardCfg.ExpectedNewTracesPerSec = cfg.ExpectedNewTracesPerSec / uint64(numShards)
		// Decision caches are keyed on trace_id and each shard only sees ~1/N of
		// the trace_id space. Without this division total decision-cache memory
		// would scale by num_shards, defeating the memory benefit of sharding.
		shardCfg.DecisionCache.SampledCacheSize = divideRate(cfg.DecisionCache.SampledCacheSize, numShards)
		shardCfg.DecisionCache.NonSampledCacheSize = divideRate(cfg.DecisionCache.NonSampledCacheSize, numShards)
		shardCfg.PolicyCfgs = shardPolicyCfgs(cfg.PolicyCfgs, numShards)
	}

	telemetry, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	shared := shardShared{
		telemetry:      telemetry,
		tracer:         metadata.Tracer(set.TelemetrySettings),
		tracesOnMemory: new(atomic.Int64),
	}

	shards := make([]*tailSamplingSpanProcessor, numShards)
	for i := range numShards {
		p, err := newShardProcessor(ctx, set, nextConsumer, shardCfg, shared)
		if err != nil {
			return nil, err
		}
		shards[i] = p
	}

	return &shardedProcessor{
		tracer:    shared.tracer,
		shards:    shards,
		numShards: numShards,
	}, nil
}

// shardPolicyCfgs returns a copy of cfgs whose rate limits are enforced
// across all shards together. rate_limiting and bytes_limiting policies get a
// limiter shared by every shard, so the configured rate and burst capacity
// apply to the processor as a whole. The composite max_total_spans_per_second
// is a per-shard counter and is instead divided by numShards; traces are
// distributed uniformly across shards by trace ID, so the aggregate limit
// stays close to the configured value.
func shardPolicyCfgs(cfgs []PolicyCfg, numShards uint32) []PolicyCfg {
	if numShards <= 1 || len(cfgs) == 0 {
		return cfgs
	}
	out := slices.Clone(cfgs)
	for i := range out {
		shareLimiter(&out[i].sharedPolicyCfg)
		out[i].CompositeCfg = shardCompositeCfg(out[i].CompositeCfg, numShards)
		out[i].AndCfg.SubPolicyCfg = shardAndSubPolicyCfgs(out[i].AndCfg.SubPolicyCfg)
		shareLimiter(&out[i].NotCfg.SubPolicy.sharedPolicyCfg)
		out[i].DropCfg.SubPolicyCfg = shardAndSubPolicyCfgs(out[i].DropCfg.SubPolicyCfg)
	}
	return out
}

func shardCompositeCfg(cfg CompositeCfg, numShards uint32) CompositeCfg {
	cfg.MaxTotalSpansPerSecond = divideRate(cfg.MaxTotalSpansPerSecond, numShards)
	// RateAllocation is percentage-based and needs no division.
	if len(cfg.SubPolicyCfg) > 0 {
		subs := slices.Clone(cfg.SubPolicyCfg)
		for i := range subs {
			shareLimiter(&subs[i].sharedPolicyCfg)
			subs[i].AndCfg.SubPolicyCfg = shardAndSubPolicyCfgs(subs[i].AndCfg.SubPolicyCfg)
		}
		cfg.SubPolicyCfg = subs
	}
	return cfg
}

func shardAndSubPolicyCfgs(subs []AndSubPolicyCfg) []AndSubPolicyCfg {
	if len(subs) == 0 {
		return subs
	}
	out := slices.Clone(subs)
	for i := range out {
		shareLimiter(&out[i].sharedPolicyCfg)
		shareLimiter(&out[i].NotCfg.SubPolicy.sharedPolicyCfg)
	}
	return out
}

// shareLimiter attaches the limiter every shard's evaluator for cfg will use.
func shareLimiter(cfg *sharedPolicyCfg) {
	switch cfg.Type {
	case RateLimiting:
		rl := &cfg.RateLimitingCfg
		rl.limiter = sampling.NewLimiter(rl.SpansPerSecond, rl.BurstCapacity)
	case BytesLimiting:
		bl := &cfg.BytesLimitingCfg
		bl.limiter = sampling.NewLimiter(bl.BytesPerSecond, bl.BurstCapacity)
	}
}

// divideRate splits a positive limit across numShards, keeping a minimum of
// 1 so a limit smaller than numShards doesn't round down to 0 (which would
// mean "sample nothing" for rates and "disabled" for cache sizes). Zero and
// negative values mean unset/disabled and are preserved.
func divideRate[T int | int64](v T, numShards uint32) T {
	if v <= 0 {
		return v
	}
	return max(1, v/T(numShards))
}

func (*shardedProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (sp *shardedProcessor) Start(ctx context.Context, host component.Host) error {
	for _, s := range sp.shards {
		if err := s.Start(ctx, host); err != nil {
			return err
		}
	}
	return nil
}

func (sp *shardedProcessor) Shutdown(ctx context.Context) error {
	var errs []error
	for _, s := range sp.shards {
		if err := s.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (sp *shardedProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	_, span := sp.tracer.Start(ctx, "tailsampling.ConsumeTraces")
	defer span.End()

	var totalSpans, totalTraces, totalResourceSpans int64

	shardBatches := make([][]traceBatch, sp.numShards)

	for _, rss := range td.ResourceSpans().All() {
		totalResourceSpans++
		for _, batch := range splitResourceSpansByTrace(rss) {
			shardIdx := sp.traceIDToShard(batch.id)
			shardBatches[shardIdx] = append(shardBatches[shardIdx], batch)
			totalSpans += batch.spanCount
			totalTraces++
		}
	}

	for i, batch := range shardBatches {
		if len(batch) > 0 {
			sp.shards[i].workChan <- batch
		}
	}

	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("traces.count", totalTraces),
			attribute.Int64("spans.count", totalSpans),
			attribute.Int64("resource_spans.count", totalResourceSpans),
		)
	}

	return nil
}

func (sp *shardedProcessor) SetSamplingPolicy(cfgs []PolicyCfg) {
	cfgs = shardPolicyCfgs(cfgs, sp.numShards)
	for _, s := range sp.shards {
		s.SetSamplingPolicy(cfgs)
	}
}

func (sp *shardedProcessor) SetMaximumTraceSizeBytes(size uint64) {
	for _, s := range sp.shards {
		s.SetMaximumTraceSizeBytes(size)
	}
}

func (sp *shardedProcessor) traceIDToShard(id pcommon.TraceID) uint32 {
	// Mix the full trace ID instead of taking raw bytes modulo numShards:
	// real-world trace IDs are not always uniformly random (some SDKs and
	// proxies embed timestamps or constants, or zero-pad 64-bit IDs), and a
	// raw modulo would concentrate such IDs on few shards, exhausting one
	// shard's NumTraces budget while the others sit empty.
	h := binary.LittleEndian.Uint64(id[0:8]) ^ binary.LittleEndian.Uint64(id[8:16])
	// splitmix64 finalizer.
	h ^= h >> 30
	h *= 0xbf58476d1ce4e5b9
	h ^= h >> 27
	h *= 0x94d049bb133111eb
	h ^= h >> 31
	return uint32(h % uint64(sp.numShards))
}
