package exphistogram

import "math"

// LowerBoundary calculates the lower boundary given index and scale.
// Adopted from https://opentelemetry.io/docs/specs/otel/metrics/data-model/#producer-expectations
func LowerBoundary(index, scale int) float64 {
	// Use this form in case the equation above computes +Inf
	// as the lower boundary of a valid bucket.
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return 2.0 * math.Exp(float64(index-(1<<scale))*inverseFactor)
}

// MapToIndex gets bucket index from value and scale.
// Adopted from https://opentelemetry.io/docs/specs/otel/metrics/data-model/#producer-expectations
func MapToIndex(value float64, scale int) int {
	// Special case for power-of-two values.
	if frac, exp := math.Frexp(value); frac == 0.5 {
		return ((exp - 1) << scale) - 1
	}
	scaleFactor := math.Ldexp(math.Log2E, scale)
	// Note: math.Floor(value) equals math.Ceil(value)-1 when value
	// is not a power of two, which is checked above.
	return int(math.Floor(math.Log(value) * scaleFactor))
}
