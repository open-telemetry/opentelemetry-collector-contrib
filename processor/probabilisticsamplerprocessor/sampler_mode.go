// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

const (
	// These four can happen at runtime and be returned by
	// randomnessFromXXX()

	ErrInconsistentArrivingTValue samplerError = "inconsistent arriving threshold: item should not have been sampled"
	ErrMissingRandomness          samplerError = "missing randomness"
	ErrRandomnessInUse            samplerError = "item has sampling randomness, equalizing or proportional mode recommended"
	ErrThresholdInUse             samplerError = "item has sampling threshold, equalizing or proportional mode recommended"
)

const (
	// Hashing method: The constants below help translate user friendly percentages
	// to numbers direct used in sampling.
	numHashBucketsLg2     = 14
	numHashBuckets        = 0x4000 // Using a power of 2 to avoid division.
	bitMaskHashBuckets    = numHashBuckets - 1
	percentageScaleFactor = numHashBuckets / 100.0
)

// samplerErrors are conditions reported by the sampler that are somewhat
// ordinary and should log as info-level.
type samplerError string

var _ error = samplerError("")

func (s samplerError) Error() string {
	return string(s)
}

// SamplerMode determines which of several modes is used for the
// sampling decision.
type SamplerMode string

const (
	// HashSeed applies the hash/fnv hash function originally used in this component.
	HashSeed SamplerMode = "hash_seed"

	// Equalizing uses OpenTelemetry consistent probability
	// sampling information (OTEP 235), applies an absolute
	// threshold to equalize incoming sampling probabilities.
	Equalizing SamplerMode = "equalizing"

	// Proportional uses OpenTelemetry consistent probability
	// sampling information (OTEP 235), multiplies incoming
	// sampling probabilities.
	Proportional SamplerMode = "proportional"

	// defaultHashSeed is applied when the mode is unset.
	defaultMode SamplerMode = HashSeed

	// modeUnset indicates the user has not configured the mode.
	modeUnset SamplerMode = ""
)

type randomnessNamer interface {
	randomness() sampling.Randomness
	policyName() string
}

type randomnessMethod sampling.Randomness

func (rm randomnessMethod) randomness() sampling.Randomness {
	return sampling.Randomness(rm)
}

type (
	traceIDHashingMethod     struct{ randomnessMethod }
	traceIDW3CSpecMethod     struct{ randomnessMethod }
	samplingRandomnessMethod struct{ randomnessMethod }
	samplingPriorityMethod   struct{ randomnessMethod }
)

type missingRandomnessMethod struct{}

func (rm missingRandomnessMethod) randomness() sampling.Randomness {
	return sampling.AllProbabilitiesRandomness
}

func (missingRandomnessMethod) policyName() string {
	return "missing_randomness"
}

type attributeHashingMethod struct {
	randomnessMethod
	attribute string
}

func (am attributeHashingMethod) policyName() string {
	return am.attribute
}

func (traceIDHashingMethod) policyName() string {
	return "trace_id_hash"
}

func (samplingRandomnessMethod) policyName() string {
	return "sampling_randomness"
}

func (traceIDW3CSpecMethod) policyName() string {
	return "trace_id_w3c"
}

func (samplingPriorityMethod) policyName() string {
	return "sampling_priority"
}

var (
	_ randomnessNamer = missingRandomnessMethod{}
	_ randomnessNamer = traceIDHashingMethod{}
	_ randomnessNamer = traceIDW3CSpecMethod{}
	_ randomnessNamer = samplingRandomnessMethod{}
	_ randomnessNamer = samplingPriorityMethod{}
)

func newMissingRandomnessMethod() randomnessNamer {
	return missingRandomnessMethod{}
}

func isMissing(rnd randomnessNamer) bool {
	_, ok := rnd.(missingRandomnessMethod)
	return ok
}

func newSamplingRandomnessMethod(rnd sampling.Randomness) randomnessNamer {
	return samplingRandomnessMethod{randomnessMethod(rnd)}
}

func newTraceIDW3CSpecMethod(rnd sampling.Randomness) randomnessNamer {
	return traceIDW3CSpecMethod{randomnessMethod(rnd)}
}

func newTraceIDHashingMethod(rnd sampling.Randomness) randomnessNamer {
	return traceIDHashingMethod{randomnessMethod(rnd)}
}

func newSamplingPriorityMethod(rnd sampling.Randomness) randomnessNamer {
	return samplingPriorityMethod{randomnessMethod(rnd)}
}

func newAttributeHashingMethod(attribute string, rnd sampling.Randomness) randomnessNamer {
	return attributeHashingMethod{
		randomnessMethod: randomnessMethod(rnd),
		attribute:        attribute,
	}
}

