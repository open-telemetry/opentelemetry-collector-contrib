// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package precision // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/precision"

import "math"

var (
	// ScaleMilliseconds converts millisecond counters to seconds.
	ScaleMilliseconds = scaleUint64(1_000)
	// ScaleNanoseconds converts nanosecond counters to seconds.
	ScaleNanoseconds = scaleUint64(1_000_000_000)
	// Scale100Nanoseconds converts 100-nanosecond counters (Windows perf counters) to seconds.
	Scale100Nanoseconds = scaleUint64(10_000_000)
)

// RatioUint64 computes numerator/denominator and rounds the result to the
// number of significant digits supported by the inputs. The significant digit
// count is derived from the magnitude of the larger operand, matching the
// information content of the integer inputs. When denominator is zero the
// native Go float64 division result is returned (NaN for 0/0, +Inf otherwise).
func RatioUint64(numerator, denominator uint64) float64 {
	if denominator == 0 {
		return float64(numerator) / float64(denominator)
	}
	return roundRatio(float64(numerator), float64(denominator))
}

func scaleUint64(denominator uint64) func(uint64) float64 {
	mul := float64(denominator)
	return func(numerator uint64) float64 {
		res := float64(numerator) / mul
		return math.Round(res*mul) / mul
	}
}

func roundRatio(numerator, denominator float64) float64 {
	ratio := numerator / denominator
	sigDigits := int(math.Floor(math.Log10(math.Max(numerator, denominator)))) + 1
	if sigDigits < 1 {
		sigDigits = 1
	}
	mul := math.Pow(10, float64(sigDigits))
	return math.Round(ratio*mul) / mul
}
