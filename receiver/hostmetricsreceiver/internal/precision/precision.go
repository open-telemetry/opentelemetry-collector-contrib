// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package precision // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/precision"

import "math"

// Common scale factors for use with ScaleUint64.
const (
	// MillisecondsPerSecond is used to convert millisecond counters (e.g.
	// disk I/O times from gopsutil on Linux) to seconds.
	MillisecondsPerSecond uint64 = 1_000

	// NanosecondsPerSecond is used to convert nanosecond timestamps (e.g.
	// pcommon.Timestamp differences) to seconds.
	NanosecondsPerSecond uint64 = 1_000_000_000

	// WindowsPerfCounterUnitsPerSecond is the scale factor for Windows
	// performance counter time values (100-nanosecond intervals) to seconds.
	WindowsPerfCounterUnitsPerSecond uint64 = 10_000_000
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

// ScaleUint64 divides numerator by a known constant denominator (e.g.
// 1000 for milliseconds-to-seconds) and rounds to the denominator's decimal
// precision. This ensures clean results even when 1/denominator is not exactly
// representable in binary floating-point (e.g. 1/100). When denominator is
// zero the native Go float64 division result is returned.
func ScaleUint64(numerator, denominator uint64) float64 {
	if denominator == 0 {
		return float64(numerator) / float64(denominator)
	}
	res := float64(numerator) / float64(denominator)
	mul := float64(denominator)
	return math.Round(res*mul) / mul
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
