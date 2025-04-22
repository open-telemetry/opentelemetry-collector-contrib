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

// ExampleW3CTraceState_Serialize shows how to parse and print a W3C
// tracestate.
func ExampleW3CTraceState() {
	// This tracestate value encodes two sections, "ot" from
	// OpenTelemetry and "zz" from a vendor.
	w3c, err := NewW3CTraceState("ot=th:c;rv:d29d6a7215ced0;pn:abc,zz=vendorcontent")
	if err != nil {
		panic(err)
	}
	ot := w3c.OTelValue()

	fmt.Println("T-Value:", ot.TValue())
	fmt.Println("R-Value:", ot.RValue())
	fmt.Println("OTel Extra:", ot.ExtraValues())
	fmt.Println("Other Extra:", w3c.ExtraValues())

	// Output:
	// T-Value: c
	// R-Value: d29d6a7215ced0
	// OTel Extra: [{pn abc}]
	// Other Extra: [{zz vendorcontent}]
}

// ExampleW3CTraceState_Serialize shows how to modify and serialize a
// new W3C tracestate.
func ExampleW3CTraceState_Serialize() {
	w3c, err := NewW3CTraceState("")
	if err != nil {
		panic(err)
	}
	// Suppose a parent context was unsampled, the child span has
	// been sampled at 25%.  The child's context should carry the
	// T-value of "c", serialize as "ot=th:c".
	th, err := ProbabilityToThreshold(0.25)
	if err != nil {
		panic(err)
	}

	// The update uses both the Threshold and its encoded string
	// value, since in some code paths the Threshold will have
	// just been parsed from a T-value, and in other code paths
	// the T-value will be precalculated.
	err = w3c.OTelValue().UpdateTValueWithSampling(th)
	if err != nil {
		panic(err)
	}

	var buf strings.Builder
	err = w3c.Serialize(&buf)
	if err != nil {
		panic(err)
	}

	fmt.Println(buf.String())

	// Output:
	// ot=th:c
}

func TestParseW3CTraceState(t *testing.T) {
	type testCase struct {
		in        string
		rval      string
		tval      string
		extra     map[string]string
		expectErr error
	}
	const ns = ""
	for _, test := range []testCase{
		// correct cases, with various whitespace
		{"ot=th:1", ns, "1", nil, nil},
		{" ot=th:1 ", ns, "1", nil, nil},
		{" ot=th:1,other=value ", ns, "1", map[string]string{
			"other": "value",
		}, nil},
		{",,,", ns, ns, nil, nil},
		{" , ot=th:1, , other=value ", ns, "1", map[string]string{
			"other": "value",
		}, nil},
		{"ot=th:100;rv:abcdabcdabcdff", "abcdabcdabcdff", "100", nil, nil},
		{" ot=th:100;rv:abcdabcdabcdff", "abcdabcdabcdff", "100", nil, nil},
		{"ot=th:100;rv:abcdabcdabcdff ", "abcdabcdabcdff", "100", nil, nil},
		{"ot=rv:11111111111111", "11111111111111", ns, nil, nil},
		{"ot=rv:ffffffffffffff,unknown=value,other=something", "ffffffffffffff", ns, map[string]string{
			"other":   "something",
			"unknown": "value",
		}, nil},

		// syntax errors
		{"-1=2", ns, ns, nil, strconv.ErrSyntax}, // invalid key char
		{"=", ns, ns, nil, strconv.ErrSyntax},    // invalid empty key

		// size errors
		{strings.Repeat("x", hardMaxKeyLength+1) + "=v", ns, ns, nil, ErrTraceStateSize},           // too long simple key
		{strings.Repeat("x", hardMaxTenantLength+1) + "@y=v", ns, ns, nil, ErrTraceStateSize},      // too long multitenant-id
		{"y@" + strings.Repeat("x", hardMaxSystemLength+1) + "=v", ns, ns, nil, ErrTraceStateSize}, // too long system-id
		{"x=" + strings.Repeat("y", hardMaxW3CLength-1), ns, ns, nil, ErrTraceStateSize},
		{strings.Repeat("x=y,", hardMaxNumPairs) + "x=y", ns, ns, nil, ErrTraceStateSize},
	} {
		t.Run(testName(test.in), func(t *testing.T) {
			w3c, err := NewW3CTraceState(test.in)

			if test.expectErr != nil {
				require.ErrorIs(t, err, test.expectErr,
					"%q: not expecting %v wanted %v", test.in, err, test.expectErr,
				)
			} else {
				require.NoError(t, err, "%q", test.in)
			}
			if test.rval != ns {
				require.True(t, w3c.OTelValue().HasAnyValue())
				require.True(t, w3c.HasAnyValue())
				require.Equal(t, test.rval, w3c.OTelValue().RValue())
			} else {
				require.Empty(t, w3c.OTelValue().RValue())
			}
			if test.tval != ns {
				require.True(t, w3c.OTelValue().HasAnyValue())
				require.True(t, w3c.HasAnyValue())
				require.NotEmpty(t, w3c.OTelValue().TValue())
				require.Equal(t, test.tval, w3c.OTelValue().TValue())
			} else {
				require.Empty(t, w3c.OTelValue().TValue())
			}
			if test.extra != nil {
				require.True(t, w3c.HasAnyValue())
				actual := map[string]string{}
				for _, kv := range w3c.ExtraValues() {
					actual[kv.Key] = kv.Value
				}
				require.Equal(t, test.extra, actual)
			}

			if test.expectErr != nil {
				return
			}
			// on success Serialize() should not modify
			// test by re-parsing
			var w strings.Builder
			require.NoError(t, w3c.Serialize(&w))
			cpy, err := NewW3CTraceState(w.String())
			require.NoError(t, err, "with %v", w.String())
			require.Equal(t, w3c, cpy, "with %v", w.String())
		})
	}
}
