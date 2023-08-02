// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
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
	// decide reports the result based on a probabilistic decision.
	decide(s ptrace.Span) bool

	// updateTracestate modifies the OTelTraceState assuming it will be
	// sampled, probabilistically or otherwise.  The "should" parameter
	// is the result from decide(), for the span's TraceID, which
	// will not be recalculated.
	updateTracestate(tid pcommon.TraceID, rnd sampling.Randomness, should bool, otts *sampling.OTelTraceState)
}

type traceProcessor struct {
	sampler traceSampler
	logger  *zap.Logger
}

type traceHashSampler struct {
	// Hash-based calculation
	hashScaledSamplingRate uint32
	hashSeed               uint32
	probability            float64
	svalueEncoding         string
}

type traceResampler struct {
	// TraceID-randomness-based calculation
	traceIDThreshold sampling.Threshold

	// tValueEncoding includes the leading "t:"
	tValueEncoding string
}

func randomnessFromSpan(s ptrace.Span) (sampling.Randomness, *sampling.W3CTraceState, error) {
	state := s.TraceState()
	raw := state.AsRaw()

	// Parse the arriving TraceState.
	wts, err := sampling.NewW3CTraceState(raw)
	var randomness sampling.Randomness
	if err == nil && wts.OTelValue().HasRValue() {
		// When the tracestate is OK and has r-value, use it.
		randomness = wts.OTelValue().RValueRandomness()
	} else {
		// All other cases, use the TraceID.
		randomness = sampling.RandomnessFromTraceID(s.TraceID())
	}
	return randomness, wts, err
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

	// error ignored below b/c already checked once
	mode, _ := parseSamplerMode(cfg.SamplerMode)
	if mode == modeUnset {
		if cfg.HashSeed != 0 {
			mode = modeHashSeed
		} else {
			mode = modeDownsample
		}
	}

	ratio := pct / 100
	switch mode {
	case modeHashSeed:
		ts := &traceHashSampler{}

		// Adjust sampling percentage on private so recalculations are avoided.
		ts.hashScaledSamplingRate = uint32(pct * percentageScaleFactor)
		ts.hashSeed = cfg.HashSeed
		ts.probability = ratio
		ts.svalueEncoding = strconv.FormatFloat(ratio, 'g', 4, 64)

		tp.sampler = ts
	case modeResample:
		// Encode t-value: for cases where the incoming context has
		tval, err := sampling.ProbabilityToTValue(ratio)
		if err != nil {
			return nil, err
		}
		// Compute the sampling threshold from the exact probability.
		threshold, err := sampling.TValueToThreshold(tval)
		if err != nil {
			return nil, err
		}

		ts := &traceResampler{}
		ts.tValueEncoding = tval
		ts.traceIDThreshold = threshold

		tp.sampler = ts
	case modeDownsample:
		// TODO
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		tp.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (ts *traceHashSampler) decide(s ptrace.Span) bool {
	// If one assumes random trace ids hashing may seems avoidable, however, traces can be coming from sources
	// with various different criteria to generate trace id and perhaps were already sampled without hashing.
	// Hashing here prevents bias due to such systems.
	tid := s.TraceID()
	return computeHash(tid[:], ts.hashSeed)&bitMaskHashBuckets < ts.hashScaledSamplingRate
}

func (ts *traceHashSampler) updateTracestate(tid pcommon.TraceID, _ sampling.Randomness, should bool, otts *sampling.OTelTraceState) {
	// No action, nothing is specified.
}

func (ts *traceResampler) decide(s ptrace.Span) bool {
	rnd := randomnessFromSpan(s)
	return ts.traceIDThreshold.ShouldSample(randomness)
}

func (ts *traceResampler) updateTracestate(tid pcommon.TraceID, rnd sampling.Randomness, should bool, otts *sampling.OTelTraceState) {
	// When this sampler decided not to sample, the t-value becomes zero.
	// Incoming TValue consistency is not checked when this happens.
	if !should {
		otts.SetTValue(sampling.ProbabilityZeroEncoding, sampling.Threshold{})
		return
	}
	arrivingHasNonZeroTValue := otts.HasTValue() && otts.TValueThreshold().Unsigned() != 0

	if arrivingHasNonZeroTValue {
		// Consistency check: if the TraceID is out of range
		// (unless the TValue is zero), the TValue is a lie.
		// If inconsistent, clear it.
		if !otts.TValueThreshold().ShouldSample(rnd) {
			arrivingHasNonZeroTValue = false
			otts.UnsetTValue()
		}
	}

	if arrivingHasNonZeroTValue &&
		otts.TValueThreshold().Unsigned() < ts.traceIDThreshold.Unsigned() {
		// Already-sampled case: test whether the unsigned value of the
		// threshold is smaller than this sampler is configured with.
		return
	}
	// Set the new effective t-value.
	otts.SetTValue(ts.tValueEncoding, ts.traceIDThreshold)
	return
}

func (tp *traceProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		rs.ScopeSpans().RemoveIf(func(ils ptrace.ScopeSpans) bool {
			ils.Spans().RemoveIf(func(s ptrace.Span) bool {
				priority := parseSpanSamplingPriority(s)
				if priority == doNotSampleSpan {
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

				probSample, otts := tp.sampler.decide(s)

				forceSample := priority == mustSampleSpan

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
					tp.sampler.updateTracestate(s.TraceID(), randomness, probSample, wts.OTelValue())

					var w strings.Builder
					wts.Serialize(&w)
					state.FromRaw(w.String())
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
