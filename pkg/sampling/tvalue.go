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

	// ProbabilityZeroEncoding is the encoding for 0 adjusted count.
	ProbabilityZeroEncoding = "0"

	// ProbabilityOneEncoding is the encoding for 100% sampling.
	ProbabilityOneEncoding = "1"
)

// Threshold used to compare with the least-significant 7 bytes of the TraceID.
type Threshold struct {
	// unsigned is in the range [0, MaxAdjustedCount]
	// - 0 represents zero probability (0 TraceID values are less-than)
	// - 1 represents MinSamplingProb (1 TraceID value is less-than)
	// - MaxAdjustedCount represents 100% sampling (all TraceID values are less-than).
	unsigned uint64
}

// Randomness may be derived from r-value or TraceID.
type Randomness struct {
	// randomness is in the range [0, MaxAdjustedCount-1]
	unsigned uint64
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

// probabilityInRange tests MinSamplingProb <= prob <= 1.
func probabilityInRange(prob float64) bool {
	return prob >= MinSamplingProb && prob <= 1
}

// AdjustedCountToEncoded encodes a s-value or t-value given an
// adjusted count. In this form, the encoding is a decimal integer.
func AdjustedCountToEncoded(count uint64) (string, error) {
	switch {
	case count == 0:
		return ProbabilityZeroEncoding, nil
	case count < 0:
		return "", ErrProbabilityRange
	case count > uint64(MaxAdjustedCount):
		return "", ErrAdjustedCountRange
	}
	return strconv.FormatInt(int64(count), 10), nil
}

// ProbabilityToEncoded encodes a s-value or t-value given a
// probability.  In this form, the user controls floating-point format
// and precision.  See strconv.FormatFloat() for an explanation of
// `format` and `prec`.
func ProbabilityToEncoded(prob float64, format byte, prec int) (string, error) {
	// Probability cases
	switch {
	case prob == 1:
		return ProbabilityOneEncoding, nil
	case prob == 0:
		return ProbabilityZeroEncoding, nil
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

// EncodedToProbabilityAndAdjustedCount parses the t-value and returns
// both the probability and the adjusted count.  In a Span-to-Metrics
// pipeline, users should count either the inverse of probability or
// the adjusted count.  When the arriving t-value encodes adjusted
// count as opposed to probability, the adjusted count will be exactly
// the specified integer value; in these cases, probability corresponds
// with exactly implemented sampling ratio.
func EncodedToProbabilityAndAdjustedCount(s string) (float64, float64, error) {
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

// ProbabilityToThreshold returns the sampling threshold exactly
// corresponding with the input probability.
func ProbabilityToThreshold(prob float64) (Threshold, error) {
	// Note: prob == 0 is an allowed special case.  Because we
	// use less-than, all spans are unsampled with Threshold{0}.
	if prob != 0 && !probabilityInRange(prob) {
		return Threshold{}, ErrProbabilityRange
	}
	return Threshold{
		unsigned: uint64(prob * MaxAdjustedCount),
	}, nil
}

// ShouldSample returns true when the span passes this sampler's
// consistent sampling decision.
func (t Threshold) ShouldSample(rnd Randomness) bool {
	return rnd.unsigned < t.unsigned
}

func RandomnessFromTraceID(id pcommon.TraceID) Randomness {
	return Randomness{
		unsigned: binary.BigEndian.Uint64(id[8:]) & LeastHalfTraceIDThresholdMask,
	}
}

// Probability is the sampling ratio in the range [MinSamplingProb, 1].
func (t Threshold) Probability() float64 {
	return float64(t.unsigned) / MaxAdjustedCount
}

// Unsigned is an unsigned integer that scales with the sampling
// threshold.  This is useful to compare two thresholds without
// floating point conversions.
func (t Threshold) Unsigned() uint64 {
	return t.unsigned
}
