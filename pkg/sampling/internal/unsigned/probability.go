package unsigned

import (
	"errors"
	"math"
)

// ErrProbabilityRange is returned when a value should be in the range [1/MaxAdjustedCount, 1].
var ErrProbabilityRange = errors.New("sampling probability out of range (0x1p-56 <= valid <= 1)")

// probabilityInRange tests MinSamplingProb <= prob <= 1.
func probabilityInRange(prob float64) bool {
	return prob >= 1/MaxAdjustedCount && prob <= 1
}

func ProbabilityToThreshold(prob float64) (Threshold, error) {
	// Probability cases
	switch {
	case prob == 1:
		return AlwaysSampleThreshold, nil
	case prob == 0:
		return NeverSampleThreshold, nil
	case !probabilityInRange(prob):
		return AlwaysSampleThreshold, ErrProbabilityRange
	}
	unsigned := uint64(math.Round(prob * MaxAdjustedCount))
	return Threshold{
		unsigned: unsigned,
	}, nil
}

// Probability is the sampling ratio in the range [MinSamplingProb, 1].
func (t Threshold) Probability() float64 {
	return float64(t.unsigned) / MaxAdjustedCount
}