// samplingCarrier conveys information about the underlying data item
// (whether span or log record) through the sampling decision.
type samplingCarrier interface {
	// explicitRandomness returns a randomness value and a boolean
	// indicating whether the item had sampling randomness
	// explicitly set.
	explicitRandomness() (randomnessNamer, bool)

	// setExplicitRandomness updates the item with the signal-specific
	// encoding for an explicit randomness value.
	setExplicitRandomness(randomnessNamer)

	// clearThreshold unsets a sampling threshold, which is used to
	// clear information that breaks the expected sampling invariants
	// described in OTEP 235.
	clearThreshold()

	// threshold returns a sampling threshold and a boolean
	// indicating whether the item had sampling threshold
	// explicitly set.
	threshold() (sampling.Threshold, bool)

	// updateThreshold modifies the sampling threshold.  This
	// returns an error if the updated sampling threshold has a
	// lower adjusted account; the only permissible updates raise
	// adjusted count (i.e., reduce sampling probability).
	updateThreshold(sampling.Threshold) error

	// reserialize re-encodes the updated sampling information
	// into the item, if necessary.  For Spans, this re-encodes
	// the tracestate. This is a no-op for logs records.
	reserialize() error
}

// dataSampler implements the logic of a sampling mode.
type dataSampler interface {
	// decide reports the result based on a probabilistic decision.
	decide(carrier samplingCarrier) sampling.Threshold

	// randomnessFromSpan extracts randomness and returns a carrier specific to traces data.
	randomnessFromSpan(s ptrace.Span) (randomness randomnessNamer, carrier samplingCarrier, err error)

	// randomnessFromLogRecord extracts randomness and returns a carrier specific to logs data.
	randomnessFromLogRecord(s plog.LogRecord) (randomness randomnessNamer, carrier samplingCarrier, err error)
}

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

// hashingSampler is the original hash-based calculation.  It is an
// equalizing sampler with randomness calculation that matches the
// original implementation.  This hash-based implementation is limited
// to 14 bits of precision.
type hashingSampler struct {
	hashSeed        uint32
	tvalueThreshold sampling.Threshold

	// Logs only: name of attribute to obtain randomness
	logsRandomnessSourceAttribute string

	// Logs only: whether traceID is being used
	logsTraceIDEnabled bool
}

func (th *hashingSampler) decide(_ samplingCarrier) sampling.Threshold {
	return th.tvalueThreshold
}

// consistentTracestateCommon contains the common aspects of the
// Proportional and Equalizing sampler modes.  These samplers sample
// using the TraceID and do not support use of logs source attribute.
type consistentTracestateCommon struct{}

// neverSampler always decides false.
type neverSampler struct{}

func (*neverSampler) decide(_ samplingCarrier) sampling.Threshold {
	return sampling.NeverSampleThreshold
}

// equalizingSampler raises thresholds up to a fixed value.
type equalizingSampler struct {
	// TraceID-randomness-based calculation
	tvalueThreshold sampling.Threshold

	consistentTracestateCommon
}

func (te *equalizingSampler) decide(carrier samplingCarrier) sampling.Threshold {
	if tv, has := carrier.threshold(); has && sampling.ThresholdLessThan(te.tvalueThreshold, tv) {
		return tv
	}
	return te.tvalueThreshold
}

// proportionalSampler raises thresholds relative to incoming value.
type proportionalSampler struct {
	// ratio in the range [2**-56, 1]
	ratio float64

	// precision is the precision in number of hex digits
	precision int

	consistentTracestateCommon
}

func (tp *proportionalSampler) decide(carrier samplingCarrier) sampling.Threshold {
	incoming := 1.0
	if tv, has := carrier.threshold(); has {
		incoming = tv.Probability()
	}

	// There is a potential here for the product probability to
	// underflow, which is checked here.
	threshold, err := sampling.ProbabilityToThresholdWithPrecision(incoming*tp.ratio, tp.precision)

	// Check the only known error condition.
	if errors.Is(err, sampling.ErrProbabilityRange) {
		// Considered valid, a case where the sampling probability
		// has fallen below the minimum supported value and simply
		// becomes unsampled.
		return sampling.NeverSampleThreshold
	}
	return threshold
}

func getBytesFromValue(value pcommon.Value) []byte {
	if value.Type() == pcommon.ValueTypeBytes {
		return value.Bytes().AsRaw()
	}
	return []byte(value.AsString())
}

