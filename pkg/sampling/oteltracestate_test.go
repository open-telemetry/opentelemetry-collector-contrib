// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ExampleOpenTelemetryTraceState_AdjustedCount shows how to access
// the adjusted count for a sampled context when it has non-zero
// probability.
func ExampleOpenTelemetryTraceState_AdjustedCount() {
	w3c, err := NewW3CTraceState("ot=th:c")
	if err != nil {
		panic(err)
	}
	ot := w3c.OTelValue()

	fmt.Printf("Adjusted count for T-value %q: %f", ot.TValue(), ot.AdjustedCount())

	// Output:
	// Adjusted count for T-value "c": 4.000000
}

func testName(in string) string {
	if len(in) > 32 {
		return in[:32] + "..."
	}
	return in
}

func TestEmptyOpenTelemetryTraceState(t *testing.T) {
	// Empty value is invalid
	_, err := NewOpenTelemetryTraceState("")
	require.Error(t, err)
}

func TestOpenTelemetryTraceStateTValueSerialize(t *testing.T) {
	const orig = "rv:10000000000000;th:3;a:b;c:d"
	otts, err := NewOpenTelemetryTraceState(orig)
	require.NoError(t, err)
	require.Equal(t, "3", otts.TValue())
	tv, hasTv := otts.TValueThreshold()
	require.True(t, hasTv)
	require.Equal(t, 1-0x3p-4, tv.Probability())

	require.NotEmpty(t, otts.RValue())
	require.Equal(t, "10000000000000", otts.RValue())
	rv, hasRv := otts.RValueRandomness()
	require.True(t, hasRv)
	require.Equal(t, "10000000000000", rv.RValue())

	require.True(t, otts.HasAnyValue())
	var w strings.Builder
	require.NoError(t, otts.Serialize(&w))
	require.Equal(t, orig, w.String())
}

func TestOpenTelemetryTraceStateZero(t *testing.T) {
	const orig = "th:0"
	otts, err := NewOpenTelemetryTraceState(orig)
	require.NoError(t, err)
	require.True(t, otts.HasAnyValue())
	require.Equal(t, "0", otts.TValue())
	tv, hasTv := otts.TValueThreshold()
	require.True(t, hasTv)
	require.Equal(t, 1.0, tv.Probability())

	var w strings.Builder
	require.NoError(t, otts.Serialize(&w))
	require.Equal(t, orig, w.String())
}

func TestOpenTelemetryTraceStateRValuePValue(t *testing.T) {
	// Ensures the caller can handle RValueSizeError and search
	// for p-value in extra-values.
	const orig = "rv:3;p:2"
	otts, err := NewOpenTelemetryTraceState(orig)
	require.Error(t, err)
	require.Equal(t, ErrRValueSize, err)
	require.Empty(t, otts.RValue())

	// The error is oblivious to the old r-value, but that's ok.
	require.ErrorContains(t, err, "14 hex digits")

	require.Equal(t, []KV{{"p", "2"}}, otts.ExtraValues())

	var w strings.Builder
	require.NoError(t, otts.Serialize(&w))
	require.Equal(t, "p:2", w.String())
}

func TestOpenTelemetryTraceStateTValueUpdate(t *testing.T) {
	const orig = "rv:abcdefabcdefab"
	otts, err := NewOpenTelemetryTraceState(orig)
	require.NoError(t, err)
	require.Empty(t, otts.TValue())
	require.NotEmpty(t, otts.RValue())

	th, _ := TValueToThreshold("3")
	require.NoError(t, otts.UpdateTValueWithSampling(th))

	require.Equal(t, "3", otts.TValue())
	tv, hasTv := otts.TValueThreshold()
	require.True(t, hasTv)
	require.Equal(t, 1-0x3p-4, tv.Probability())

	const updated = "rv:abcdefabcdefab;th:3"
	var w strings.Builder
	require.NoError(t, otts.Serialize(&w))
	require.Equal(t, updated, w.String())
}

func TestOpenTelemetryTraceStateRTUpdate(t *testing.T) {
	otts, err := NewOpenTelemetryTraceState("a:b")
	require.NoError(t, err)
	require.Empty(t, otts.TValue())
	require.Empty(t, otts.RValue())
	require.True(t, otts.HasAnyValue())

	th, _ := TValueToThreshold("3")
	require.NoError(t, otts.UpdateTValueWithSampling(th))
	otts.SetRValue(must(RValueToRandomness("00000000000003")))

	const updated = "rv:00000000000003;th:3;a:b"
	var w strings.Builder
	require.NoError(t, otts.Serialize(&w))
	require.Equal(t, updated, w.String())
}

