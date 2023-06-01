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
	"fmt"
	"strconv"
	"strings"

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

var ErrInconsistentTValue = fmt.Errorf("inconsistent OTel TraceState t-value set")

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

	// Hashing method: The constants below help translate user friendly percentages
	// to numbers direct used in sampling.
	numHashBuckets        = 0x4000 // Using a power of 2 to avoid division.
	bitMaskHashBuckets    = numHashBuckets - 1
	percentageScaleFactor = numHashBuckets / 100.0
)

type traceSampler interface {
	// shouldSample reports the result based on a probabilistic decision.
	shouldSample(trace pcommon.TraceID) bool

	// updateSampled modifies the span assuming it will be
	// sampled, probabilistically or otherwise.  The "should" parameter
	// is the result from shouldSample(), for the span's TraceID, which
	// will not be recalculated.  Returns an error when the incoming TraceState
	// cannot be parsed.
	updateSampled(span ptrace.Span, should bool) error
}

type traceProcessor struct {
	sampler traceSampler
	logger  *zap.Logger
}

type traceHashSampler struct {
	// Hash-based calculation
	hashScaledSamplingRate uint32
	hashSeed               uint32
}

type traceIDSampler struct {
	// TraceID-randomness-based calculation
	traceIDThreshold sampling.Threshold

	// tValueEncoding includes the leading "t:"
	tValueEncoding string
}

// newTracesProcessor returns a processor.TracesProcessor that will perform head sampling according to the given
// configuration.
func newTracesProcessor(ctx context.Context, set processor.CreateSettings, cfg *Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	// README allows percents >100 to equal 100%, but t-value
	// encoding does not.  Correct it here.
	pct := float64(cfg.SamplingPercentage)
	if pct > 100 {
		pct = 100
	}

	tp := &traceProcessor{
		logger: set.Logger,
	}

	if cfg.HashSeed != 0 {
		ts := &traceHashSampler{}

		// Adjust sampling percentage on private so recalculations are avoided.
		ts.hashScaledSamplingRate = uint32(pct * percentageScaleFactor)
		ts.hashSeed = cfg.HashSeed

		tp.sampler = ts
	} else {
		// Encode t-value (OTEP 226), like %.4f.  (See FormatFloat().)
		ratio := pct / 100
		tval, err := sampling.ProbabilityToEncoded(ratio, 'f', 4)
		if err != nil {
			return nil, err
		}
		// Parse the exact value of probability encoded at this precision.
		ratio, _, err = sampling.EncodedToProbabilityAndAdjustedCount(tval)
		if err != nil {
			return nil, err
		}
		// Compute the sampling threshold from the exact probability.
		threshold, err := sampling.ProbabilityToThreshold(ratio)
		if err != nil {
			return nil, err
		}

		ts := &traceIDSampler{}
		ts.tValueEncoding = tval
		ts.traceIDThreshold = threshold

		tp.sampler = ts
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		tp.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (ts *traceHashSampler) shouldSample(input pcommon.TraceID) bool {
	// If one assumes random trace ids hashing may seems avoidable, however, traces can be coming from sources
	// with various different criteria to generate trace id and perhaps were already sampled without hashing.
	// Hashing here prevents bias due to such systems.
	return computeHash(input[:], ts.hashSeed)&bitMaskHashBuckets < ts.hashScaledSamplingRate
}

func (ts *traceHashSampler) updateSampled(ptrace.Span, bool) error {
	// Nothing specified
	return nil
}

func (ts *traceIDSampler) shouldSample(input pcommon.TraceID) bool {
	return ts.traceIDThreshold.ShouldSample(input)
}

func (ts *traceIDSampler) updateSampled(span ptrace.Span, should bool) error {
	state := span.TraceState()
	raw := state.AsRaw()

	// Fast path for the case where there is no arriving TraceState.
	if raw == "" {
		if should {
			state.FromRaw(ts.tValueEncoding)
		} else {
			state.FromRaw(sampling.ProbabilityZeroEncoding)
		}
		return nil
	}

	// Parse the arriving TraceState.
	wts, err := sampling.NewW3CTraceState(raw)
	if err != nil {
		return err
	}

	// Using the OTel trace state value:
	otts := wts.OTelValue()

	// When this sampler decided not to sample, the t-value becomes zero.
	// Incoming TValue consistency is not checked when this happens.
	if !should {
		otts.SetTValue("0", sampling.Threshold{})
		var w strings.Builder
		wts.Serialize(&w)
		state.FromRaw(w.String())
		return nil
	}

	arrivingHasNonZeroTValue := otts.HasTValue() && otts.TValueThreshold().Unsigned() != 0

	if arrivingHasNonZeroTValue {
		// Consistency check: if the TraceID is out of range
		// (unless the TValue is zero), the TValue is a lie.
		// If inconsistent, clear it.
		if !otts.TValueThreshold().ShouldSample(span.TraceID()) {
			// This value is returned below; the span continues
			// with any t-value.
			err = ErrInconsistentTValue
			arrivingHasNonZeroTValue = false
			otts.UnsetTValue()
		}
	}

	if arrivingHasNonZeroTValue && otts.TValueThreshold().Unsigned() < ts.traceIDThreshold.Unsigned() {
		// Already-sampled case: test whether the unsigned value of the
		// threshold is smaller than this sampler is configured with.
		return err
	}

	// Set the new effective t-value.
	otts.SetTValue(ts.tValueEncoding, ts.traceIDThreshold)
	var w strings.Builder
	wts.Serialize(&w)
	state.FromRaw(w.String())
	return err
}

func (tp *traceProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
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

				probSample := tp.sampler.shouldSample(s.TraceID())

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

				if sampled {
					err := tp.sampler.updateSampled(s, probSample)
					if err != nil {
						tp.logger.Info("sampling t-value update failed", zap.Error(err))
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
