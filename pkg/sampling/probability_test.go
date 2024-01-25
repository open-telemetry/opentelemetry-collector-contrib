// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// ExampleProbabilityToThresholdWithPrecision demonstrates how 1/3,
// 2/3, and 3/3 are encoded with precision 3.
func ExampleProbabilityToThresholdWithPrecision() {
	const divisor = 3.0
	const precision = 3

	for dividend := 1.0; dividend <= divisor; dividend++ {
		tval, _ := ProbabilityToThresholdWithPrecision(dividend/divisor, precision)
		fmt.Println(tval.TValue())
	}

	// Output:
	// aab
	// 555
	// 0
}

// ExampleProbabilityToThreshold_rounding demonstrates that with full
// precision, the resulting t-value appears to round in an unexpected
// way.
func ExampleProbabilityToThreshold_rounding() {
	// 1/3 sampling corresponds with a rejection threshold of (1 - 1/3).
	const exampleProb = 1.0 / 3.0

	// 1/3 in decimal is the repeating fraction of 6 (0.333333), while in
	// hexadecimal it is the repeating fraction of a (0x0.555555).
	tval, _ := ProbabilityToThreshold(exampleProb)

	// Note the trailing hex "c" below, which does not match
	// intuition for a repeating pattern of hex "a" digits.  Why
	// is the final digit not hex "b"?  The reason it is hex "c"
	// is that ProbabilityToThreshold computes the number of spans
	// selected as a 56-bit integer using a 52-bit significand.
	// Because the fraction uses fewer bits than the threshold,
	// the last digit rounds down, with 0x55555555555554 spans
	// rejected out of 0x100000000000000.  The subtraction of 0x4
	// from 0x10 leads to a trailing hex "c".
	fmt.Println(tval.TValue())

	// Output:
	// aaaaaaaaaaaaac
}

func ExampleProbabilityToThreshold_verysmall() {
	for _, prob := range []float64{
		MinSamplingProbability, // Skip 1 out of 2**56
		0x2p-56,                // Skip 2 out of 2**56
		0x3p-56,                // Skip 3 out of 2**56
		0x4p-56,                // Skip 4 out of 2**56
		0x8p-56,                // Skip 8 out of 2**56
		0x10p-56,               // Skip 16 out of 2**56
	} {
		tval, _ := ProbabilityToThreshold(prob)
		fmt.Println(tval.TValue())
	}

	// Output:
	// ffffffffffffff
	// fffffffffffffe
	// fffffffffffffd
	// fffffffffffffc
	// fffffffffffff8
	// fffffffffffff
}

func TestProbabilityToThresholdWithPrecision(t *testing.T) {
	type kase struct {
		prob    float64
		exact   string
		rounded []string
	}

	for _, test := range []kase{
		// Note: remember 8 is half of 16: hex rounds up at 8+, down at 7-.
		{
			1 - 0x456789ap-28,
			"456789a",
			[]string{
				"45678a",
				"45679",
				"4568",
				"456",
				"45",
				"4",
			},
		},
		// Add 3 leading zeros
		{
			1 - 0x456789ap-40,
			"000456789a",
			[]string{
				"00045678a",
				"00045679",
				"0004568",
				"000456",
				"00045",
				"0004",
			},
		},
		// Rounding up
		{
			1 - 0x789abcdefp-40,
			"0789abcdef",
			[]string{
				"0789abcdef",
				"0789abcdf",
				"0789abce",
				"0789abd",
				"0789ac",
				"0789b",
				"078a",
				"079",
				"08",
			},
		},
		// Rounding down
		{
			1 - 0x12345678p-32,
			"12345678",
			[]string{
				"1234568",
				"123456",
				"12345",
				"1234",
				"123",
				"12",
				"1",
			},
		},
		// Zeros
		{
			1 - 0x80801p-28,
			"0080801",
			[]string{
				"00808",
				"008",
			},
		},
		// 100% sampling
		{
			1,
			"0",
			[]string{
				"0",
			},
		},
	} {
		t.Run(test.exact, func(t *testing.T) {
			th, err := ProbabilityToThreshold(test.prob)
			require.NoError(t, err)
			require.Equal(t, th.TValue(), test.exact)

			for _, round := range test.rounded {
				t.Run(round, func(t *testing.T) {
					// Requested precision is independent of leading zeros,
					// so strip them to calculate test precision.
					strip := round
					for len(strip) > 0 && strip[0] == '0' {
						strip = strip[1:]
					}
					rth, err := ProbabilityToThresholdWithPrecision(test.prob, uint8(len(strip)))
					require.NoError(t, err)
					require.Equal(t, round, rth.TValue())
				})
			}
		})
	}
}