func TestOpenTelemetryTraceStateRTClear(t *testing.T) {
	otts, err := NewOpenTelemetryTraceState("a:b;rv:12341234123412;th:1234")
	require.NoError(t, err)

	otts.ClearTValue()
	otts.ClearRValue()

	const updated = "a:b"
	var w strings.Builder
	require.NoError(t, otts.Serialize(&w))
	require.Equal(t, updated, w.String())
}

func TestParseOpenTelemetryTraceState(t *testing.T) {
	type testCase struct {
		in        string
		rval      string
		tval      string
		extra     []string
		expectErr error
	}
	const ns = ""
	for _, test := range []testCase{
		// t-value correct cases
		{"th:2", ns, "2", nil, nil},
		{"th:ab", ns, "ab", nil, nil},
		{"th:abcdefabcdefab", ns, "abcdefabcdefab", nil, nil},

		// correct with trailing zeros. the parser does not re-format
		// to remove trailing zeros.
		{"th:1000", ns, "1000", nil, nil},

		// syntax errors
		{"", ns, ns, nil, strconv.ErrSyntax},
		{"th:1;", ns, ns, nil, strconv.ErrSyntax},
		{"th:1=p:2", ns, ns, nil, strconv.ErrSyntax},
		{"th:1;p:2=s:3", ns, ns, nil, strconv.ErrSyntax},
		{":1;p:2=s:3", ns, ns, nil, strconv.ErrSyntax},
		{":;p:2=s:3", ns, ns, nil, strconv.ErrSyntax},
		{":;:", ns, ns, nil, strconv.ErrSyntax},
		{":", ns, ns, nil, strconv.ErrSyntax},
		{"th:;p=1", ns, ns, nil, strconv.ErrSyntax},
		{"th:$", ns, ns, nil, strconv.ErrSyntax},      // not-hexadecimal
		{"th:0x1p+3", ns, ns, nil, strconv.ErrSyntax}, // + is invalid
		{"th:14.5", ns, ns, nil, strconv.ErrSyntax},   // integer syntax
		{"th:-1", ns, ns, nil, strconv.ErrSyntax},     // non-negative

		// too many digits
		{"th:ffffffffffffffff", ns, ns, nil, ErrTValueSize},
		{"th:100000000000000", ns, ns, nil, ErrTValueSize},

		// one field
		{"e100:1", ns, ns, []string{"e100:1"}, nil},

		// two fields
		{"e1:1;e2:2", ns, ns, []string{"e1:1", "e2:2"}, nil},

		// one extra key, two ways
		{"th:2;extra:stuff", ns, "2", []string{"extra:stuff"}, nil},
		{"extra:stuff;th:2", ns, "2", []string{"extra:stuff"}, nil},

		// two extra fields
		{"e100:100;th:1;e101:101", ns, "1", []string{"e100:100", "e101:101"}, nil},
		{"th:1;e100:100;e101:101", ns, "1", []string{"e100:100", "e101:101"}, nil},
		{"e100:100;e101:101;th:1", ns, "1", []string{"e100:100", "e101:101"}, nil},

		// parse error prevents capturing unrecognized keys
		{"1:1;u:V", ns, ns, nil, strconv.ErrSyntax},
		{"X:1;u:V", ns, ns, nil, strconv.ErrSyntax},
		{"x:1;u:V", ns, ns, []string{"x:1", "u:V"}, nil},

		// r-value
		{"rv:22222222222222;extra:stuff", "22222222222222", ns, []string{"extra:stuff"}, nil},
		{"extra:stuff;rv:22222222222222", "22222222222222", ns, []string{"extra:stuff"}, nil},
		{"rv:ffffffffffffff", "ffffffffffffff", ns, nil, nil},

		// r-value range error (15 bytes of hex or more)
		{"rv:100000000000000", ns, ns, nil, ErrRValueSize},
		{"rv:fffffffffffffffff", ns, ns, nil, ErrRValueSize},

		// no trailing ;
		{"x:1;", ns, ns, nil, strconv.ErrSyntax},

		// empty key
		{"x:", ns, ns, []string{"x:"}, nil},

		// charset test
		{"x:0X1FFF;y:.-_-.;z:", ns, ns, []string{"x:0X1FFF", "y:.-_-.", "z:"}, nil},
		{"x1y2z3:1-2-3;y1:y_1;xy:-;th:50", ns, "50", []string{"x1y2z3:1-2-3", "y1:y_1", "xy:-"}, nil},

		// size exceeded
		{"x:" + strings.Repeat("_", 255), ns, ns, nil, ErrTraceStateSize},
		{"x:" + strings.Repeat("_", 254), ns, ns, []string{"x:" + strings.Repeat("_", 254)}, nil},
	} {
		t.Run(testName(test.in), func(t *testing.T) {
			otts, err := NewOpenTelemetryTraceState(test.in)

			if test.expectErr != nil {
				require.ErrorIs(t, err, test.expectErr, "%q: not expecting %v wanted %v", test.in, err, test.expectErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.rval, otts.RValue())
			require.Equal(t, test.tval, otts.TValue())
			var expect []KV
			for _, ex := range test.extra {
				k, v, _ := strings.Cut(ex, ":")
				expect = append(expect, KV{
					Key:   k,
					Value: v,
				})
			}
			require.Equal(t, expect, otts.ExtraValues())

			if test.expectErr != nil {
				return
			}
			// on success Serialize() should not modify
			// test by re-parsing
			var w strings.Builder
			require.NoError(t, otts.Serialize(&w))
			cpy, err := NewOpenTelemetryTraceState(w.String())
			require.NoError(t, err)
			require.Equal(t, otts, cpy)
		})
	}
}

func TestUpdateTValueWithSampling(t *testing.T) {
	type testCase struct {
		// The input otel tracestate; no error conditions tested
		in string

		// The incoming adjusted count; defined whether
		// t-value is present or not.
		adjCountIn float64

		// the update probability; threshold and tvalue are
		// derived from this
		prob float64

		// when update error is expected
		updateErr error

		// output t-value
		out string

		// output adjusted count
		adjCountOut float64
	}
	for _, test := range []testCase{
		// 8/16 in, sampled at 2/16 (smaller prob) => adjCount 8
		{"th:8", 2, 0x2p-4, nil, "th:e", 8},

		// 8/16 in, sampled at 14/16 (larger prob) => error, adjCount 2
		{"th:8", 2, 0xep-4, ErrInconsistentSampling, "th:8", 2},

		// 1/16 in, 50% update (larger prob) => (error)
		{"th:f", 16, 0x8p-4, ErrInconsistentSampling, "th:f", 16},

		// 1/1 sampling in, 1/16 update
		{"th:0", 1, 0x1p-4, nil, "th:f", 16},

		// no t-value in, 1/16 update
		{"", 0, 0x1p-4, nil, "th:f", 16},

		// none in, 100% update
		{"", 0, 1, nil, "th:0", 1},

		// 1/2 in, 100% update (error)
		{"th:8", 2, 1, ErrInconsistentSampling, "th:8", 2},

		// 1/1 in, 0x1p-56 update
		{"th:0", 1, 0x1p-56, nil, "th:ffffffffffffff", 0x1p56},

		// 1/1 in, 0x1p-56 update
		{"th:0", 1, 0x1p-56, nil, "th:ffffffffffffff", 0x1p56},

		// 2/3 in, 1/3 update.  Note that 0x555 + 0xaab = 0x1000.
		{"th:555", 1 / (1 - 0x555p-12), 0x555p-12, nil, "th:aab", 1 / (1 - 0xaabp-12)},
	} {
		t.Run(test.in+"/"+test.out, func(t *testing.T) {
			otts := OpenTelemetryTraceState{}
			if test.in != "" {
				var err error
				otts, err = NewOpenTelemetryTraceState(test.in)
				require.NoError(t, err)
			}

			require.Equal(t, test.adjCountIn, otts.AdjustedCount())

			newTh, err := ProbabilityToThreshold(test.prob)
			require.NoError(t, err)

			upErr := otts.UpdateTValueWithSampling(newTh)

			require.Equal(t, test.updateErr, upErr)

			var outData strings.Builder
			err = otts.Serialize(&outData)
			require.NoError(t, err)
			require.Equal(t, test.out, outData.String())

			require.Equal(t, test.adjCountOut, otts.AdjustedCount())
		})
	}
}
