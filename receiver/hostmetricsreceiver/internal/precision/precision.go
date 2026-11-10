// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package precision // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/precision"

import (
	"math"
	"time"
)

// Ratio computes numerator/denominator and rounds the result to the
// number of significant digits supported by the inputs. The significant digit
// count is derived from the magnitude of the larger operand, matching the
// information content of the integer inputs. When denominator is zero the
// native Go float64 division result is returned (NaN for 0/0, +Inf otherwise).
func Ratio(numerator, denominator uint64) float64 {
	if denominator == 0 {
		return float64(numerator) / float64(denominator)
	}
	return roundRatio(float64(numerator), float64(denominator))
}

// Scale converts a tick count in the given unit to seconds and rounds
// to the unit's decimal precision. This avoids binary float artifacts
// like 12345/1000 = 12.345000000000001.
func Scale(numerator uint64, unit time.Duration) float64 {
	mul := float64(time.Second / unit)
	res := float64(numerator) / mul
	return math.Round(res*mul) / mul
}

func roundRatio(numerator, denominator float64) float64 {
	ratio := numerator / denominator
	sigDigits := int(math.Floor(math.Log10(math.Max(numerator, denominator)))) + 1
	sigDigits = max(sigDigits, 1)
	mul := math.Pow(10, float64(sigDigits))
	return math.Round(ratio*mul) / mul
}
