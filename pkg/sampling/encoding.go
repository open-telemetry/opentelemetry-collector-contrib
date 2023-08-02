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
	"errors"
	"fmt"
	"math"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	// MinSamplingProb is one in 2^56.
	MinSamplingProb = 0x1p-56

	// LeastHalfTraceIDThresholdMask is the mask to use on the
	// least-significant half of the TraceID, i.e., bytes 8-15.
	// Because this is a 56 bit mask, the result after masking is
	// the unsigned value of bytes 9 through 15.
	LeastHalfTraceIDThresholdMask = 1/MinSamplingProb - 1

	// ProbabilityZeroEncoding is the encoding for 0 adjusted count.
	ProbabilityZeroEncoding = "0"

	// ProbabilityOneEncoding is the encoding for 100% sampling.
	ProbabilityOneEncoding = ""
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
	ErrProbabilityRange = errors.New("sampling probability out of range (0x1p-56 <= valid <= 1)")

	// ErrTValueSize is returned for t-values longer than 14 hex digits.
	ErrTValueSize = errors.New("t-value exceeds 14 hex digits")

	// ErrRValueSize is returned for r-values != 14 hex digits.
	ErrRValueSize = errors.New("r-value must have 14 hex digits")
)

// probabilityInRange tests MinSamplingProb <= prob <= 1.
func probabilityInRange(prob float64) bool {
	return prob >= MinSamplingProb && prob <= 1
}

// removeTrailingZeros elimiantes trailing zeros from a string.
func removeTrailingZeros(in string) string {
	for len(in) > 1 && in[len(in)-1] == '0' {
		in = in[:len(in)-1]
	}
	return in
}

// ProbabilityToTValue encodes a t-value given a probability.
func ProbabilityToTValue(prob float64) (string, error) {
	// Probability cases
	switch {
	case prob == 1:
		return ProbabilityOneEncoding, nil
	case prob == 0:
		return ProbabilityZeroEncoding, nil
	case !probabilityInRange(prob):
		return "", ErrProbabilityRange
	}
	unsigned := uint64(math.Round(prob / MinSamplingProb))

	// Note fmt.Sprintf handles zero padding to 14 bytes as well as setting base=16.
	// Otherwise could be done by hand using strconv.FormatUint(unsigned, 16) and
	// and padding to 14 bytes before removing the trailing zeros.
	return removeTrailingZeros(fmt.Sprintf("%014x", unsigned)), nil
}

// TValueToProbability parses the t-value and returns
// the probability.
func TValueToProbability(s string) (float64, error) {
	if len(s) > 14 {
		return 0, ErrTValueSize
	}
	if s == ProbabilityOneEncoding {
		return 1, nil
	}

	unsigned, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0, err
	}

	// Zero-padding is done by shifting 4 bit positions per
	// missing hex digit.
	extend := 14 - len(s)
	return float64(unsigned<<(4*extend)) * MinSamplingProb, nil
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
		unsigned: uint64(prob / MinSamplingProb),
	}, nil
}

// TValueToThreshold return a Threshold, see
// Threshold.ShouldSample(TraceID) and Threshold.Probability().
func TValueToThreshold(s string) (Threshold, error) {
	prob, err := TValueToProbability(s)
	if err != nil {
		return Threshold{}, err
	}
	return ProbabilityToThreshold(prob)
}

// ShouldSample returns true when the span passes this sampler's
// consistent sampling decision.
func (t Threshold) ShouldSample(rnd Randomness) bool {
	return rnd.unsigned < t.unsigned
}

// Probability is the sampling ratio in the range [MinSamplingProb, 1].
func (t Threshold) Probability() float64 {
	return float64(t.unsigned) * MinSamplingProb
}

// Unsigned is an unsigned integer that scales with the sampling
// threshold.  This is useful to compare two thresholds without
// floating point conversions.
func (t Threshold) Unsigned() uint64 {
	return t.unsigned
}

// Randomness is the value we compare with Threshold in ShouldSample.
func RandomnessFromTraceID(id pcommon.TraceID) Randomness {
	return Randomness{
		unsigned: binary.BigEndian.Uint64(id[8:]) & LeastHalfTraceIDThresholdMask,
	}
}

// Unsigned is an unsigned integer that scales with the randomness
// value.  This is useful to compare two randomness values without
// floating point conversions.
func (r Randomness) Unsigned() uint64 {
	return r.unsigned
}

// RValueToRandomness parses 14 hex bytes into a Randomness.
func RValueToRandomness(s string) (Randomness, error) {
	if len(s) != 14 {
		return Randomness{}, ErrRValueSize
	}

	unsigned, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return Randomness{}, err
	}

	return Randomness{
		unsigned: unsigned,
	}, nil
}
