// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"errors"
	"math"
)

// ErrProbabilityRange is returned when a value should be in the range [1/MaxAdjustedCount, 1].
var ErrProbabilityRange = errors.New("sampling probability out of the range [1/MaxAdjustedCount, 1]")

// ErrPrecisionUnderflow is returned when a precision is too great for the range.
var ErrPrecisionUnderflow = errors.New("sampling precision is too great for the range")

// MinSamplingProbability is the smallest representable probability
// and is the inverse of MaxAdjustedCount.
const MinSamplingProbability = 1.0 / MaxAdjustedCount

// probabilityInRange tests MinSamplingProb <= prob <= 1.
func probabilityInRange(prob float64) bool {
	return prob >= MinSamplingProbability && prob <= 1
}

// ProbabilityToThreshold converts a probability to a Threshold.  It
// returns an error when the probability is out-of-range.
func ProbabilityToThreshold(prob float64) (Threshold, error) {
	// Probability cases
	if !probabilityInRange(prob) {
		return AlwaysSampleThreshold, ErrProbabilityRange
	}

	scaled := uint64(math.Round(prob * MaxAdjustedCount))

	return Threshold{
		unsigned: MaxAdjustedCount - scaled,
	}, nil
}

// ProbabilityToThresholdWithPrecision is like ProbabilityToThreshold
// with support for reduced precision.  The `prec` argument determines
// how many significant hex digits will be used to encode the exact
// probability.
func ProbabilityToThresholdWithPrecision(prob float64, prec uint8) (Threshold, error) {
	// Assume full precision at 0.
	if prec == 0 {
		return ProbabilityToThreshold(prob)
	}
	if !probabilityInRange(prob) {
		return AlwaysSampleThreshold, ErrProbabilityRange
	}
	// Special case for prob == 1.  The logic for revising precision
	// that follows requires 0 < 1 - prob < 1.
	if prob == 1 {
		return AlwaysSampleThreshold, nil
	}

	// Adjust precision considering the significance of leading
	// zeros.  If we can multiply the rejection probability by 16
	// and still be less than 1, then there is a leading zero of
	// obligatory precision.
	for reject := 1 - prob; reject*16 < 1; {
		reject *= 16
		prec++
	}

	// Check if leading zeros plus precision is above the maximum.
	// This is called underflow because the requested precision
	// leads to complete no significant figures.
	if prec > NumHexDigits {
		return AlwaysSampleThreshold, ErrPrecisionUnderflow
	}

	scaled := uint64(math.Round(prob * MaxAdjustedCount))
	rscaled := MaxAdjustedCount - scaled
	shift := 4 * (14 - prec)
	half := uint64(1) << (shift - 1)

	rscaled += half
	rscaled >>= shift
	rscaled <<= shift

	return Threshold{
		unsigned: rscaled,
	}, nil
}

// Probability is the sampling ratio in the range [MinSamplingProb, 1].
func (t Threshold) Probability() float64 {
	return float64(MaxAdjustedCount-t.unsigned) / MaxAdjustedCount
}
