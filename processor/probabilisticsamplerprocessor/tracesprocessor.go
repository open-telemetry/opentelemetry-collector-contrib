// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

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

	zeroTvalue = "t:0"
)

type traceSamplerProcessor struct {
	// Legacy hash-based calculation
	hashScaledSamplingRate uint32
	hashSeed               uint32

	// Modern TraceID-randomness-based calculation
	traceIDThreshold sampling.Threshold
	tValueEncoding   string

	logger *zap.Logger
}

// newTracesProcessor returns a processor.TracesProcessor that will perform head sampling according to the given
// configuration.
func newTracesProcessor(ctx context.Context, set processor.CreateSettings, cfg *Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	tsp := &traceSamplerProcessor{
		logger: set.Logger,
	}
	// README allows percents >100 to equal 100%, but t-value
	// encoding does not.  Correct it here.
	pct := float64(cfg.SamplingPercentage)
	if pct > 100 {
		pct = 100
	}

	if cfg.HashSeed != 0 {
		// Adjust sampling percentage on private so recalculations are avoided.
		tsp.hashScaledSamplingRate = uint32(pct * percentageScaleFactor)
		tsp.hashSeed = cfg.HashSeed
	} else {
		// Encode t-value (OTEP 226), like %.4f.  (See FormatFloat().)
		ratio := pct / 100
		tval, err := sampling.ProbabilityToTvalue(ratio, 'f', 4)
		if err != nil {
			return nil, err
		}
		threshold, err := sampling.ProbabilityToThreshold(ratio)
		if err != nil {
			return nil, err
		}

		tsp.tValueEncoding = tval
		tsp.traceIDThreshold = threshold
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		tsp.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (tsp *traceSamplerProcessor) probabilitySampleFromTraceID(input pcommon.TraceID) (sample, consistent bool) {
	// When the hash seed is set, fall back to the legacy behavior
	// using the FNV hash.
	if tsp.hashSeed != 0 {
		// If one assumes random trace ids hashing may seems avoidable, however, traces can be coming from sources
		// with various different criteria to generate trace id and perhaps were already sampled without hashing.
		// Hashing here prevents bias due to such systems.
		return computeHash(input[:], tsp.hashSeed)&bitMaskHashBuckets < tsp.hashScaledSamplingRate, false
	}

	// Hash seed zero => assume tracecontext v2

	return tsp.traceIDThreshold.ShouldSample(input), true
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

				forceSample := sp == mustSampleSpan

				probSample, consistent := tsp.probabilitySampleFromTraceID(s.TraceID())

				sampled := forceSample || probSample

				if forceSample {
					_ = stats.RecordWithTags(
						ctx,
						[]tag.Mutator{tag.Upsert(tagPolicyKey, "sampling_priority"), tag.Upsert(tagSampledKey, "true")},
						statCountTracesSampled.M(int64(1)),
					)
				} else {
					_ = stats.RecordWithTags(
						ctx,
						[]tag.Mutator{tag.Upsert(tagPolicyKey, "trace_id_hash"), tag.Upsert(tagSampledKey, strconv.FormatBool(sampled))},
						statCountTracesSampled.M(int64(1)),
					)
				}

				if consistent {
					// Attach the t-value!
					ts := s.TraceState()

					// Get the t-value encoding.
					enc := tsp.tValueEncoding
					if !probSample {
						// forceSample is implied, use the zero value.
						enc = zeroTvalue
					}

					raw := ts.AsRaw()
					if raw == "" {
						// No incoming t-value, i.e., the simple case.
						ts.FromRaw(enc)
					} else {
						// Complex case: combine t-values.
						// TODO @@@ bring in code from
						// https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main/samplers/probability/consistent
					}
				}

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
