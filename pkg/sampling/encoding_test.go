// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"encoding/binary"
	"errors"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// must panics when the error is non-nil and returns the
// generic argument.  this does not call require.NoError() in order to
// use a one-line test calling convention, meaning `must(functionCall())`
// ensures there is no error and returns the first argument.
//
// To do the same with NoError() means rewriting the expression with
// a two-line statement:
//
//	value, err := functionCall()
//	require.NoError(t, value)
func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

// mustNot panics when the error is nil.  this does not call
// require.Error() in order to use a one-line test calling
// convention, meaning `mustNot(functionCall())` ensures there is an
// error without requiring a separate variable assignment.
//
// To do the same with Error() means rewriting the expression with
// a two-line statement:
//
//	_, err := functionCall()
//	require.Error(t, value)
func mustNot[T any](_ T, err error) error {
	if err == nil {
		return errors.New("expected an error, got nil")
	}
	return err
}

func probabilityToTValue(prob float64) (string, error) {
	th, err := ProbabilityToThreshold(prob)
	return th.TValue(), err
}

func tValueToProbability(tv string) (float64, error) {
	th, err := TValueToThreshold(tv)
	return th.Probability(), err
}

func TestValidProbabilityToTValue(t *testing.T) {
	require.Equal(t, "0", must(probabilityToTValue(1.0)))
	require.Equal(t, "8", must(probabilityToTValue(0.5)))
	require.Equal(t, "ffffffffffffff", must(probabilityToTValue(0x1p-56)))
	require.Equal(t, "aaaaaaaaaaaaac", must(probabilityToTValue(1/3.)))
	require.Equal(t, "55555555555558", must(probabilityToTValue(2/3.)))
	require.Equal(t, "54", must(probabilityToTValue(1-0x54p-8))) // 0x54p-8 is approximately 1/3
	require.Equal(t, "01", must(probabilityToTValue(1-0x1p-8)))
}

func TestThresholdGreater(t *testing.T) {
	require.True(t, ThresholdGreater(
		must(TValueToThreshold("5")),
		must(TValueToThreshold("4")),
	))

	require.True(t, ThresholdGreater(
		must(TValueToThreshold("4")),
		must(TValueToThreshold("04")),
	))

	require.False(t, ThresholdGreater(
		must(TValueToThreshold("234")),
		must(TValueToThreshold("4")),
	))

	require.True(t, ThresholdGreater(
		must(TValueToThreshold("4")),
		must(TValueToThreshold("234")),
	))
}

func TestInvalidprobabilityToTValue(t *testing.T) {
	// Too small
	require.Error(t, mustNot(probabilityToTValue(0x1p-57)))
	require.Error(t, mustNot(probabilityToTValue(0x1p-57)))

	// Too big
	require.Error(t, mustNot(probabilityToTValue(1.1)))
	require.Error(t, mustNot(probabilityToTValue(1.1)))
}

func TestTValueToProbability(t *testing.T) {
	require.Equal(t, 0.5, must(tValueToProbability("8")))
	require.Equal(t, 1-0x444p-12, must(tValueToProbability("444")))
	require.Equal(t, 1.0, must(tValueToProbability("0")))

	// 0x55555554p-32 is very close to 1/3
	require.InEpsilon(t, 1-1/3., must(tValueToProbability("55555554")), 1e-9)
}

func TestProbabilityToThreshold(t *testing.T) {
	require.Equal(t,
		must(TValueToThreshold("8")),
		must(ProbabilityToThreshold(0.5)))
	require.Equal(t,
		must(TValueToThreshold("ffffffffffffff")),
		must(ProbabilityToThreshold(0x1p-56)))
	require.Equal(t,
		must(TValueToThreshold("ffffffffffff00")),
		must(ProbabilityToThreshold(0x100p-56)))
	require.Equal(t,
		must(TValueToThreshold("00000000000010")),
		must(ProbabilityToThreshold(1.0-0x1p-52)))
	require.Equal(t,
		AlwaysSampleThreshold,
		must(ProbabilityToThreshold(1.0)))

	zt, err := ProbabilityToThreshold(0)
	require.Equal(t, zt, AlwaysSampleThreshold)
	require.Error(t, err)
	require.Equal(t, err, ErrProbabilityRange)
}

