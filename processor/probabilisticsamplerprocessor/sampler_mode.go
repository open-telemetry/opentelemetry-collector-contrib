// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"fmt"
	"io"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type SamplerMode string

const (
	HashSeed     SamplerMode = "hash_seed"
	Equalizing   SamplerMode = "equalizing"
	Proportional SamplerMode = "proportional"
	DefaultMode  SamplerMode = Proportional
	modeUnset    SamplerMode = ""
)

type samplingCarrier interface {
	explicitRandomness() (sampling.Randomness, bool)
	setExplicitRandomness(sampling.Randomness)

	clearThreshold()
	threshold() (sampling.Threshold, bool)
	updateThreshold(sampling.Threshold, string) error

	serialize(io.StringWriter) error
}

type dataSampler interface {
	// decide reports the result based on a probabilistic decision.
	decide(rnd sampling.Randomness, carrier samplingCarrier) sampling.Threshold

	// randomnessFromSpan extracts randomness and returns a carrier specific to traces data.
	randomnessFromSpan(s ptrace.Span) (randomness sampling.Randomness, carrier samplingCarrier, err error)

	// randomnessFromLogRecord extracts randomness and returns a carrier specific to logs data.
	randomnessFromLogRecord(s plog.LogRecord) (randomness sampling.Randomness, carrier samplingCarrier, err error)
}

var (
	ErrInconsistentArrivingTValue = fmt.Errorf("inconsistent arriving t-value: span should not have been sampled")
	ErrMissingRandomness          = fmt.Errorf("missing randomness; trace flag not set")
)

var AllModes = []SamplerMode{HashSeed, Equalizing, Proportional}

func (sm *SamplerMode) UnmarshalText(in []byte) error {
	switch mode := SamplerMode(in); mode {
	case HashSeed,
		Equalizing,
		Proportional,
		modeUnset:
		*sm = mode
		return nil
	default:
		return fmt.Errorf("unsupported sampler mode %q", mode)
	}
}

// commonFields includes fields used in all sampler modes.
type commonFields struct {
	// strict detetrmines how strongly randomness is enforced
	strict bool

	logger *zap.Logger
}

// hashingSampler is the original hash-based calculation.  It is an
// equalizing sampler with randomness calculation that matches the
// original implementation.  This hash-based implementation is limited
// to 14 bits of precision.
type hashingSampler struct {
	hashSeed        uint32
	tvalueThreshold sampling.Threshold

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

// neverSampler always decides false.
type neverSampler struct {
	consistentTracestateCommon
}

// equalizingSampler adjusts thresholds absolutely.  Cannot be used with zero.
type equalizingSampler struct {
	// TraceID-randomness-based calculation
	tvalueThreshold sampling.Threshold

	consistentTracestateCommon
}

// proportionalSampler adjusts thresholds relatively.  Cannot be used with zero.
type proportionalSampler struct {
	// ratio in the range [2**-56, 1]
	ratio float64

	// prec is the precision in number of hex digits
	prec int

	consistentTracestateCommon
}

func (th *hashingSampler) randomnessFromLogRecord(l plog.LogRecord) (sampling.Randomness, samplingCarrier, error) {
	// TBD@@@
	panic("nope")
	//return sampling.Randomness{}, nil, nil
}

func (th *hashingSampler) randomnessFromSpan(s ptrace.Span) (sampling.Randomness, samplingCarrier, error) {
	tid := s.TraceID()
	hashed32 := computeHash(tid[:], th.hashSeed)
	hashed := uint64(hashed32 & bitMaskHashBuckets)

	// Ordinarily, hashed is compared against an acceptance
	// threshold i.e., sampled when hashed < scaledSamplerate,
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
		// When no sampling information is present, add an R-value.  The threshold
		// will be added following the decision.
		tsc.setExplicitRandomness(rnd)
	}
	return rnd, tsc, nil
}

