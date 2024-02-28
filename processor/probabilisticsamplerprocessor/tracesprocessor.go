// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
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

	// randomFlagValue is defined in W3C Trace Context Level 2.
	randomFlagValue = 0x2
)

var (
	ErrInconsistentArrivingTValue = fmt.Errorf("inconsistent arriving t-value: span should not have been sampled")
	ErrMissingRandomness          = fmt.Errorf("missing randomness; trace flag not set")
)

type traceSampler interface {
	// decide reports the result based on a probabilistic decision.
	decide(s ptrace.Span) (bool, *sampling.W3CTraceState, error)

	// updateTracestate modifies the OTelTraceState assuming it will be
	// sampled, probabilistically or otherwise.  The "should" parameter
	// is the result from decide().
	updateTracestate(tid pcommon.TraceID, should bool, wts *sampling.W3CTraceState)
}

type traceProcessor struct {
	sampler traceSampler
	logger  *zap.Logger
}

// inconsistentCommon implements updateTracestate() for samplers that
// do not use OTel consistent sampling.
type inconsistentCommon struct {
}

// traceHasher is the original hash-based implementation.
type traceHasher struct {
	// Hash-based calculation
	hashScaledSamplerate uint32
	hashSeed             uint32
	probability          float64

	inconsistentCommon
}

// zeroProbability is a bypass for all cases with Percent==0.
type zeroProbability struct {
	inconsistentCommon
}

// inconsistentCommon implements updateTracestate() for samplers that
// use OTel consistent sampling.
type consistentCommon struct {
	// strict randomness checking
	strict bool
}

// traceEqualizer adjusts thresholds absolutely.  Cannot be used with zero.
type traceEqualizer struct {
	// TraceID-randomness-based calculation
	traceIDThreshold sampling.Threshold

	// tValueEncoding includes the leading "t:"
	tValueEncoding string

	consistentCommon
}

// traceEqualizer adjusts thresholds relatively.  Cannot be used with zero.
type traceProportionalizer struct {
	// ratio in the range [2**-56, 1]
	ratio float64

	// precision in number of hex digits
	prec uint8

	consistentCommon
}

func (*consistentCommon) updateTracestate(tid pcommon.TraceID, should bool, wts *sampling.W3CTraceState) {
	// When this sampler decided not to sample, the t-value becomes zero.
	if !should {
		wts.OTelValue().ClearTValue()
	}
}

