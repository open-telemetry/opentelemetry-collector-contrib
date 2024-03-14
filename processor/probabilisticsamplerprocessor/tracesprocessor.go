// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"fmt"
	"io"
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

type samplingCarrier interface {
	threshold() (sampling.Threshold, bool)
	explicitRandomness() (sampling.Randomness, bool)
	updateThreshold(sampling.Threshold, string) error
	setExplicitRandomness(sampling.Randomness)
	clearThreshold()
	serialize(io.StringWriter) error
}

type dataSampler interface {
	// decide reports the result based on a probabilistic decision.
	decide(rnd sampling.Randomness, carrier samplingCarrier) (should bool, err error)

	// update modifies the item when it will be sampled,
	// probabilistically or otherwise.  The "should" parameter is
	// the result from decide().
	update(should bool, carrier samplingCarrier)

	randomnessFromSpan(s ptrace.Span) (randomness sampling.Randomness, carrier samplingCarrier, err error)
}

type traceProcessor struct {
	sampler dataSampler

	commonFields
}

// commonFields includes fields used in all sampler modes.
type commonFields struct {
	// strict detetrmines how strongly randomness is enforced
	strict bool

	logger *zap.Logger
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

	consistentCommon
}

// consistentCommon implements update() for all samplers, which clears
// the sampling threshold when probability sampling decides false.
type consistentCommon struct {
	commonFields
}

// consistentTracestateCommon includes all except the legacy hash-based
// method, which overrides randomnessFromX.
type consistentTracestateCommon struct {
	consistentCommon
}

// zeroProbability always decides false.
type zeroProbability struct {
	consistentTracestateCommon
}

// traceEqualizer adjusts thresholds absolutely.  Cannot be used with zero.
type traceEqualizer struct {
	// TraceID-randomness-based calculation
	tValueThreshold sampling.Threshold

	// tValueEncoding is the encoded string T-value representation.
	tValueEncoding string

	consistentTracestateCommon
}

// traceEqualizer adjusts thresholds relatively.  Cannot be used with zero.
type traceProportionalizer struct {
	// ratio in the range [2**-56, 1]
	ratio float64

	// prec is the precision in number of hex digits
	prec int

	consistentTracestateCommon
}

func (*consistentCommon) update(should bool, wts samplingCarrier) {
	// When this sampler decided not to sample, the t-value becomes zero.
	if !should {
		wts.clearThreshold()
	}
}

// randomnessToHashed returns the original 14-bit hash value used by
// this component.
func randomnessToHashed(rnd sampling.Randomness) uint32 {
	// By design, the least-significant bits of the unsigned value matches
	// the original hash function.
	return uint32(rnd.Unsigned() & bitMaskHashBuckets)
}

func (th *traceHasher) randomnessFromSpan(s ptrace.Span) (sampling.Randomness, samplingCarrier, error) {
	tid := s.TraceID()
	hashed32 := computeHash(tid[:], th.hashSeed)
	hashed := uint64(hashed32 & bitMaskHashBuckets)

	// Ordinarily, hashed is compared against an acceptance
	// threshold i.e., sampled when hashed < hashScaledSamplerate,
	// which has the form R < T with T in [1, 2^14] and
	// R in [0, 2^14-1].
	//
	// Here, modify R to R' and T to T', so that the sampling
	// equation has identical form to the specification, i.e., T'
	// <= R', using:
	//
	//   T' = numHashBuckets-T
	//   R' = numHashBuckets-1-R
	//
	// As a result, R' has the correct most-significant 14 bits to
	// use in an R-value.
	rprime14 := uint64(numHashBuckets - 1 - hashed)

	// There are 18 unused bits from the FNV hash function.
	unused18 := uint64(hashed32 >> (32 - numHashBucketsLg2))
	mixed28 := unused18 ^ (unused18 << 10)

	// The 56 bit quantity here consists of, most- to least-significant:
	// - 14 bits: R' = numHashBuckets - 1 - hashed
	// - 28 bits: mixture of unused 18 bits
	// - 14 bits: original `hashed`.
	rnd56 := (rprime14 << 42) | (mixed28 << 14) | hashed

	// Note: by construction:
	// - OTel samplers make the same probabilistic decision with this r-value,
	// - only 14 out of 56 bits are used in the sampling decision,
	// - there are only 32 actual random bits.
	rnd, _ := sampling.UnsignedToRandomness(rnd56)
	if th.strict {
		// In strict mode, we never parse the TraceState and let
		// it pass through untouched.
		return rnd, nil, nil
	}
	tsc := &tracestateCarrier{}

	var err error
	tsc.W3CTraceState, err = sampling.NewW3CTraceState(s.TraceState().AsRaw())
	if err != nil {
		// This failure is logged but not fatal, since the legacy
		// behavior of this sampler disregarded TraceState and
		// because we are already not strict.
		th.logger.Debug("invalid tracestate in hash_seed sampler, ignoring", zap.Error(err))
		return rnd, nil, nil
	}

	// If the tracestate contains a proper R-value or T-value, we
	// have to leave it alone.  The user should not be using this
	// sampler mode if they are using specified forms of consistent
	// sampling in OTel.
	if _, has := tsc.explicitRandomness(); has {
		th.logger.Warn("tracestate has r-value, equalizing or proportional mode recommended")
	} else if _, has := tsc.threshold(); has {
		th.logger.Warn("tracestate has t-value, equalizing or proportional mode recommended")
	} else {
		// When no sampling information is present, add an R-value
		// and T-value to convey a sampling probability.  There is no
		// error possibility, since no existing T-value.
		_ = tsc.updateThreshold(th.unstrictTValueThreshold, th.unstrictTValueEncoding)

		tsc.setExplicitRandomness(rnd)
	}
	return rnd, tsc, nil
}

