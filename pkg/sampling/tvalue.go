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

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	// MinSamplingProb is one in 2^56.
	MinSamplingProb = 0x1p-56

	// MaxAdjustedCount is the adjusted count corresponding with
	// MinSamplingProb (i.e., 1 / MinSamplingProb).  0x1p+56
	MaxAdjustedCount = 1 / MinSamplingProb

	// LeastHalfTraceIDThresholdMask is the mask to use on the
	// least-significant half of the TraceID, i.e., bytes 8-15.
	// Because this is a 56 bit mask, the result after masking is
	// the unsigned value of bytes 9 through 15.
	LeastHalfTraceIDThresholdMask = MaxAdjustedCount - 1

	// TValueZeroEncoding is the encoding for 0 adjusted count.
	TValueZeroEncoding = "t:0"
	TValueOneEncoding  = "t:1"
)

// Threshold is an opaque type used to compare with the least-significant 7 bytes of the TraceID.
type Threshold struct {
	// limit is in the range [0, 0x1p+56].
	// - 0 represents zero probability (no TraceID values are less-than)
	// - 1 represents MinSamplingProb (1 TraceID value is less-than)
	// - MaxAdjustedCount represents 100% sampling (all TraceID values are less-than).
	limit uint64
}

var (
	// ErrProbabilityRange is returned when a value should be in the range [MinSamplingProb, 1].
	ErrProbabilityRange = fmt.Errorf("sampling probability out of range (0x1p-56 <= valid <= 1)")

	// ErrAdjustedCountRange is returned when a value should be in the range [1, MaxAdjustedCount].
	ErrAdjustedCountRange = fmt.Errorf("sampling adjusted count out of range (1 <= valid <= 0x1p+56)")

	// ErrAdjustedCountOnlyInteger is returned when a floating-point syntax is used to convey adjusted count.
	ErrAdjustedCountOnlyInteger = fmt.Errorf("sampling adjusted count must be an integer")

	// ErrPrecisionRange is returned when the precision argument is out of range.
	ErrPrecisionRange = fmt.Errorf("sampling precision out of range (-1 <= valid <= 14)")
)

func probabilityInRange(prob float64) bool {
	return prob <= 1 && prob >= MinSamplingProb
}

func AdjustedCountToTvalue(count uint64) (string, error) {
	switch {
	case count == 0:
		// Special case.
	case count < 0:
		return "", ErrProbabilityRange
	case count > uint64(MaxAdjustedCount):
		return "", ErrAdjustedCountRange
	}
	return strconv.FormatInt(int64(count), 10), nil
}

// E.g., 3/7 w/ prec=2 -> "0x1.b7p-02"
func ProbabilityToTvalue(prob float64, format byte, prec int) (string, error) {
	// Probability cases
	switch {
	case prob == 1:
		return TValueOneEncoding, nil
	case prob == 0:
		return TValueZeroEncoding, nil
	case !probabilityInRange(prob):
		return "", ErrProbabilityRange
	}
	// Precision cases
	switch {
	case prec == -1:
		// Default precision (see FormatFloat)
	case prec == 0:
		// Precision == 0 forces probabilities to be powers-of-two.
	case prec <= 14:
		// Precision is in-range
	default:
		return "", ErrPrecisionRange

	}
	return "t:" + strconv.FormatFloat(prob, format, prec, 64), nil
}

func TvalueToProbabilityAndAdjustedCount(s string) (float64, float64, error) {
	if !strings.HasPrefix(s, "t:") {
		return 0, 0, strconv.ErrSyntax
	}
	number, err := strconv.ParseFloat(s[2:], 64) // e.g., "0x1.b7p-02" -> approx 3/7
	if err != nil {
		return 0, 0, err
	}

	adjusted := 0.0
	switch {
	case number == 0:

	case number < MinSamplingProb:
		return 0, 0, ErrProbabilityRange
	case number > 1:
		// Greater than 1 indicates adjusted count; re-parse
		// as a decimal integer.
		integer, err := strconv.ParseInt(s[2:], 10, 64)
		if err != nil {
			return 0, 0, ErrAdjustedCountOnlyInteger
		}
		if integer > MaxAdjustedCount {
			return 0, 0, ErrAdjustedCountRange
		}
		adjusted = float64(integer)
		number = 1 / adjusted
	default:
		adjusted = 1 / number
	}

	return number, adjusted, nil
}

func ProbabilityToThreshold(prob float64) (Threshold, error) {
	// Note: prob == 0 is an allowed special case.  Because we
	// use less-than, all spans are unsampled with Threshold{0}.
	if prob != 0 && !probabilityInRange(prob) {
		return Threshold{}, ErrProbabilityRange
	}
	return Threshold{
		limit: uint64(prob * MaxAdjustedCount),
	}, nil
}

func (t Threshold) ShouldSample(id pcommon.TraceID) bool {
	value := binary.BigEndian.Uint64(id[8:]) & LeastHalfTraceIDThresholdMask
	return value < t.limit
}

func (t Threshold) Probability() float64 {
	return float64(t.limit) / MaxAdjustedCount
}

func (t Threshold) Unsigned() uint64 {
	return t.limit
}
