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

package bytes

import (
	"bytes"
	"encoding/hex"
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
	bytes [8]byte
}

var (
	// ErrTValueSize is returned for t-values longer than NumHexDigits hex digits.
	ErrTValueSize = errors.New("t-value exceeds 14 hex digits")

	NeverSampleThreshold  = Threshold{bytes: [8]byte{0, 0, 0, 0, 0, 0, 0, 0}}
	AlwaysSampleThreshold = Threshold{bytes: [8]byte{1, 0, 0, 0, 0, 0, 0, 0}}

	hex14Zeros = func() (r [NumHexDigits]byte) {
		for i := range r {
			r[i] = '0'
		}
		return
	}()
)

// TValueToThreshold returns a Threshold, see Threshold.ShouldSample(TraceID).
func TValueToThreshold(s string) (Threshold, error) {
	if len(s) > NumHexDigits {
		return AlwaysSampleThreshold, ErrTValueSize
	}
	if len(s) == 0 {
		return AlwaysSampleThreshold, nil
	}

	// Fill with padding, then copy most-significant hex digits.
	hexPadded := hex14Zeros
	copy(hexPadded[0:len(s)], s)

	var th Threshold
	if _, err := hex.Decode(th.bytes[1:], hexPadded[:]); err != nil {
		return AlwaysSampleThreshold, strconv.ErrSyntax // ErrSyntax for consistency w/ ../unsigned
	}
	return th, nil
}

func (th Threshold) TValue() string {
	// Special cases
	switch {
	case th == AlwaysSampleThreshold:
		return ""
	case th == NeverSampleThreshold:
		return "0"
	}

	var hexDigits [14]byte
	_ = hex.Encode(hexDigits[:], th.bytes[1:])
	return strings.TrimRight(string(hexDigits[:]), "0")
}

// ShouldSample returns true when the span passes this sampler's
// consistent sampling decision.
func (t Threshold) ShouldSample(rnd Randomness) bool {
	if t == AlwaysSampleThreshold {
		// 100% sampling case
		return true
	}
	return bytes.Compare(rnd.bytes[1:], t.bytes[1:]) < 0
}

func ThresholdLessThan(a, b Threshold) bool {
	// Note full 8 byte compare
	return bytes.Compare(a.bytes[:], b.bytes[:]) < 0
}
