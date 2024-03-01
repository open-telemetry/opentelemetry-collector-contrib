// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"bytes"
	"context"
	"encoding/binary"
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
	numHashBucketsLg2     = 14
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

// samplerCommon includes fields used in all sampler modes.
type samplerCommon struct {
	// strict detetrmines how strongly randomness is enforced
	strict bool

	logger *zap.Logger
}

// inconsistentCommon implements updateTracestate() for samplers that
// do not use OTel consistent sampling.
type inconsistentCommon struct {
	samplerCommon
}

// traceHasher is the original hash-based implementation.
type traceHasher struct {
	// Hash-based calculation
	hashScaledSamplerate uint32
	hashSeed             uint32

	// When not strict, this sampler inserts T-value and R-value
	// to convey consistent sampling probability.
	strict                  bool
	unstrictTValueThreshold sampling.Threshold
	unstrictTValueEncoding  string

	inconsistentCommon
}

// zeroProbability is a bypass for all cases with Percent==0.
type zeroProbability struct {
	inconsistentCommon
}

// inconsistentCommon implements updateTracestate() for samplers that
// use OTel consistent sampling.
type consistentCommon struct {
	samplerCommon
}

// traceEqualizer adjusts thresholds absolutely.  Cannot be used with zero.
type traceEqualizer struct {
	// TraceID-randomness-based calculation
	tValueThreshold sampling.Threshold

	// tValueEncoding is the encoded string T-value representation.
	tValueEncoding string

	consistentCommon
}

// traceEqualizer adjusts thresholds relatively.  Cannot be used with zero.
type traceProportionalizer struct {
	// ratio in the range [2**-56, 1]
	ratio float64

	// prec is the precision in number of hex digits
	prec uint8

	consistentCommon
}

func (*inconsistentCommon) updateTracestate(_ pcommon.TraceID, _ bool, _ *sampling.W3CTraceState) {
}

func (*consistentCommon) updateTracestate(tid pcommon.TraceID, should bool, wts *sampling.W3CTraceState) {
	// When this sampler decided not to sample, the t-value becomes zero.
	if !should {
		wts.OTelValue().ClearTValue()
	}
}

func (sc *samplerCommon) randomnessFromSpan(s ptrace.Span) (sampling.Randomness, *sampling.W3CTraceState, error) {
	state := s.TraceState()
	raw := state.AsRaw()

	// Parse the arriving TraceState.
	wts, err := sampling.NewW3CTraceState(raw)
	var randomness sampling.Randomness
	if err == nil {
		if rv, has := wts.OTelValue().RValueRandomness(); has {
			// When the tracestate is OK and has r-value, use it.
			randomness = rv
		} else if sc.strict && (s.Flags()&randomFlagValue) != randomFlagValue {
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
				if sc.strict {
					err = ErrInconsistentArrivingTValue
				} else {
					sc.logger.Warn("tracestate", zap.Error(ErrInconsistentArrivingTValue))
					otts.ClearTValue()
				}
			}
		}
	}

	return randomness, &wts, err
}

