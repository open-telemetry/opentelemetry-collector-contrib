// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"errors"
	"math"
)

// ErrProbabilityRange is returned when a value should be in the range [1/MaxAdjustedCount, 1].
var ErrProbabilityRange = errors.New("sampling probability out of range (0x1p-56 <= valid <= 1)")

// MinSamplingProbability is the smallest representable probability
// and is the inverse of MaxAdjustedCount.
const MinSamplingProbability = 1.0 / MaxAdjustedCount

// probabilityInRange tests MinSamplingProb <= prob <= 1.
func probabilityInRange(prob float64) bool {
	return prob >= MinSamplingProbability && prob <= 1
}

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

// Probability is the sampling ratio in the range [MinSamplingProb, 1].
func (t Threshold) Probability() float64 {
	return float64(MaxAdjustedCount-t.unsigned) / MaxAdjustedCount
}
