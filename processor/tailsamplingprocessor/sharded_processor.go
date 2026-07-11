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
)

// shardedProcessor wraps N independent tailSamplingSpanProcessor instances,
// routing traces to shards by trace ID. Each shard runs its own event loop
// goroutine, eliminating contention between trace ingestion and sampling
// decision evaluation under high load. It is the single code path for all
// configurations: num_shards of 1 (the default) simply creates one shard.
type shardedProcessor struct {
	ctx       context.Context
	tracer    trace.Tracer
	shards    []*tailSamplingSpanProcessor
	numShards uint32
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
		// A size of 0 means the cache is disabled and must stay 0.
		if cfg.DecisionCache.SampledCacheSize > 0 {
			shardCfg.DecisionCache.SampledCacheSize = max(1, cfg.DecisionCache.SampledCacheSize/int(numShards))
		}
		if cfg.DecisionCache.NonSampledCacheSize > 0 {
			shardCfg.DecisionCache.NonSampledCacheSize = max(1, cfg.DecisionCache.NonSampledCacheSize/int(numShards))
		}
		shardCfg.PolicyCfgs = dividePolicyRates(cfg.PolicyCfgs, numShards)
	}

	// Shared across shards so the traces_on_memory gauge reports the
	// processor-wide total.
	tracesOnMemory := new(atomic.Int64)

	shards := make([]*tailSamplingSpanProcessor, numShards)
	for i := range numShards {
		p, err := newShardProcessor(ctx, set, nextConsumer, shardCfg, tracesOnMemory)
		if err != nil {
			return nil, err
		}
		shards[i] = p
	}

	return &shardedProcessor{
		ctx:       ctx,
		tracer:    metadata.Tracer(set.TelemetrySettings),
		shards:    shards,
		numShards: numShards,
	}, nil
}

// dividePolicyRates returns a copy of cfgs with per-second rate limits
// divided by numShards. Traces are distributed uniformly across shards by
// trace ID, so each shard enforcing 1/N of a limit keeps the aggregate limit
// close to the configured value. Without this, every shard would enforce the
// full limit and the effective limit would be limit*numShards.
func dividePolicyRates(cfgs []PolicyCfg, numShards uint32) []PolicyCfg {
	if numShards <= 1 || len(cfgs) == 0 {
		return cfgs
	}
	out := slices.Clone(cfgs)
	for i := range out {
		divideSharedPolicyRates(&out[i].sharedPolicyCfg, numShards)
		out[i].CompositeCfg = divideCompositeRates(out[i].CompositeCfg, numShards)
		out[i].AndCfg.SubPolicyCfg = divideAndSubPolicyRates(out[i].AndCfg.SubPolicyCfg, numShards)
		divideSharedPolicyRates(&out[i].NotCfg.SubPolicy.sharedPolicyCfg, numShards)
		out[i].DropCfg.SubPolicyCfg = divideAndSubPolicyRates(out[i].DropCfg.SubPolicyCfg, numShards)
	}
	return out
}

func divideCompositeRates(cfg CompositeCfg, numShards uint32) CompositeCfg {
	cfg.MaxTotalSpansPerSecond = divideRate(cfg.MaxTotalSpansPerSecond, numShards)
	// RateAllocation is percentage-based and needs no division.
	if len(cfg.SubPolicyCfg) > 0 {
		subs := slices.Clone(cfg.SubPolicyCfg)
		for i := range subs {
			divideSharedPolicyRates(&subs[i].sharedPolicyCfg, numShards)
			subs[i].AndCfg.SubPolicyCfg = divideAndSubPolicyRates(subs[i].AndCfg.SubPolicyCfg, numShards)
		}
		cfg.SubPolicyCfg = subs
	}
	return cfg
}

func divideAndSubPolicyRates(subs []AndSubPolicyCfg, numShards uint32) []AndSubPolicyCfg {
	if len(subs) == 0 {
		return subs
	}
	out := slices.Clone(subs)
	for i := range out {
		divideSharedPolicyRates(&out[i].sharedPolicyCfg, numShards)
	}
	return out
}

func divideSharedPolicyRates(cfg *sharedPolicyCfg, numShards uint32) {
	cfg.RateLimitingCfg.SpansPerSecond = divideRate(cfg.RateLimitingCfg.SpansPerSecond, numShards)
	cfg.RateLimitingCfg.BurstCapacity = divideRate(cfg.RateLimitingCfg.BurstCapacity, numShards)
	cfg.BytesLimitingCfg.BytesPerSecond = divideRate(cfg.BytesLimitingCfg.BytesPerSecond, numShards)
	cfg.BytesLimitingCfg.BurstCapacity = divideRate(cfg.BytesLimitingCfg.BurstCapacity, numShards)
}

// divideRate splits a positive per-second limit across numShards, keeping a
// minimum of 1 so a limit smaller than numShards doesn't round down to
// "sample nothing". Zero and negative values mean unset/disabled and are
// preserved.
func divideRate(v int64, numShards uint32) int64 {
	if v <= 0 {
		return v
	}
	return max(1, v/int64(numShards))
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
		idToSpansAndScope := groupSpansByTraceKey(rss)

		for traceID, spans := range idToSpansAndScope {
			newRSS, rootSpan := newResourceSpanFromSpanAndScopes(rss, spans)
			shardIdx := sp.traceIDToShard(traceID)
			shardBatches[shardIdx] = append(shardBatches[shardIdx], traceBatch{
				id:        traceID,
				rootSpan:  rootSpan,
				rss:       newRSS,
				spanCount: int64(len(spans)),
			})
			totalSpans += int64(len(spans))
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
	cfgs = dividePolicyRates(cfgs, sp.numShards)
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
	h := binary.LittleEndian.Uint64(id[8:])
	return uint32(h % uint64(sp.numShards))
}
