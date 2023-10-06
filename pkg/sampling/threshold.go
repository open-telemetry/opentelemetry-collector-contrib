// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"errors"
	"strconv"
	"strings"
)

const (
	// MaxAdjustedCount is 2^56 i.e. 0x100000000000000 i.e., 1<<56.
	MaxAdjustedCount = 1 << 56

	// NumHexDigits is the number of hex digits equalling 56 bits.
	NumHexDigits = 56 / hexBits

	hexBits = 4
	hexBase = 16

	NeverSampleTValue = "0"
)

// Threshold used to compare with the least-significant 7 bytes of the TraceID.
type Threshold struct {
	// unsigned is in the range [0, MaxAdjustedCount]
	// - 0 represents never sampling (0 TraceID values are less-than)
	// - 1 represents 1-in-MaxAdjustedCount (1 TraceID value is less-than)
	// - MaxAdjustedCount represents always sampling (all TraceID values are less-than).
	unsigned uint64
}

var (
	// ErrTValueSize is returned for t-values longer than NumHexDigits hex digits.
	ErrTValueSize = errors.New("t-value exceeds 14 hex digits")

	NeverSampleThreshold  = Threshold{unsigned: 0}
	AlwaysSampleThreshold = Threshold{unsigned: MaxAdjustedCount}
)

// TValueToThreshold returns a Threshold.  Because TValue strings
// have trailing zeros omitted, this function performs the reverse.
func TValueToThreshold(s string) (Threshold, error) {
	if len(s) > NumHexDigits {
		return AlwaysSampleThreshold, ErrTValueSize
	}
	if len(s) == 0 {
		return AlwaysSampleThreshold, nil
	}

	// Having checked length above, there are no range errors
	// possible.  Parse the hex string to an unsigned valued.
	unsigned, err := strconv.ParseUint(s, hexBase, 64)
	if err != nil {
		return AlwaysSampleThreshold, err // e.g. parse error
	}

	// The unsigned value requires shifting to account for the
	// trailing zeros that were omitted by the encoding (see
	// TValue for the reverse).  Compute the number to shift by:
	extendByHexZeros := NumHexDigits - len(s)
	return Threshold{
		unsigned: unsigned << (hexBits * extendByHexZeros),
	}, nil
}

// TValue encodes a threshold, which is a variable-length hex string
// up to 14 characters.  The empty string is returned for 100%
// sampling.
func (th Threshold) TValue() string {
	// Special cases
	switch th.unsigned {
	case MaxAdjustedCount:
		// 100% sampling.  Samplers are specified not to
		// include a TValue in this case.
		return ""
	case 0:
		// 0% sampling.  This is a special case, otherwise,
		// the TrimRight below will return an empty string
		// matching the case above.
		return "0"
	}

	// For thresholds other than the extremes, format a full-width
	// (14 digit) unsigned value with leading zeros, then, remove
	// the trailing zeros.  Use the logic for (Randomness).RValue().
	digits := Randomness(th).RValue()

	// Remove trailing zeros.
	return strings.TrimRight(digits, "0")
}

// ShouldSample returns true when the span passes this sampler's
// consistent sampling decision.
func (t Threshold) ShouldSample(rnd Randomness) bool {
	return rnd.unsigned < t.unsigned
}

// ThresholdLessThan allows direct comparison of Threshold values.
// Smaller thresholds equate with smaller probabilities.
func ThresholdLessThan(a, b Threshold) bool {
	return a.unsigned < b.unsigned
}
