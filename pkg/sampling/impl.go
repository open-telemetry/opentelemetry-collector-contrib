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

var AlwaysSampleThreshold = unsigned.AlwaysSampleThreshold
var NeverSampleThreshold = unsigned.NeverSampleThreshold
var ErrTValueSize = unsigned.ErrTValueSize
var ErrRValueSize = unsigned.ErrRValueSize