func (cc *consistentCommon) randomnessFromSpan(s ptrace.Span) (sampling.Randomness, *sampling.W3CTraceState, error) {
	state := s.TraceState()
	raw := state.AsRaw()

	// Parse the arriving TraceState.
	wts, err := sampling.NewW3CTraceState(raw)
	var randomness sampling.Randomness
	if err == nil {
		if rv, has := wts.OTelValue().RValueRandomness(); has {
			// When the tracestate is OK and has r-value, use it.
			randomness = rv
		} else if cc.strict && (s.Flags()&randomFlagValue) != randomFlagValue {
			// If strict and the flag is missing
			err = ErrMissingRandomness
		} else {
			// Whether !strict or the random flag is correctly set.
			randomness = sampling.TraceIDToRandomness(s.TraceID())
		}
	}

	// Consistency check: if the TraceID is out of range, the
	// TValue is a lie.  If inconsistent, clear it and return an error.
	if err == nil {
		otts := wts.OTelValue()
		if tv, has := otts.TValueThreshold(); has {
			if !tv.ShouldSample(randomness) {
				if cc.strict {
					err = ErrInconsistentArrivingTValue
				} else {
					// TODO: warning?
					otts.ClearTValue()
				}
			}
		}
	}

	return randomness, &wts, err
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
	if cfg.SamplerMode == modeUnset {
		if cfg.HashSeed != 0 {
			cfg.SamplerMode = HashSeed
		} else {
			cfg.SamplerMode = DefaultMode
		}
	}

	if pct == 0 {
		tp.sampler = &zeroProbability{}
	} else {
		ratio := pct / 100
		switch cfg.SamplerMode {
		case HashSeed:
			ts := &traceHasher{}

			// Adjust sampling percentage on private so recalculations are avoided.
			ts.hashScaledSamplerate = uint32(pct * percentageScaleFactor)
			ts.hashSeed = cfg.HashSeed
			ts.probability = ratio

			tp.sampler = ts
		case Equalizing:
			threshold, err := sampling.ProbabilityToThresholdWithPrecision(ratio, cfg.SamplingPrecision)
			if err != nil {
				return nil, err
			}

			tp.sampler = &traceEqualizer{
				tValueEncoding:   threshold.TValue(),
				traceIDThreshold: threshold,
				consistentCommon: consistentCommon{
					strict: cfg.StrictRandomness,
				},
			}
		case Proportional:
			tp.sampler = &traceProportionalizer{
				ratio: ratio,
				prec:  cfg.SamplingPrecision,
				consistentCommon: consistentCommon{
					strict: cfg.StrictRandomness,
				},
			}
		}
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		tp.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (ts *traceHasher) decide(s ptrace.Span) (bool, *sampling.W3CTraceState, error) {
	// If one assumes random trace ids hashing may seems avoidable, however, traces can be coming from sources
	// with various different criteria to generate trace id and perhaps were already sampled without hashing.
	// Hashing here prevents bias due to such systems.
	tid := s.TraceID()
	decision := computeHash(tid[:], ts.hashSeed)&bitMaskHashBuckets < ts.hashScaledSamplerate
	return decision, nil, nil
}

func (*inconsistentCommon) updateTracestate(_ pcommon.TraceID, _ bool, _ *sampling.W3CTraceState) {
}

func (ts *traceEqualizer) decide(s ptrace.Span) (bool, *sampling.W3CTraceState, error) {
	rnd, wts, err := ts.randomnessFromSpan(s)
	if err != nil {
		return false, nil, err
	}

	should := ts.traceIDThreshold.ShouldSample(rnd)
	if should {
		// This error is unchecked by the rules of consistent probability sampling.
		// If it was sampled correctly before, and it is still sampled after this
		// decision, then the rejection threshold must be rising.
		_ = wts.OTelValue().UpdateTValueWithSampling(ts.traceIDThreshold, ts.tValueEncoding)
	}

	return should, wts, err
}

func (ts *traceProportionalizer) decide(s ptrace.Span) (bool, *sampling.W3CTraceState, error) {
	rnd, wts, err := ts.randomnessFromSpan(s)
	if err != nil {
		return false, nil, err
	}

	incoming := 1.0
	otts := wts.OTelValue()
	if tv, has := otts.TValueThreshold(); has {
		incoming = tv.Probability()
	}

	// There is a potential here for the product probability to
	// underflow, which is checked here.
	threshold, err := sampling.ProbabilityToThresholdWithPrecision(incoming*ts.ratio, ts.prec)

	if err == sampling.ErrProbabilityRange {
		// Considered valid, a case where the sampling probability
		// has fallen below the minimum supported value and simply
		// becomes unsampled.
		return false, wts, nil
	} else if err == sampling.ErrPrecisionUnderflow {
		// Considered valid, any case where precision underflow
		// occurs, use full-precision encoding.
		threshold, err = sampling.ProbabilityToThreshold(incoming * ts.ratio)
	}
	if err != nil {
		return false, wts, err
	}

	should := threshold.ShouldSample(rnd)
	if should {
		// Note: an unchecked error here, because the threshold is
		// larger by construction via `incoming*ts.ratio`, which was
		// already range-checked above.
		_ = otts.UpdateTValueWithSampling(threshold, threshold.TValue())
	}
	return should, wts, err
}

func (*zeroProbability) decide(s ptrace.Span) (bool, *sampling.W3CTraceState, error) {
	return false, nil, nil
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

				probShould, wts, err := tp.sampler.decide(s)
				if err != nil {
					tp.logger.Error("trace-state", zap.Error(err))
				}

				forceSample := priority == mustSampleSpan
				sampled := forceSample || probShould

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

				if sampled && wts != nil {
					tp.sampler.updateTracestate(s.TraceID(), probShould, wts)

					var w strings.Builder
					if err := wts.Serialize(&w); err != nil {
						tp.logger.Debug("tracestate serialize", zap.Error(err))
					}
					s.TraceState().FromRaw(w.String())
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