func (ctc *consistentTracestateCommon) randomnessFromLogRecord(l plog.LogRecord) (sampling.Randomness, samplingCarrier, error) {
	// @@@
	panic("nope")
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
		} else if ctc.strict && s.TraceID().IsEmpty() {
			// If strict and the TraceID() is all zeros,
			// which W3C calls an invalid TraceID.
			err = ErrMissingRandomness
		} else {
			// Whether !strict or the random flag is correctly set.
			//
			// Note: We do not check TraceID().IsValid() in this case,
			// the outcome is:
			//  - R-value equals "00000000000000"
			//  - Sampled at 100% otherwise not sampled
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

// makeSample constructs a sampler. There are no errors, as the only
// potential error, out-of-range probability, is corrected automatically
// according to the README, which allows percents >100 to equal 100%.
//
// Extending this logic, we round very small probabilities up to the
// minimum supported value(s) which varies according to sampler mode.
func makeSampler(cfg *Config, common commonFields) dataSampler {
	// README allows percents >100 to equal 100%.
	pct := cfg.SamplingPercentage
	if pct > 100 {
		pct = 100
	}
	mode := cfg.SamplerMode
	if mode == modeUnset {
		if cfg.HashSeed != 0 {
			mode = HashSeed
		} else {
			mode = DefaultMode
		}
	}

	ccom := consistentCommon{
		commonFields: common,
	}
	ctcom := consistentTracestateCommon{
		consistentCommon: ccom,
	}
	never := &neverSampler{
		consistentTracestateCommon: ctcom,
	}

	if pct == 0 {
		return never
	}
	// Note: Convert to float64 before dividing by 100, otherwise loss of precision.
	// If the probability is too small, round it up to the minimum.
	ratio := float64(pct) / 100
	// Like the pct > 100 test above, but for values too small to
	// express in 14 bits of precision.
	if ratio < sampling.MinSamplingProbability {
		ratio = sampling.MinSamplingProbability
	}

	switch mode {
	case Equalizing:
		// The error case below is ignored, we have rounded the probability so
		// that it is in-range
		threshold, _ := sampling.ProbabilityToThresholdWithPrecision(ratio, cfg.SamplingPrecision)

		return &equalizingSampler{
			tvalueThreshold:            threshold,
			consistentTracestateCommon: ctcom,
		}

	case Proportional:
		return &proportionalSampler{
			ratio:                      ratio,
			prec:                       cfg.SamplingPrecision,
			consistentTracestateCommon: ctcom,
		}

	default: // i.e., HashSeed

		// Note: the original hash function used in this code
		// is preserved to ensure consistency across updates.
		//
		//   uint32(pct * percentageScaleFactor)
		//
		// (a) carried out the multiplication in 32-bit precision
		// (b) rounded to zero instead of nearest.
		scaledSamplerate := uint32(pct * percentageScaleFactor)

		if scaledSamplerate == 0 {
			ccom.logger.Warn("probability rounded to zero", zap.Float32("percent", pct))
			return never
		}

		// Convert the accept threshold to a reject threshold,
		// then shift it into 56-bit value.
		reject := numHashBuckets - scaledSamplerate
		reject56 := uint64(reject) << 42

		threshold, _ := sampling.ThresholdFromUnsigned(reject56)

		ts := &hashingSampler{
			consistentCommon: ccom,
			tvalueThreshold:  threshold,
		}

		return ts
	}
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
	// return should, nil
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

func (*neverSampler) decide(_ sampling.Randomness, _ samplingCarrier) sampling.Threshold {
	return sampling.NeverSampleThreshold
}

func (th *hashingSampler) decide(rnd sampling.Randomness, carrier samplingCarrier) sampling.Threshold {
	return th.tvalueThreshold
}

func (te *equalizingSampler) decide(rnd sampling.Randomness, carrier samplingCarrier) sampling.Threshold {
	return te.tvalueThreshold
}

func (tp *proportionalSampler) decide(rnd sampling.Randomness, carrier samplingCarrier) sampling.Threshold {
	incoming := 1.0
	if tv, has := carrier.threshold(); has {
		incoming = tv.Probability()
	}

	// There is a potential here for the product probability to
	// underflow, which is checked here.
	threshold, err := sampling.ProbabilityToThresholdWithPrecision(incoming*tp.ratio, tp.prec)

	// Check the only known error condition.
	if err == sampling.ErrProbabilityRange {
		// Considered valid, a case where the sampling probability
		// has fallen below the minimum supported value and simply
		// becomes unsampled.
		return sampling.NeverSampleThreshold
	}
	return threshold
}

// if err != nil {
// 	return threshold, err
// }

// return
// should := threshold.ShouldSample(rnd)
// if should {
// 	// Note: an unchecked error here, because the threshold is
// 	// larger by construction via `incoming*tp.ratio`, which was
// 	// already range-checked above.
// 	_ = carrier.updateThreshold(threshold, threshold.TValue())
// }
// return should, err

// @@@
// should := te.tvalueThreshold.ShouldSample(rnd)
// if should {
// 	err := carrier.updateThreshold(te.tvalueThreshold, te.tValueEncoding)
// 	if err != nil {
// 		te.logger.Warn("tracestate", zap.Error(err))
// 	}
// }

// func (*consistentCommon) update(should bool, wts samplingCarrier) {
// 	// When this sampler decided not to sample, the t-value becomes zero.
// 	if !should {
// 		wts.clearThreshold()
// 	}
// }
