// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"context"
	"encoding/binary"
	"errors"

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
// decision evaluation under high load.
type shardedProcessor struct {
	ctx       context.Context
	tracer    trace.Tracer
	shards    []*tailSamplingSpanProcessor
	numShards uint32
}

func newShardedTracesProcessor(ctx context.Context, set processor.Settings, nextConsumer consumer.Traces, cfg Config) (*shardedProcessor, error) {
	numShards := cfg.NumShards

	shardCfg := cfg
	shardCfg.NumTraces = max(1, cfg.NumTraces/uint64(numShards))
	shardCfg.ExpectedNewTracesPerSec = cfg.ExpectedNewTracesPerSec / uint64(numShards)
	shardCfg.NumShards = 1

	shards := make([]*tailSamplingSpanProcessor, numShards)
	for i := range numShards {
		p, err := newTracesProcessor(ctx, set, nextConsumer, shardCfg)
		if err != nil {
			return nil, err
		}
		shards[i] = p.(*tailSamplingSpanProcessor)
	}

	return &shardedProcessor{
		ctx:       ctx,
		tracer:    metadata.Tracer(set.TelemetrySettings),
		shards:    shards,
		numShards: numShards,
	}, nil
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
