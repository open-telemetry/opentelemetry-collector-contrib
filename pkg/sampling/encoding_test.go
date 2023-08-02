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

package sampling

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func mustNot[T any](t T, err error) error {
	if err == nil {
		return fmt.Errorf("expected an error, got nil")
	}
	return err
}

func TestValidAdjustedCountToTvalue(t *testing.T) {
	require.Equal(t, "0", must(AdjustedCountToEncoded(0)))
	require.Equal(t, "1", must(AdjustedCountToEncoded(1)))
	require.Equal(t, "2", must(AdjustedCountToEncoded(2)))

	const largest uint64 = 0x1p+56
	require.Equal(t, "72057594037927936", must(AdjustedCountToEncoded(largest)))
	require.Equal(t, fmt.Sprint(largest-1), must(AdjustedCountToEncoded(largest-1)))
}

func TestInvalidAdjustedCountToEncoded(t *testing.T) {
	// Because unsigned, no too-small value.
	require.Error(t, mustNot(AdjustedCountToEncoded(0x1p56+1)))
	require.Error(t, mustNot(AdjustedCountToEncoded(math.MaxInt64)))
}

func TestValidProbabilityToEncoded(t *testing.T) {
	require.Equal(t, "0x1p-01", must(ProbabilityToEncoded(0.5, 'x', -1)))
	require.Equal(t, "0x1p-56", must(ProbabilityToEncoded(0x1p-56, 'x', -1)))
	require.Equal(t, "0x1.555p-02", must(ProbabilityToEncoded(1/3., 'x', 3)))
	require.Equal(t, "0", must(ProbabilityToEncoded(0, 'x', 3)))
	require.Equal(t, "0", must(ProbabilityToEncoded(0, 'f', 4)))
}

func TestInvalidProbabilityToEncoded(t *testing.T) {
	// Too small
	require.Error(t, mustNot(ProbabilityToEncoded(0x1p-57, 'x', -1)))
	require.Error(t, mustNot(ProbabilityToEncoded(0x1p-57, 'x', 0)))

	// Too big
	require.Error(t, mustNot(ProbabilityToEncoded(1.1, 'x', -1)))
	require.Error(t, mustNot(ProbabilityToEncoded(1.1, 'x', 0)))

	// Bad precision
	require.Error(t, mustNot(ProbabilityToEncoded(0.5, 'x', -3)))
	require.Error(t, mustNot(ProbabilityToEncoded(0.5, 'x', 15)))
}

func testTValueToProb(tv string) (float64, error) {
	p, _, err := EncodedToProbabilityAndAdjustedCount(tv)
	return p, err
}

func testTValueToAdjCount(tv string) (float64, error) {
	_, ac, err := EncodedToProbabilityAndAdjustedCount(tv)
	return ac, err
}

func TestEncodedToProbability(t *testing.T) {
	require.Equal(t, 0.5, must(testTValueToProb("0.5")))
	require.Equal(t, 0.444, must(testTValueToProb("0.444")))
	require.Equal(t, 1.0, must(testTValueToProb("1")))
	require.Equal(t, 0.0, must(testTValueToProb("0")))

	require.InEpsilon(t, 1/3., must(testTValueToProb("3")), 1e-9)
}

func TestEncodedToAdjCount(t *testing.T) {
	require.Equal(t, 2.0, must(testTValueToAdjCount("0.5")))
	require.Equal(t, 2.0, must(testTValueToAdjCount("2")))
	require.Equal(t, 3., must(testTValueToAdjCount("3")))
	require.Equal(t, 5., must(testTValueToAdjCount("5")))

	require.InEpsilon(t, 1/0.444, must(testTValueToAdjCount("0.444")), 1e-9)
	require.InEpsilon(t, 1/0.111111, must(testTValueToAdjCount("0.111111")), 1e-9)

	require.Equal(t, 1.0, must(testTValueToAdjCount("1")))
	require.Equal(t, 0.0, must(testTValueToAdjCount("0")))
}

func TestProbabilityToThreshold(t *testing.T) {
	require.Equal(t,
		Threshold{0x1p+55},
		must(ProbabilityToThreshold(0.5)))
	require.Equal(t,
		Threshold{1},
		must(ProbabilityToThreshold(0x1p-56)))
	require.Equal(t,
		Threshold{0x100},
		must(ProbabilityToThreshold(0x100p-56)))
	require.Equal(t,
		Threshold{2},
		must(ProbabilityToThreshold(0x1p-55)))
	require.Equal(t,
		Threshold{MaxAdjustedCount},
		must(ProbabilityToThreshold(1.0)))

	require.Equal(t,
		Threshold{0x1.555p-2 * MaxAdjustedCount},
		must(ProbabilityToThreshold(0x1.555p-2)))
}

// The two benchmarks below were used to choose the implementation for
// the Threshold type in this package.  The results indicate that it
// is faster to compare a 56-bit number than to compare as 7 element []byte.

type benchTIDs [1024]pcommon.TraceID

func (tids *benchTIDs) init() {
	for i := range tids {
		binary.BigEndian.PutUint64(tids[i][:8], rand.Uint64())
		binary.BigEndian.PutUint64(tids[i][8:], rand.Uint64())
	}
}

// BenchmarkThresholdCompareAsUint64-10    	1000000000	         0.4515 ns/op	       0 B/op	       0 allocs/op
func BenchmarkThresholdCompareAsUint64(b *testing.B) {
	var tids benchTIDs
	var comps [1024]uint64
	tids.init()
	for i := range comps {
		comps[i] = (rand.Uint64() % 0x1p+56) + 1
	}

	b.ReportAllocs()
	b.ResetTimer()
	yes := 0
	no := 0
	for i := 0; i < b.N; i++ {
		tid := tids[i%len(tids)]
		comp := comps[i%len(comps)]
		// Read 8 bytes, mask to 7 bytes
		val := binary.BigEndian.Uint64(tid[8:]) & (0x1p+56 - 1)

		if val < comp {
			yes++
		} else {
			no++
		}
	}
}

// BenchmarkThresholdCompareAsBytes-10     	528679580	         2.288 ns/op	       0 B/op	       0 allocs/op
func BenchmarkThresholdCompareAsBytes(b *testing.B) {
	var tids benchTIDs
	var comps [1024][7]byte
	tids.init()
	for i := range comps {
		var e8 [8]byte
		binary.BigEndian.PutUint64(e8[:], rand.Uint64())
		copy(comps[i][:], e8[1:])
	}

	b.ReportAllocs()
	b.ResetTimer()
	yes := 0
	no := 0
	for i := 0; i < b.N; i++ {
		if bytes.Compare(tids[i%len(tids)][9:], comps[i%len(comps)][:]) <= 0 {
			yes++
		} else {
			no++
		}
	}
}
