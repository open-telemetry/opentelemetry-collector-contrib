// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"strconv"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

// samplingPriority has the semantic result of parsing the "sampling.priority"
// attribute per OpenTracing semantic conventions.
type samplingPriority int

const (
	// deferDecision means that the decision if a span will be "sampled" (ie.:
	// forwarded by the collector) is made by hashing the trace ID according
	// to the configured sampling rate.
	deferDecision samplingPriority = iota
	// mustSampleSpan indicates that the span had a "sampling.priority" attribute
	// greater than zero and it is going to be sampled, ie.: forwarded by the
	// collector.
	mustSampleSpan
	// doNotSampleSpan indicates that the span had a "sampling.priority" attribute
	// equal zero and it is NOT going to be sampled, ie.: it won't be forwarded
	// by the collector.
	doNotSampleSpan

	// The constants help translate user friendly percentages to numbers direct used in sampling.
	numHashBuckets        = 0x4000 // Using a power of 2 to avoid division.
	bitMaskHashBuckets    = numHashBuckets - 1
	percentageScaleFactor = numHashBuckets / 100.0
)

type traceSamplerProcessor struct {
	scaledSamplingRate uint32
	hashSeed           uint32
	logger             *zap.Logger
}

// newTracesProcessor returns a processor.TracesProcessor that will perform head sampling according to the given
// configuration.
func newTracesProcessor(ctx context.Context, set processor.CreateSettings, cfg *Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	tsp := &traceSamplerProcessor{
		// Adjust sampling percentage on private so recalculations are avoided.
		scaledSamplingRate: uint32(cfg.SamplingPercentage * percentageScaleFactor),
		hashSeed:           cfg.HashSeed,
		logger:             set.Logger,
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		tsp.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (tsp *traceSamplerProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		rs.ScopeSpans().RemoveIf(func(ils ptrace.ScopeSpans) bool {
			ils.Spans().RemoveIf(func(s ptrace.Span) bool {
				sp := parseSpanSamplingPriority(s)
				if sp == doNotSampleSpan {
					// The OpenTelemetry mentions this as a "hint" we take a stronger
					// approach and do not sample the span since some may use it to
					// remove specific spans from traces.
					_ = stats.RecordWithTags(
						ctx,
						[]tag.Mutator{tag.Upsert(tagPolicyKey, "sampling_priority"), tag.Upsert(tagSampledKey, "false")},
						statCountTracesSampled.M(int64(1)),
					)
					return true
				}

				_ = stats.RecordWithTags(
					ctx,
					[]tag.Mutator{tag.Upsert(tagPolicyKey, "sampling_priority"), tag.Upsert(tagSampledKey, "true")},
					statCountTracesSampled.M(int64(1)),
				)

				// If one assumes random trace ids hashing may seems avoidable, however, traces can be coming from sources
				// with various different criteria to generate trace id and perhaps were already sampled without hashing.
				// Hashing here prevents bias due to such systems.
				tidBytes := s.TraceID()
				sampled := sp == mustSampleSpan ||
					computeHash(tidBytes[:], tsp.hashSeed)&bitMaskHashBuckets < tsp.scaledSamplingRate

				_ = stats.RecordWithTags(
					ctx,
					[]tag.Mutator{tag.Upsert(tagPolicyKey, "trace_id_hash"), tag.Upsert(tagSampledKey, strconv.FormatBool(sampled))},
					statCountTracesSampled.M(int64(1)),
				)
				return !sampled
			})
			// Filter out empty ScopeMetrics
			return ils.Spans().Len() == 0
		})
		// Filter out empty ResourceMetrics
		return rs.ScopeSpans().Len() == 0
	})
	if td.ResourceSpans().Len() == 0 {
		return td, processorhelper.ErrSkipProcessingData
	}
	return td, nil
}

// parseSpanSamplingPriority checks if the span has the "sampling.priority" tag to
// decide if the span should be sampled or not. The usage of the tag follows the
// OpenTracing semantic tags:
// https://github.com/opentracing/specification/blob/main/semantic_conventions.md#span-tags-table
func parseSpanSamplingPriority(span ptrace.Span) samplingPriority {
	attribMap := span.Attributes()
	if attribMap.Len() <= 0 {
		return deferDecision
	}

	samplingPriorityAttrib, ok := attribMap.Get("sampling.priority")
	if !ok {
		return deferDecision
	}

	// By default defer the decision.
	decision := deferDecision

	// Try check for different types since there are various client libraries
	// using different conventions regarding "sampling.priority". Besides the
	// client libraries it is also possible that the type was lost in translation
	// between different formats.
	switch samplingPriorityAttrib.Type() {
	case pcommon.ValueTypeInt:
		value := samplingPriorityAttrib.Int()
		if value == 0 {
			decision = doNotSampleSpan
		} else if value > 0 {
			decision = mustSampleSpan
		}
	case pcommon.ValueTypeDouble:
		value := samplingPriorityAttrib.Double()
		if value == 0.0 {
			decision = doNotSampleSpan
		} else if value > 0.0 {
			decision = mustSampleSpan
		}
	case pcommon.ValueTypeStr:
		attribVal := samplingPriorityAttrib.Str()
		if value, err := strconv.ParseFloat(attribVal, 64); err == nil {
			if value == 0.0 {
				decision = doNotSampleSpan
			} else if value > 0.0 {
				decision = mustSampleSpan
			}
		}
	}

	return decision
}
