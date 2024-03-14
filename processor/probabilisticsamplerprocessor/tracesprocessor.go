// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"bytes"
	"context"
	"encoding/binary"
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

type dataItem interface {
	getCarrier() (sampling.Randomness, samplingCarrier, error)
}

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

// inconsistentCommon implements update() for samplers that
// use OTel consistent sampling.
type consistentCommon struct {
	commonFields
}

// zeroProbability is a bypass for all cases with Percent==0.
type zeroProbability struct {
	consistentCommon
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
	prec int

	consistentCommon
}

func (*zeroProbability) update(_ bool, _ samplingCarrier) {
}

func (*consistentCommon) update(should bool, wts samplingCarrier) {
	// When this sampler decided not to sample, the t-value becomes zero.
	if !should {
		wts.clearThreshold()
	}
}

func randomnessFromSpan(s ptrace.Span, common commonFields) (randomness sampling.Randomness, carrier samplingCarrier, err error) {
	state := s.TraceState()
	raw := state.AsRaw()
	tsc := &tracestateCarrier{}

	// Parse the arriving TraceState.
	tsc.W3CTraceState, err = sampling.NewW3CTraceState(raw)
	if err == nil {
		if rv, has := tsc.W3CTraceState.OTelValue().RValueRandomness(); has {
			// When the tracestate is OK and has r-value, use it.
			randomness = rv
		} else if common.strict && (s.Flags()&randomFlagValue) != randomFlagValue {
			// If strict and the flag is missing
			err = ErrMissingRandomness
		} else {
			// Whether !strict or the random flag is correctly set.
			randomness = sampling.TraceIDToRandomness(s.TraceID())
		}
	}

	return randomness, tsc, err
}

// The body of this function does not change the return result,
// and errors are suppressed.

// The decision, supposedly independent of past decisions,
// can be incorporated into the decision, provided:
// - we are not strict, @@@
// _, wts, err := th.randomnessFromSpan(s)

// Since we're already being non-strict, and since we have an invalid
// tracestate, we'll just leave the tracestate alone.
// if err != nil { @@@
// 	th.logger.Debug("invalid tracestate in hash_seed sampler, ignoring", zap.Error(err))
// 	return nil
// }

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
			consistentCommon: consistentCommon{
				commonFields: common,
			},
		}
	} else {
		ratio := pct / 100
		switch cfg.SamplerMode {
		case HashSeed:
			ts := &traceHasher{
				consistentCommon: consistentCommon{
					commonFields: common,
				},
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
				tValueEncoding:  threshold.TValue(),
				tValueThreshold: threshold,
				consistentCommon: consistentCommon{
					commonFields: common,
				},
			}
		case Proportional:
			tp.sampler = &traceProportionalizer{
				ratio: ratio,
				prec:  cfg.SamplingPrecision,
				consistentCommon: consistentCommon{
					commonFields: common,
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

func randomnessToHashed(rnd sampling.Randomness) uint32 {
	//hashed := computeHash(tid[:], th.hashSeed) & bitMaskHashBuckets
	return uint32(rnd.Unsigned() >> (56 - numHashBucketsLg2))
}

func (th *traceHasher) decide(rnd sampling.Randomness, carrier samplingCarrier) (bool, error) {
	// In this mode, we do not assume the trace ID is random, unlike the
	// other two modes (depending on strictness, and according to the OTel
	// specification).
	hashed := randomnessToHashed(rnd)
	should := hashed < th.hashScaledSamplerate

	// In non-strict mode, insert T-value and R-value to mimic the operation
	// of an OTel consistent probability sampler.
	if should && !th.strict {
		// If the tracestate contains a proper R-value or T-value, we
		// have to leave it alone.  The user should not be using this
		// sampler mode if they are using specified forms of consistent
		// sampling in OTel.
		if _, has := carrier.explicitRandomness(); has {
			th.logger.Warn("tracestate has r-value, equalizing or proportional mode recommended")
		} else if _, has := carrier.threshold(); has {
			th.logger.Warn("tracestate has t-value, equalizing or proportional mode recommended")
		} else {

			// When no sampling information is present, add an R-value
			// and T-value to convey a sampling probability.
			_ = carrier.updateThreshold(th.unstrictTValueThreshold, th.unstrictTValueEncoding)

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
			carrier.setExplicitRandomness(rnd)
		}
	}
	return should, nil
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

func (*zeroProbability) decide(_ sampling.Randomness, _ samplingCarrier) (should bool, err error) {
	return false, nil
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

				if rnd, carrier, err := randomnessFromSpan(s, tp.commonFields); err != nil {
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
