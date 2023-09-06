package sampling

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling/internal/unsigned"
)

// Randomness represents individual trace randomness.
type Randomness = unsigned.Randomness

// Threshold represents sampling selectivity.
type Threshold = unsigned.Threshold

// RValueToRandomness parses a R-value.
func RValueToRandomness(s string) (Randomness, error) {
	return unsigned.RValueToRandomness(s)
}

// TValueToThreshold parses a T-value.
func TValueToThreshold(s string) (Threshold, error) {
	return unsigned.TValueToThreshold(s)
}

// ProbabilityToThreshold computes a re-usable Threshold value.
func ProbabilityToThreshold(prob float64) (Threshold, error) {
	return unsigned.ProbabilityToThreshold(prob)
}

// RandomnessFromTraceID returns the randomness using the least
// significant 56 bits of the TraceID (without consideration for
// trace flags).
func RandomnessFromTraceID(tid pcommon.TraceID) Randomness {
	return unsigned.RandomnessFromTraceID(tid)
}

// ThresholdLessThan allows comparing thresholds directly.  Smaller
// thresholds have smaller probabilities, larger adjusted counts.
func ThresholdLessThan(a, b Threshold) bool {
	return unsigned.ThresholdLessThan(a, b)
}

const MaxAdjustedCount = unsigned.MaxAdjustedCount

var (
	AlwaysSampleThreshold = unsigned.AlwaysSampleThreshold
	NeverSampleThreshold  = unsigned.NeverSampleThreshold

	AlwaysSampleTValue = AlwaysSampleThreshold.TValue()
	NeverSampleTValue  = NeverSampleThreshold.TValue()

	ErrTValueSize = unsigned.ErrTValueSize
	ErrRValueSize = unsigned.ErrRValueSize
)
