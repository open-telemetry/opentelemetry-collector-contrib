// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import "fmt"

func ExampleProbabilityToThresholdWithPrecision() {
	// 2/3 sampling corresponds with a rejection threshold of 1/3.
	const exampleProb = 2.0 / 3.0
	const precision = 3

	// 1/3 in decimal is the repeating fraction of 3s (0.333333), while in
	// hexadecimal it is the repeating fraction of 5s (0x0.555555p0)
	tval, _ := ProbabilityToThresholdWithPrecision(2./3., precision)
	fmt.Println(tval.TValue())

	// Output: 555
}

func ExampleProbabilityToThreshold() {
	// 1/3 sampling corresponds with a rejection threshold of (1 - 1/3).
	const exampleProb = 1.0 / 3.0

	// 1/3 in decimal is the repeating fraction of 6 (0.333333), while in
	// hexadecimal it is the repeating fraction of a (0x0.555555).
	tval, _ := ProbabilityToThreshold(exampleProb)

	// Note the trailing c below, which seems out of place-- for a
	// repeating fraction of hex as it makes intuitive sense to
	// have the final digit be b, right?  The reason it is "c" is
	// that ProbabilityToThreshold computes the number of spans
	// selected as a 56-bit integer using a 52-bit significand.
	// Because the fraction uses fewer bits than the threshold,
	// the last digit rounds down, with 0x55555555555554 spans
	// rejected out of 0x100000000000000.  The subtraction of
	// 0x4 from 0x10 leads to a trailing "c".
	fmt.Println(tval.TValue())

	// Output: aaaaaaaaaaaaac
}
