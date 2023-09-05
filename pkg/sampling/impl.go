package sampling

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling/internal/unsigned"
)

type Randomness = unsigned.Randomness
type Threshold = unsigned.Threshold

func RValueToRandomness(s string) (Randomness, error) {
	return unsigned.RValueToRandomness(s)
}

func TValueToThreshold(s string) (Threshold, error) {
	return unsigned.TValueToThreshold(s)
}

func ProbabilityToThreshold(prob float64) (Threshold, error) {
	return unsigned.ProbabilityToThreshold(prob)
}

func RandomnessFromTraceID(tid pcommon.TraceID) Randomness {
	return unsigned.RandomnessFromTraceID(tid)
}

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