func (ctc *consistentTracestateCommon) randomnessFromSpan(s ptrace.Span) (randomness sampling.Randomness, carrier samplingCarrier, err error) {
	rawts := s.TraceState().AsRaw()
	tsc := &tracestateCarrier{}

	// Parse the arriving TraceState.
	tsc.W3CTraceState, err = sampling.NewW3CTraceState(rawts)
	if err == nil {
		if rv, has := tsc.W3CTraceState.OTelValue().RValueRandomness(); has {
			// When the tracestate is OK and has r-value, use it.
			randomness = rv
		} else if ctc.strict && (s.Flags()&randomFlagValue) != randomFlagValue {
			// If strict and the flag is missing
			err = ErrMissingRandomness
		} else {
			// Whether !strict or the random flag is correctly set.
			randomness = sampling.TraceIDToRandomness(s.TraceID())
		}
	}

	return randomness, tsc, err
}

func consistencyCheck(randomness sampling.Randomness, carrier samplingCarrier, common commonFields) error {
	// Consistency check: if the TraceID is out of range, the
	// TValue is a lie.  If inconsistent, clear it and return an error.
	if tv, has := carrier.threshold(); has {
		if !tv.ShouldSample(randomness) {
			if common.strict {
				return ErrInconsistentArrivingTValue
			} else {
				common.logger.Warn("tracestate", zap.Error(ErrInconsistentArrivingTValue))
				carrier.clearThreshold()
			}
		}
	}

	return nil
}