func randomnessFromBytes(b []byte, hashSeed uint32) sampling.Randomness {
	hashed32 := computeHash(b, hashSeed)
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
	rprime14 := numHashBuckets - 1 - hashed

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
	return rnd
}

func consistencyCheck(rnd randomnessNamer, carrier samplingCarrier) error {
	// Without randomness, do not check the threshold.
	if isMissing(rnd) {
		return ErrMissingRandomness
	}
	// When the carrier is nil, it means there was trouble parsing the
	// tracestate or trace-related attributes.  In this case, skip the
	// consistency check.
	if carrier == nil {
		return nil
	}
	// Consistency check: if the TraceID is out of range, the
	// TValue is a lie.  If inconsistent, clear it and return an error.
	if tv, has := carrier.threshold(); has {
		if !tv.ShouldSample(rnd.randomness()) {
			// In case we fail open, the threshold is cleared as
			// recommended in the OTel spec.
			carrier.clearThreshold()
			return ErrInconsistentArrivingTValue
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
func makeSampler(cfg *Config, isLogs bool) dataSampler {
	// README allows percents >100 to equal 100%.
	pct := cfg.SamplingPercentage
	if pct > 100 {
		pct = 100
	}
	mode := cfg.Mode
	if mode == modeUnset {
		// Reasons to choose the legacy behavior include:
		// (a) having set the hash seed
		// (b) logs signal w/o trace ID source
		if cfg.HashSeed != 0 || (isLogs && cfg.AttributeSource != traceIDAttributeSource) {
			mode = HashSeed
		} else {
			mode = defaultMode
		}
	}

	if pct == 0 {
		return &neverSampler{}
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
			tvalueThreshold: threshold,
		}

	case Proportional:
		return &proportionalSampler{
			ratio:     ratio,
			precision: cfg.SamplingPrecision,
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
			return &neverSampler{}
		}

		// Convert the accept threshold to a reject threshold,
		// then shift it into 56-bit value.
		reject := numHashBuckets - scaledSamplerate
		reject56 := uint64(reject) << 42

		threshold, _ := sampling.UnsignedToThreshold(reject56)

		return &hashingSampler{
			tvalueThreshold: threshold,
			hashSeed:        cfg.HashSeed,

			// Logs specific:
			logsTraceIDEnabled:            cfg.AttributeSource == traceIDAttributeSource,
			logsRandomnessSourceAttribute: cfg.FromAttribute,
		}
	}
}

// randFunc returns randomness (w/ named policy), a carrier, and the error.
type randFunc[T any] func(T) (randomnessNamer, samplingCarrier, error)

// priorityFunc makes changes resulting from sampling priority.
type priorityFunc[T any] func(T, randomnessNamer, sampling.Threshold) (randomnessNamer, sampling.Threshold)

// commonShouldSampleLogic implements sampling on a per-item basis
// independent of the signal type, as embodied in the functional
// parameters:
func commonShouldSampleLogic[T any](
	ctx context.Context,
	item T,
	sampler dataSampler,
	failClosed bool,
	randFunc randFunc[T],
	priorityFunc priorityFunc[T],
	description string,
	logger *zap.Logger,
	counter metric.Int64Counter,
) bool {
	rnd, carrier, err := randFunc(item)

	if err == nil {
		err = consistencyCheck(rnd, carrier)
	}
	var threshold sampling.Threshold
	if err != nil {
		var se samplerError
		if errors.As(err, &se) {
			logger.Debug(description, zap.Error(err))
		} else {
			logger.Info(description, zap.Error(err))
		}
		if failClosed {
			threshold = sampling.NeverSampleThreshold
		} else {
			threshold = sampling.AlwaysSampleThreshold
		}
	} else {
		threshold = sampler.decide(carrier)
	}

	rnd, threshold = priorityFunc(item, rnd, threshold)

	sampled := threshold.ShouldSample(rnd.randomness())

	if sampled && carrier != nil {
		// Note: updateThreshold limits loss of adjusted count, by
		// preventing the threshold from being lowered, only allowing
		// probability to fall and never to rise.
		if err := carrier.updateThreshold(threshold); err != nil {
			if errors.Is(err, sampling.ErrInconsistentSampling) {
				// This is working-as-intended.  You can't lower
				// the threshold, it's illogical.
				logger.Debug(description, zap.Error(err))
			} else {
				logger.Info(description, zap.Error(err))
			}
		}
		if err := carrier.reserialize(); err != nil {
			logger.Info(description, zap.Error(err))
		}
	}

	counter.Add(ctx, 1, metric.WithAttributes(attribute.String("policy", rnd.policyName()), attribute.String("sampled", strconv.FormatBool(sampled))))

	return sampled
}
