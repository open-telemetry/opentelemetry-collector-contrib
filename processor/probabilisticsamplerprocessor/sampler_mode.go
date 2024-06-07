// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

const (
	// Hashing method: The constants below help translate user friendly percentages
	// to numbers direct used in sampling.
	numHashBucketsLg2     = 14
	numHashBuckets        = 0x4000 // Using a power of 2 to avoid division.
	bitMaskHashBuckets    = numHashBuckets - 1
	percentageScaleFactor = numHashBuckets / 100.0
)

// SamplerMode controls the logic used in making a sampling decision.
// The HashSeed mode is the only mode, presently, and it is also the
// default mode.
//
// TODO: In the future, when OTEP 235 is introduced, there will be two
// new modes.
type SamplerMode string

const (
	HashSeed    SamplerMode = "hash_seed"
	DefaultMode SamplerMode = HashSeed
	modeUnset   SamplerMode = ""
)

// ErrMissingRandomness indicates no randomness source was found.
var ErrMissingRandomness = errors.New("missing randomness")

type randomnessNamer interface {
	randomness() sampling.Randomness
	policyName() string
}

type randomnessMethod sampling.Randomness

func (rm randomnessMethod) randomness() sampling.Randomness {
	return sampling.Randomness(rm)
}

type traceIDHashingMethod struct{ randomnessMethod }
type samplingPriorityMethod struct{ randomnessMethod }

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

func (samplingPriorityMethod) policyName() string {
	return "sampling_priority"
}

var _ randomnessNamer = missingRandomnessMethod{}
var _ randomnessNamer = traceIDHashingMethod{}
var _ randomnessNamer = samplingPriorityMethod{}

func newMissingRandomnessMethod() randomnessNamer {
	return missingRandomnessMethod{}
}

func isMissing(rnd randomnessNamer) bool {
	_, ok := rnd.(missingRandomnessMethod)
	return ok
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

// TODO: Placeholder interface, see #31894 for its future contents,
// will become a non-empty interface.  (Linter forces us to write "any".)
type samplingCarrier any

type dataSampler interface {
	// decide reports the result based on a probabilistic decision.
	decide(carrier samplingCarrier) sampling.Threshold

	// randomnessFromSpan extracts randomness and returns a carrier specific to traces data.
	randomnessFromSpan(s ptrace.Span) (randomness randomnessNamer, carrier samplingCarrier, err error)

	// randomnessFromLogRecord extracts randomness and returns a carrier specific to logs data.
	randomnessFromLogRecord(s plog.LogRecord) (randomness randomnessNamer, carrier samplingCarrier, err error)
}

var AllModes = []SamplerMode{HashSeed}

func (sm *SamplerMode) UnmarshalText(in []byte) error {
	switch mode := SamplerMode(in); mode {
	case HashSeed,
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

	// Logs only: name of attribute to obtain randomness
	logsTraceIDEnabled bool
}

func (th *hashingSampler) decide(_ samplingCarrier) sampling.Threshold {
	return th.tvalueThreshold
}

// neverSampler always decides false.
type neverSampler struct {
}

func (*neverSampler) decide(_ samplingCarrier) sampling.Threshold {
	return sampling.NeverSampleThreshold
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

// consistencyCheck checks for certain inconsistent inputs.
//
// if the randomness is missing, returns ErrMissingRandomness.
func consistencyCheck(rnd randomnessNamer, _ samplingCarrier) error {
	if isMissing(rnd) {
		return ErrMissingRandomness
	}
	return nil
}

// makeSample constructs a sampler. There are no errors, as the only
// potential error, out-of-range probability, is corrected automatically
// according to the README, which allows percents >100 to equal 100%.
//
// Extending this logic, we round very small probabilities up to the
// minimum supported value(s) which varies according to sampler mode.
func makeSampler(cfg *Config) dataSampler {
	// README allows percents >100 to equal 100%.
	pct := cfg.SamplingPercentage
	if pct > 100 {
		pct = 100
	}

	never := &neverSampler{}

	if pct == 0 {
		return never
	}

	// Note: the original hash function used in this code
	// is preserved to ensure consistency across updates.
	//
	//   uint32(pct * percentageScaleFactor)
	//
	// (a) carried out the multiplication in 32-bit precision
	// (b) rounded to zero instead of nearest.
	scaledSampleRate := uint32(pct * percentageScaleFactor)

	if scaledSampleRate == 0 {
		return never
	}

	// Convert the accept threshold to a reject threshold,
	// then shift it into 56-bit value.
	reject := numHashBuckets - scaledSampleRate
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

// randFunc returns randomness (w/ named policy), a carrier, and the error.
type randFunc[T any] func(T) (randomnessNamer, samplingCarrier, error)

// priorityFunc makes changes resulting from sampling priority.
type priorityFunc[T any] func(T, randomnessNamer, sampling.Threshold) (randomnessNamer, sampling.Threshold)

// commonSamplingLogic implements sampling on a per-item basis
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
) bool {
	rnd, carrier, err := randFunc(item)
	if err == nil {
		err = consistencyCheck(rnd, carrier)
	}
	var threshold sampling.Threshold
	if err != nil {
		logger.Debug(description, zap.Error(err))
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

	_ = stats.RecordWithTags(
		ctx,
		[]tag.Mutator{tag.Upsert(tagPolicyKey, rnd.policyName()), tag.Upsert(tagSampledKey, strconv.FormatBool(sampled))},
		statCountTracesSampled.M(int64(1)),
	)

	return sampled
}
