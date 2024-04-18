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

// Bounds returns the half-open interval (min,max] of the bucket at index.
// This means a value min<v<=max belongs to this bucket.
// NOTE: this is different from Go slice intervals, which are [a,b)
func (scale Scale) Bounds(index int) (min, max float64) {
	lower := func(index int) float64 {
		inverseFactor := math.Ldexp(math.Ln2, int(-scale))
		return math.Exp(float64(index) * inverseFactor)
	}

	return lower(index), lower(index + 1)
}
