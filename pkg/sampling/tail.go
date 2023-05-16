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

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Threshold is an opaque type used to compare with the least-significant 7 bytes of the TraceID.
type Threshold struct {
	// limit in range [1, 0x1p+56]
	limit uint64
}

const (
	MinSamplingProb  = 0x1p-56
	MaxAdjustedCount = 0x1p+56 // i.e., 1 / MinSamplingProb

	LeastHalfTraceIDThresholdMask = MaxAdjustedCount - 1
)

var (
	ErrPrecisionRange           = fmt.Errorf("sampling precision out of range (-1 <= valid <= 14)")
	ErrProbabilityRange         = fmt.Errorf("sampling probability out of range (0x1p-56 <= valid <= 1)")
	ErrAdjustedCountRange       = fmt.Errorf("sampling adjusted count out of range (1 <= valid <= 0x1p+56)")
	ErrAdjustedCountOnlyInteger = fmt.Errorf("sampling adjusted count must be an integer")
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
		return "1", nil
	case prob == 0:
		return "0", nil
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
	return strconv.FormatFloat(prob, format, prec, 64), nil
}

func TvalueToProbabilityAndAdjustedCount(s string) (float64, float64, error) {
	number, err := strconv.ParseFloat(s, 64) // e.g., "0x1.b7p-02" -> approx 3/7
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
		integer, err := strconv.ParseInt(s, 10, 64)
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
	if !probabilityInRange(prob) {
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