// safeProbToThresholdWithPrecision avoids the ErrPrecisionUnderflow
// condition and falls back to use of full precision in certain corner cases.
func safeProbToThresholdWithPrecision(ratio float64, prec uint8) (sampling.Threshold, error) {
	th, err := sampling.ProbabilityToThresholdWithPrecision(ratio, prec)
	if err == sampling.ErrPrecisionUnderflow {
		// Use full-precision encoding.
		th, err = sampling.ProbabilityToThreshold(ratio)
	}
	return th, err
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

	common := samplerCommon{
		strict: cfg.StrictRandomness,
		logger: set.Logger,
	}

	if pct == 0 {
		tp.sampler = &zeroProbability{
			inconsistentCommon: inconsistentCommon{
				samplerCommon: common,
			},
		}
	} else {
		ratio := pct / 100
		switch cfg.SamplerMode {
		case HashSeed:
			ts := &traceHasher{
				inconsistentCommon: inconsistentCommon{
					samplerCommon: common,
				},
			}

			// Adjust sampling percentage on private so recalculations are avoided.
			ts.hashScaledSamplerate = uint32(pct * percentageScaleFactor)
			ts.hashSeed = cfg.HashSeed
			ts.strict = cfg.StrictRandomness

			if !ts.strict {
				threshold, err := safeProbToThresholdWithPrecision(ratio, cfg.SamplingPrecision)
				if err != nil {
					return nil, err
				}

				ts.unstrictTValueEncoding = threshold.TValue()
				ts.unstrictTValueThreshold = threshold
			}
			tp.sampler = ts
		case Equalizing:
			threshold, err := safeProbToThresholdWithPrecision(ratio, cfg.SamplingPrecision)
			if err != nil {
				return nil, err
			}

			tp.sampler = &traceEqualizer{
				tValueEncoding:  threshold.TValue(),
				tValueThreshold: threshold,
				consistentCommon: consistentCommon{
					samplerCommon: common,
				},
			}
		case Proportional:
			tp.sampler = &traceProportionalizer{
				ratio: ratio,
				prec:  cfg.SamplingPrecision,
				consistentCommon: consistentCommon{
					samplerCommon: common,
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

func (th *traceHasher) decide(s ptrace.Span) (bool, *sampling.W3CTraceState, error) {
	// In this mode, we do not assume the trace ID is random, unlike the
	// other two modes (depending on strictness, and according to the OTel
	// specification).
	tid := s.TraceID()
	hashed := computeHash(tid[:], th.hashSeed) & bitMaskHashBuckets
	should := hashed < th.hashScaledSamplerate

	// In non-strict mode, insert T-value and R-value to mimic the operation
	// of an OTel consistent probability sampler.
	var retWts *sampling.W3CTraceState
	if should && !th.strict {
		// The body of this function does not change the return result,
		// and errors are suppressed.
		retWts = func() *sampling.W3CTraceState {
			// The decision, supposedly independent of past decisions,
			// can be incorporated into the decision, provided:
			// - we are not strict,
			_, wts, err := th.randomnessFromSpan(s)

			// Since we're already being non-strict, and since we have an invalid
			// tracestate, we'll just leave the tracestate alone.
			if err != nil {
				th.logger.Debug("invalid tracestate in hash_seed sampler, ignoring", zap.Error(err))
				return nil
			}
			otts := wts.OTelValue()

			// If the tracestate contains a proper R-value or T-value, we
			// have to leave it alone.  The user should not be using this
			// sampler mode if they are using specified forms of consistent
			// sampling in OTel.
			if _, has := otts.RValueRandomness(); has {
				th.logger.Warn("tracestate has r-value, equalizing or proportional mode recommended")
				return nil
			}
			if _, has := otts.TValueThreshold(); has {
				th.logger.Warn("tracestate has t-value, equalizing or proportional mode recommended")
				return nil
			}

			// When no sampling information is present, add an R-value
			// and T-value to convey a sampling probability.
			_ = otts.UpdateTValueWithSampling(th.unstrictTValueThreshold, th.unstrictTValueEncoding)

			// Place the 32 bits we have into position 9-13, to
			// form a (32-bits-significant) R-value.
			var tid bytes.Buffer
			// 8 bytes of 0s, these aren't used
			_ = binary.Write(&tid, binary.BigEndian, uint64(0))

			// count rejections
			reject := uint64(numHashBuckets - hashed)

			// only 14 bits of randomness are used
			unusedBits := 32 - numHashBucketsLg2

			// shift the most-significant bit into the
			// most-significant position.
			_ = binary.Write(&tid, binary.BigEndian, reject<<(24+unusedBits))

			rnd := sampling.TraceIDToRandomness(pcommon.TraceID(tid.Bytes()))
			otts.SetRValue(rnd)

			// Return this to modify the span.
			return wts
		}()
	}
	return should, retWts, nil
}

func (te *traceEqualizer) decide(s ptrace.Span) (bool, *sampling.W3CTraceState, error) {
	rnd, wts, err := te.randomnessFromSpan(s)
	if err != nil {
		return false, nil, err
	}

	should := te.tValueThreshold.ShouldSample(rnd)
	if should {
		// This error is unchecked by the rules of consistent probability sampling.
		// If it was sampled correctly before, and it is still sampled after this
		// decision, then the rejection threshold must be rising.
		_ = wts.OTelValue().UpdateTValueWithSampling(te.tValueThreshold, te.tValueEncoding)
	}

	return should, wts, err
}

func (tp *traceProportionalizer) decide(s ptrace.Span) (bool, *sampling.W3CTraceState, error) {
	rnd, wts, err := tp.randomnessFromSpan(s)
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
	threshold, err := safeProbToThresholdWithPrecision(incoming*tp.ratio, tp.prec)

	if err == sampling.ErrProbabilityRange {
		// Considered valid, a case where the sampling probability
		// has fallen below the minimum supported value and simply
		// becomes unsampled.
		return false, wts, nil
	}
	if err != nil {
		return false, wts, err
	}

	should := threshold.ShouldSample(rnd)
	if should {
		// Note: an unchecked error here, because the threshold is
		// larger by construction via `incoming*tp.ratio`, which was
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
					tp.logger.Error("tracestate", zap.Error(err))
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
