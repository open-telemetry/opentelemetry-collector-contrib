// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
		// correct cases
		{"ot=th:1", ns, "1", nil, nil},
		{" ot=th:1 ", ns, "1", nil, nil},
		{"ot=th:1", ns, "1", nil, nil},
		{" ot=th:1 ", ns, "1", nil, nil},
		{" ot=th:1,other=value ", ns, "1", map[string]string{
			"other": "value",
		}, nil},
		{"ot=th:1 , other=value", ns, "1", map[string]string{
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
				require.True(t, errors.Is(err, test.expectErr),
					"%q: not expecting %v wanted %v", test.in, err, test.expectErr,
				)
			} else {
				require.NoError(t, err, "%q", test.in)
			}
			if test.rval != ns {
				require.True(t, w3c.HasOTelValue())
				require.True(t, w3c.HasAnyValue())
				require.True(t, w3c.OTelValue().HasRValue())
				require.Equal(t, test.rval, w3c.OTelValue().RValue())
			} else {
				require.False(t, w3c.OTelValue().HasRValue(), "should have no r-value")
			}
			if test.tval != ns {
				require.True(t, w3c.HasOTelValue())
				require.True(t, w3c.HasAnyValue())
				require.True(t, w3c.OTelValue().HasTValue())
				require.Equal(t, test.tval, w3c.OTelValue().TValue())
			} else {
				require.False(t, w3c.OTelValue().HasTValue(), "should have no t-value")
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
			w3c.Serialize(&w)
			cpy, err := NewW3CTraceState(w.String())
			require.NoError(t, err, "with %v", w.String())
			require.Equal(t, w3c, cpy, "with %v", w.String())
		})
	}
}
