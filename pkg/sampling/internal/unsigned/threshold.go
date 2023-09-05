// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package unsigned

import (
	"errors"
	"strconv"
	"strings"
)

const (
	// MaxAdjustedCount is 2^56 i.e. 0x100000000000000 i.e., 1<<56.
	MaxAdjustedCount = 1 << 56

	// NumHexDigits is the number of hex digits equalling 56 bits.
	NumHexDigits = 56 / 4

	hexBase = 16
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

// TValueToThreshold returns a Threshold, see Threshold.ShouldSample(TraceID).
func TValueToThreshold(s string) (Threshold, error) {
	if len(s) > NumHexDigits {
		return AlwaysSampleThreshold, ErrTValueSize
	}
	if len(s) == 0 {
		return AlwaysSampleThreshold, nil
	}

	// Note that this handles zero correctly, but the inverse
	// operation does not.  I.e., "0" parses as unsigned == 0.
	unsigned, err := strconv.ParseUint(s, hexBase, 64)
	if err != nil {
		return AlwaysSampleThreshold, err
	}

	// Zero-padding is done by shifting 4 bits per absent hex digit.
	extend := NumHexDigits - len(s)
	return Threshold{
		unsigned: unsigned << (4 * extend),
	}, nil
}

func (th Threshold) TValue() string {
	// Special cases
	switch th.unsigned {
	case MaxAdjustedCount:
		// 100% sampling
		return ""
	case 0:
		// 0% sampling.  This is a special case, otherwise, the TrimRight
		// below will return an empty matching the case above.
		return "0"
	}
	// Add MaxAdjustedCount yields 15 hex digits with a leading "1".
	allBits := MaxAdjustedCount + th.unsigned
	// Then format and remove the most-significant hex digit.
	digits := strconv.FormatUint(allBits, hexBase)[1:]
	// Leaving NumHexDigits hex digits, with trailing zeros removed.
	return strings.TrimRight(digits, "0")
}

// ShouldSample returns true when the span passes this sampler's
// consistent sampling decision.
func (t Threshold) ShouldSample(rnd Randomness) bool {
	return rnd.unsigned < t.unsigned
}

func ThresholdLessThan(a, b Threshold) bool {
	return a.unsigned < b.unsigned
}
