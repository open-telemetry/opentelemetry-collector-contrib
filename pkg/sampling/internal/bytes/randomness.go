package bytes

import (
	"encoding/hex"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// ErrRValueSize is returned for r-values != NumHexDigits hex digits.
var ErrRValueSize = errors.New("r-value must have 14 hex digits")

// Randomness may be derived from r-value or TraceID.
type Randomness struct {
	// bytes[0] is unused, so that the relevant portion of these 8
	// bytes align with the TraceID's second 8 bytes.
	bytes [8]byte
}

// Randomness is the value we compare with Threshold in ShouldSample.
func RandomnessFromTraceID(id pcommon.TraceID) Randomness {
	var r Randomness
	copy(r.bytes[1:], id[9:])
	return r
}

// RValueToRandomness parses NumHexDigits hex bytes into a Randomness.
func RValueToRandomness(s string) (Randomness, error) {
	if len(s) != NumHexDigits {
		return Randomness{}, ErrRValueSize
	}

	var r Randomness
	_, err := hex.Decode(r.bytes[1:], []byte(s))
	return r, err
}