func TestShouldSample(t *testing.T) {
	// Test four boundary conditions for 50% sampling,
	thresh := must(ProbabilityToThreshold(0.5))
	// Smallest TraceID that should NOT sample.
	require.False(t, thresh.ShouldSample(TraceIDToRandomness(pcommon.TraceID{
		// 9 meaningless bytes
		0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
		0, // randomness starts here
		0, 0, 0, 0, 0, 0,
	})))
	// Largest TraceID that should NOT sample.
	require.False(t, thresh.ShouldSample(TraceIDToRandomness(pcommon.TraceID{
		// 9 meaningless bytes
		0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
		0x7f, // randomness starts here
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	})))
	// Smallest TraceID that should sample.
	require.True(t, thresh.ShouldSample(TraceIDToRandomness(pcommon.TraceID{
		// 9 meaningless bytes
		0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
		0x80, // randomness starts here
		0, 0, 0, 0, 0, 0,
	})))
	// Largest TraceID that should sample.
	require.True(t, thresh.ShouldSample(TraceIDToRandomness(pcommon.TraceID{
		// 9 meaningless bytes
		0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
		0xff, // randomness starts here
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	})))
}

func TestRValueSyntax(t *testing.T) {
	type testCase struct {
		in        string
		expectErr error
	}
	for _, test := range []testCase{
		// correct cases
		{"12341234123412", nil},

		// wrong size
		{"123412341234120", ErrRValueSize},
		{"1234123412341", ErrRValueSize},
		{"", ErrRValueSize},

		// bad syntax
		{"abcdefgabcdefg", strconv.ErrSyntax},
	} {
		t.Run(testName(test.in), func(t *testing.T) {
			rnd, err := RValueToRandomness(test.in)

			if test.expectErr != nil {
				require.ErrorIs(t, err, test.expectErr,
					"%q: not expecting %v wanted %v", test.in, err, test.expectErr,
				)
				require.Equal(t, must(RValueToRandomness("00000000000000")), rnd)
			} else {
				require.NoError(t, err, "%q", test.in)

				val, err := strconv.ParseUint(test.in, 16, 64)
				require.NoError(t, err)

				require.Equal(t, TraceIDToRandomness(
					// This explicitly constructs a TraceID from 9 random
					// bytes plus the 7 lowest bytes of the input value.
					pcommon.TraceID{
						byte(rand.IntN(256)),   // 0
						byte(rand.IntN(256)),   // 1
						byte(rand.IntN(256)),   // 2
						byte(rand.IntN(256)),   // 3
						byte(rand.IntN(256)),   // 4
						byte(rand.IntN(256)),   // 5
						byte(rand.IntN(256)),   // 6
						byte(rand.IntN(256)),   // 7
						byte(rand.IntN(256)),   // 8
						byte(val >> 48 & 0xff), // 9
						byte(val >> 40 & 0xff), // 10
						byte(val >> 32 & 0xff), // 11
						byte(val >> 24 & 0xff), // 12
						byte(val >> 16 & 0xff), // 13
						byte(val >> 8 & 0xff),  // 14
						byte(val >> 0 & 0xff),  // 15
					},
				), rnd)
			}
		})
	}
}

func TestTValueSyntax(t *testing.T) {
	type testCase struct {
		in        string
		expectErr error
	}
	for _, test := range []testCase{
		// correct cases
		{"1", nil},

		// syntax error
		{"", ErrTValueEmpty},
		{"g", strconv.ErrSyntax},
	} {
		t.Run(testName(test.in), func(t *testing.T) {
			_, err := TValueToThreshold(test.in)

			if test.expectErr != nil {
				require.ErrorIs(t, err, test.expectErr,
					"%q: not expecting %v wanted %v", test.in, err, test.expectErr,
				)
			} else {
				require.NoError(t, err, "%q", test.in)
			}
		})
	}
}

// There were two benchmarks used to choose the implementation for the
// Threshold type in this package.  The results indicate that it is
// faster to compare a 56-bit number than to compare as 7 element
// []byte.

type benchTIDs [1024]pcommon.TraceID

func (tids *benchTIDs) init() {
	for i := range tids {
		binary.BigEndian.PutUint64(tids[i][:8], rand.Uint64())
		binary.BigEndian.PutUint64(tids[i][8:], rand.Uint64())
	}
}

// The current implementation, using unsigned:
//
//	BenchmarkThresholdCompareAsUint64-10    	1000000000	         0.4515 ns/op	       0 B/op	       0 allocs/op
//
// vs the tested and rejected, using bytes:
//
//	BenchmarkThresholdCompareAsBytes-10     	528679580	         2.288 ns/op	       0 B/op	       0 allocs/op
func BenchmarkThresholdCompareAsUint64(b *testing.B) {
	var tids benchTIDs
	var comps [1024]Threshold
	tids.init()
	for i := range comps {
		var err error
		comps[i], err = ProbabilityToThreshold(rand.Float64())
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	yes := 0
	no := 0
	for i := 0; i < b.N; i++ {
		idx := i % len(tids)
		tid := tids[idx]
		comp := comps[idx]

		if comp.ShouldSample(TraceIDToRandomness(tid)) {
			yes++
		} else {
			no++
		}
	}
}