// newTracesProcessor returns a processor.TracesProcessor that will
// perform intermediate span sampling according to the given
// configuration.
func newTracesProcessor(ctx context.Context, set processor.CreateSettings, cfg *Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	// README allows percents >100 to equal 100%, but t-value
	// encoding does not.  Correct it here.
	pct := float64(cfg.SamplingPercentage)
	if pct > 100 {
		pct = 100
	}

	common := commonFields{
		strict: cfg.StrictRandomness,
		logger: set.Logger,
	}
	ccom := consistentCommon{
		commonFields: common,
	}
	ctcom := consistentTracestateCommon{
		consistentCommon: ccom,
	}
	tp := &traceProcessor{
		commonFields: common,
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
		tp.sampler = &zeroProbability{
			consistentTracestateCommon: ctcom,
		}
	} else {
		ratio := pct / 100
		switch cfg.SamplerMode {
		case HashSeed:
			ts := &traceHasher{
				consistentCommon: ccom,
			}

			// Adjust sampling percentage on private so recalculations are avoided.
			ts.hashScaledSamplerate = uint32(pct * percentageScaleFactor)
			ts.hashSeed = cfg.HashSeed
			ts.strict = cfg.StrictRandomness

			if !ts.strict {
				threshold, err := sampling.ProbabilityToThresholdWithPrecision(ratio, cfg.SamplingPrecision)
				if err != nil {
					return nil, err
				}

				ts.unstrictTValueEncoding = threshold.TValue()
				ts.unstrictTValueThreshold = threshold
			}
			tp.sampler = ts
		case Equalizing:
			threshold, err := sampling.ProbabilityToThresholdWithPrecision(ratio, cfg.SamplingPrecision)
			if err != nil {
				return nil, err
			}

			tp.sampler = &traceEqualizer{
				tValueEncoding:             threshold.TValue(),
				tValueThreshold:            threshold,
				consistentTracestateCommon: ctcom,
			}
		case Proportional:
			tp.sampler = &traceProportionalizer{
				ratio:                      ratio,
				prec:                       cfg.SamplingPrecision,
				consistentTracestateCommon: ctcom,
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

type tracestateCarrier struct {
	sampling.W3CTraceState
}

var _ samplingCarrier = &tracestateCarrier{}

func (tc *tracestateCarrier) threshold() (sampling.Threshold, bool) {
	return tc.W3CTraceState.OTelValue().TValueThreshold()
}

func (tc *tracestateCarrier) explicitRandomness() (sampling.Randomness, bool) {
	return tc.W3CTraceState.OTelValue().RValueRandomness()
}

func (tc *tracestateCarrier) updateThreshold(th sampling.Threshold, tv string) error {
	return tc.W3CTraceState.OTelValue().UpdateTValueWithSampling(th, tv)
}

func (tc *tracestateCarrier) setExplicitRandomness(rnd sampling.Randomness) {
	tc.W3CTraceState.OTelValue().SetRValue(rnd)
}

func (tc *tracestateCarrier) clearThreshold() {
	tc.W3CTraceState.OTelValue().ClearTValue()
}

func (tc *tracestateCarrier) serialize(w io.StringWriter) error {
	return tc.W3CTraceState.Serialize(w)
}

func (*zeroProbability) decide(_ sampling.Randomness, _ samplingCarrier) (should bool, err error) {
	return false, nil
}

func (th *traceHasher) decide(rnd sampling.Randomness, carrier samplingCarrier) (bool, error) {
	hashed := randomnessToHashed(rnd)
	should := hashed < th.hashScaledSamplerate
	return should, nil
}

func (te *traceEqualizer) decide(rnd sampling.Randomness, carrier samplingCarrier) (bool, error) {
	should := te.tValueThreshold.ShouldSample(rnd)
	if should {
		err := carrier.updateThreshold(te.tValueThreshold, te.tValueEncoding)
		if err != nil {
			te.logger.Warn("tracestate", zap.Error(err))
		}
	}

	return should, nil
}

func (tp *traceProportionalizer) decide(rnd sampling.Randomness, carrier samplingCarrier) (bool, error) {
	incoming := 1.0
	if tv, has := carrier.threshold(); has {
		incoming = tv.Probability()
	}

	// There is a potential here for the product probability to
	// underflow, which is checked here.
	threshold, err := sampling.ProbabilityToThresholdWithPrecision(incoming*tp.ratio, tp.prec)

	if err == sampling.ErrProbabilityRange {
		// Considered valid, a case where the sampling probability
		// has fallen below the minimum supported value and simply
		// becomes unsampled.
		return false, nil
	}
	if err != nil {
		return false, err
	}

	should := threshold.ShouldSample(rnd)
	if should {
		// Note: an unchecked error here, because the threshold is
		// larger by construction via `incoming*tp.ratio`, which was
		// already range-checked above.
		_ = carrier.updateThreshold(threshold, threshold.TValue())
	}
	return should, err
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
				// probShould is the probabilistic decision
				var probShould bool
				var toUpdate samplingCarrier

				// forceSample is the sampling.priority decision
				forceSample := priority == mustSampleSpan

				if rnd, carrier, err := tp.sampler.randomnessFromSpan(s); err != nil {
					tp.logger.Error("tracestate", zap.Error(err))
				} else if err = consistencyCheck(rnd, carrier, tp.commonFields); err != nil {
					tp.logger.Error("tracestate", zap.Error(err))
				} else if probShould, err = tp.sampler.decide(rnd, carrier); err != nil {
					tp.logger.Error("tracestate", zap.Error(err))
				} else {
					toUpdate = carrier
				}
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

				if sampled && toUpdate != nil {

					tp.sampler.update(probShould, toUpdate)

					var w strings.Builder
					if err := toUpdate.serialize(&w); err != nil {
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
