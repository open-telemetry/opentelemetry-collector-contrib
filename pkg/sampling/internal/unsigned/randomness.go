package unsigned

import (
	"encoding/binary"
	"errors"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// ErrRValueSize is returned for r-values != NumHexDigits hex digits.
var ErrRValueSize = errors.New("r-value must have 14 hex digits")

// LeastHalfTraceIDThresholdMask is the mask to use on the
// least-significant half of the TraceID, i.e., bytes 8-15.
// Because this is a 56 bit mask, the result after masking is
// the unsigned value of bytes 9 through 15.
const LeastHalfTraceIDThresholdMask = MaxAdjustedCount - 1

// Randomness may be derived from R-value or TraceID.
type Randomness struct {
	// unsigned is in the range [0, MaxAdjustedCount-1]
	unsigned uint64
}

// RandomnessFromTraceID returns randomness from a TraceID (assumes
// the traceparent random flag was set).
func RandomnessFromTraceID(id pcommon.TraceID) Randomness {
	return Randomness{
		unsigned: binary.BigEndian.Uint64(id[8:]) & LeastHalfTraceIDThresholdMask,
	}
}

// RandomnessFromBits returns randomness from 56 random bits.
func RandomnessFromBits(bits uint64) Randomness {
	return Randomness{
		unsigned: bits & LeastHalfTraceIDThresholdMask,
	}
}

// RValueToRandomness parses NumHexDigits hex bytes into a Randomness.
func RValueToRandomness(s string) (Randomness, error) {
	if len(s) != NumHexDigits {
		return Randomness{}, ErrRValueSize
	}

	unsigned, err := strconv.ParseUint(s, hexBase, 64)
	if err != nil {
		return Randomness{}, err
	}

	return Randomness{
		unsigned: unsigned,
	}, nil
}

func (rnd Randomness) ToRValue() string {
	// Note: adding MaxAdjustedCount then removing the leading byte accomplishes
	// zero padding.
	return strconv.FormatUint(MaxAdjustedCount+rnd.unsigned, hexBase)[1:]

}
