package expo

import "math"

type Scale int32

// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#all-scales-use-the-logarithm-function
func (scale Scale) Idx(v float64) int {
	// Special case for power-of-two values.
	if frac, exp := math.Frexp(v); frac == 0.5 {
		return ((exp - 1) << scale) - 1
	}

	scaleFactor := math.Ldexp(math.Log2E, int(scale))
	// Note: math.Floor(value) equals math.Ceil(value)-1 when value
	// is not a power of two, which is checked above.
	return int(math.Floor(math.Log(v) * scaleFactor))
}
