// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

const (
	// DefaultSamplingPrecision is the number of hexadecimal
	// digits of precision used to expressed the samplling probability.
	defaultSamplingPrecision = 4

	// MinSupportedProbability is the smallest probability that
	// can be encoded by this implementation, and it defines the
	// smallest interval between probabilities across the range.
	// The largest supported probability is (1-MinSupportedProbability).
	//
	// This value corresponds with the size of a float64
	// significand, because it simplifies this implementation to
	// restrict the probability to use 52 bits (vs 56 bits).
	minSupportedProbability float64 = 1 / float64(maxAdjustedCount)

	// maxSupportedProbability is the number closest to 1.0 (i.e.,
	// near 99.999999%) that is not equal to 1.0 in terms of the
	// float64 representation, having 52 bits of significand.
	// Other ways to express this number:
	//
	//   0x1.ffffffffffffe0p-01
	//   0x0.fffffffffffff0p+00
	//   math.Nextafter(1.0, 0.0)
	maxSupportedProbability float64 = 1 - 0x1p-52

	// maxAdjustedCount is the inverse of the smallest
	// representable sampling probability, it is the number of
	// distinct 56 bit values.
	maxAdjustedCount uint64 = 1 << 56

	// randomnessMask is a mask that selects the least-significant
	// 56 bits of a uint64.
	randomnessMask uint64 = maxAdjustedCount - 1

	// NEVER_SAMPLE_THRESHOLD indicates a span that should not be sampled.
	// This is equivalent to sampling with 0% probability.
	NEVER_SAMPLE_THRESHOLD int64 = 1 << 56

	// ALWAYS_SAMPLE_THREHSOLD indicates to sample with 100% probability.
	ALWAYS_SAMPLE_THRESHOLD int64 = 0

	// INVALID_THREHSOLD indicates a span that should be sampled with
	// unknown probability.
	INVALID_THRESHOLD int64 = -1
)
