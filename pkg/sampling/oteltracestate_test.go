// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testName(in string) string {
	x := strings.NewReplacer(":", "_", ";", "_").Replace(in)
	if len(x) > 32 {
		return ""
	}
	return x
}

func TestNewTraceState(t *testing.T) {
	otts := newTraceState()
	require.False(t, otts.hasTValue())
	require.Equal(t, "", otts.serialize())
}

func TestTraceStatePRValueSerialize(t *testing.T) {
	otts := newTraceState()
	otts.tvalueString = "t:3"
	otts.unknown = []string{"a:b", "c:d"}
	require.True(t, otts.hasTValue())
	require.Equal(t, "t:3;a:b;c:d", otts.serialize())
}

func TestTraceStateSerializeOverflow(t *testing.T) {
	long := "x:" + strings.Repeat(".", 254)
	otts := newTraceState()
	otts.unknown = []string{long}
	// this drops the extra key, sorry!
	require.Equal(t, long, otts.serialize())
	otts.tvalueString = "t:1"
	require.Equal(t, "t:1", otts.serialize())
}

// func TestParseTraceStateForTraceID(t *testing.T) {
// 	type testCase struct {
// 		in        string
// 		rval      uint8
// 		expectErr error
// 	}
// 	const notset = 255
// 	for _, test := range []testCase{
// 		// All are unsampled tests, i.e., `sampled` is not set in traceparent.
// 		{"r:2", 2, nil},
// 		{"r:1;", notset, strconv.ErrSyntax},
// 		{"r:1", 1, nil},
// 		{"r:1=p:2", notset, strconv.ErrSyntax},
// 		{"r:1;p:2=s:3", notset, strconv.ErrSyntax},
// 		{":1;p:2=s:3", notset, strconv.ErrSyntax},
// 		{":;p:2=s:3", notset, strconv.ErrSyntax},
// 		{":;:", notset, strconv.ErrSyntax},
// 		{":", notset, strconv.ErrSyntax},
// 		{"", notset, nil},
// 		{"r:;p=1", notset, strconv.ErrSyntax},
// 		{"r:1", 1, nil},
// 		{"r:10", 10, nil},
// 		{"r:33", 33, nil},
// 		{"r:61", 61, nil},
// 		{"r:62", 62, nil},                      // max r-value
// 		{"r:63", notset, strconv.ErrRange},     // out-of-range
// 		{"r:100", notset, strconv.ErrRange},    // out-of-range
// 		{"r:100001", notset, strconv.ErrRange}, // out-of-range
// 		{"p:64", notset, strconv.ErrRange},
// 		{"p:100", notset, strconv.ErrRange},
// 		{"r:1a", notset, strconv.ErrSyntax}, // not-hexadecimal
// 		{"p:-1", notset, strconv.ErrSyntax}, // non-negative
// 	} {
// 		t.Run(testName(test.in), func(t *testing.T) {
// 			// Note: passing isSampled=false as stated above.
// 			ts := pcommon.NewTraceState(test.in)
// 			otts, err := parseOTelTraceState(ts, false)

// 			require.False(t, otts.hasTValue(), "should have no p-value")

// 			if test.expectErr != nil {
// 				require.True(t, errors.Is(err, test.expectErr), "not expecting %v", err)
// 			}
// 			if test.rval != notset {
// 				require.True(t, otts.hasRValue())
// 				require.Equal(t, test.rval, otts.rvalue)
// 			} else {
// 				require.False(t, otts.hasRValue(), "should have no r-value")
// 			}
// 			require.EqualValues(t, []string(nil), otts.unknown)

// 			if test.expectErr == nil {
// 				// Require serialize to round-trip
// 				otts2, err := parseOTelTraceState(otts.serialize(), false)
// 				require.NoError(t, err)
// 				require.Equal(t, otts, otts2)
// 			}
// 		})
// 	}
// }

// func TestParseTraceStateSampled(t *testing.T) {
// 	type testCase struct {
// 		in         string
// 		rval, pval uint8
// 		expectErr  error
// 	}
// 	const notset = 255
// 	for _, test := range []testCase{
// 		// All are sampled tests, i.e., `sampled` is set in traceparent.
// 		{"r:2;p:2", 2, 2, nil},
// 		{"r:2;p:1", 2, 1, nil},
// 		{"r:2;p:0", 2, 0, nil},

// 		{"r:1;p:1", 1, 1, nil},
// 		{"r:1;p:0", 1, 0, nil},

// 		{"r:0;p:0", 0, 0, nil},

// 		{"r:62;p:0", 62, 0, nil},
// 		{"r:62;p:62", 62, 62, nil},

// 		// The important special case:
// 		{"r:0;p:63", 0, 63, nil},
// 		{"r:2;p:63", 2, 63, nil},
// 		{"r:62;p:63", 62, 63, nil},

// 		// Inconsistent p causes unset p-value.
// 		{"r:2;p:3", 2, notset, errTraceStateInconsistent},
// 		{"r:2;p:4", 2, notset, errTraceStateInconsistent},
// 		{"r:2;p:62", 2, notset, errTraceStateInconsistent},
// 		{"r:0;p:1", 0, notset, errTraceStateInconsistent},
// 		{"r:1;p:2", 1, notset, errTraceStateInconsistent},
// 		{"r:61;p:62", 61, notset, errTraceStateInconsistent},

// 		// Inconsistent r causes unset p-value and r-value.
// 		{"r:63;p:2", notset, notset, strconv.ErrRange},
// 		{"r:120;p:2", notset, notset, strconv.ErrRange},
// 		{"r:ab;p:2", notset, notset, strconv.ErrSyntax},

// 		// Syntax is tested before range errors
// 		{"r:ab;p:77", notset, notset, strconv.ErrSyntax},

// 		// p without r (when sampled)
// 		{"p:1", notset, 1, nil},
// 		{"p:62", notset, 62, nil},
// 		{"p:63", notset, 63, nil},

// 		// r without p (when sampled)
// 		{"r:2", 2, notset, nil},
// 		{"r:62", 62, notset, nil},
// 		{"r:0", 0, notset, nil},
// 	} {
// 		t.Run(testName(test.in), func(t *testing.T) {
// 			// Note: passing isSampled=true as stated above.
// 			otts, err := parseOTelTraceState(test.in, true)

// 			if test.expectErr != nil {
// 				require.True(t, errors.Is(err, test.expectErr), "not expecting %v", err)
// 			} else {
// 				require.NoError(t, err)
// 			}
// 			if test.pval != notset {
// 				require.True(t, otts.hasTValue())
// 				require.Equal(t, test.pval, otts.pvalue)
// 			} else {
// 				require.False(t, otts.hasTValue(), "should have no p-value")
// 			}
// 			if test.rval != notset {
// 				require.True(t, otts.hasRValue())
// 				require.Equal(t, test.rval, otts.rvalue)
// 			} else {
// 				require.False(t, otts.hasRValue(), "should have no r-value")
// 			}
// 			require.EqualValues(t, []string(nil), otts.unknown)

// 			if test.expectErr == nil {
// 				// Require serialize to round-trip
// 				otts2, err := parseOTelTraceState(otts.serialize(), true)
// 				require.NoError(t, err)
// 				require.Equal(t, otts, otts2)
// 			}
// 		})
// 	}
// }

func TestParseTraceStateExtra(t *testing.T) {
	type testCase struct {
		in        string
		tval      string
		extra     []string
		expectErr error
	}
	const notset = ""
	for _, test := range []testCase{
		//
		{"t:2", "2", nil, nil},
		{"t:1;", notset, nil, strconv.ErrSyntax},
		{"t:1", "1", nil, nil},
		{"t:1=p:2", notset, nil, strconv.ErrSyntax},
		{"t:1;p:2=s:3", notset, nil, strconv.ErrSyntax},
		{":1;p:2=s:3", notset, nil, strconv.ErrSyntax},
		{":;p:2=s:3", notset, nil, strconv.ErrSyntax},
		{":;:", notset, nil, strconv.ErrSyntax},
		{":", notset, nil, strconv.ErrSyntax},
		{"", notset, nil, nil},
		{"t:;p=1", notset, nil, strconv.ErrSyntax},
		{"t:1", "1", nil, nil},
		{"t:10", "10", nil, nil},
		{"t:33", "33", nil, nil},
		{"t:61", "61", nil, nil},
		{"t:72057594037927936", "72057594037927936", nil, nil}, // max t-value = 0x1p+56
		{"t:0x1p-56", "0x1p-56", nil, nil},                     // min t-value

		// various errors
		{"t:0x1p+57", notset, nil, ErrAdjustedCountOnlyInteger},     // integer syntax
		{"t:72057594037927937", notset, nil, ErrAdjustedCountRange}, // out-of-range
		{"t:$", notset, nil, strconv.ErrSyntax},                     // not-hexadecimal
		{"t:-1", notset, nil, ErrProbabilityRange},                  // non-negative

		// one field
		{"e100:1", notset, []string{"e100:1"}, nil},

		// two fields
		{"e1:1;e2:2", notset, []string{"e1:1", "e2:2"}, nil},
		{"e1:1;e2:2", notset, []string{"e1:1", "e2:2"}, nil},

		// one extra key, two ways
		{"t:2;extra:stuff", "2", []string{"extra:stuff"}, nil},
		{"extra:stuff;t:2", "2", []string{"extra:stuff"}, nil},

		// two extra fields
		{"e100:100;t:1;e101:101", "1", []string{"e100:100", "e101:101"}, nil},
		{"t:1;e100:100;e101:101", "1", []string{"e100:100", "e101:101"}, nil},
		{"e100:100;e101:101;t:1", "1", []string{"e100:100", "e101:101"}, nil},

		// parse error prevents capturing unrecognized keys
		{"1:1;u:V", notset, nil, strconv.ErrSyntax},
		{"X:1;u:V", notset, nil, strconv.ErrSyntax},
		{"x:1;u:V", notset, []string{"x:1", "u:V"}, nil},

		// no trailing ;
		{"x:1;", notset, nil, strconv.ErrSyntax},

		// empty key
		{"x:", notset, []string{"x:"}, nil},

		// charset test
		{"x:0X1FFF;y:.-_-.;z:", notset, []string{"x:0X1FFF", "y:.-_-.", "z:"}, nil},
		{"x1y2z3:1-2-3;y1:y_1;xy:-;t:50", "50", []string{"x1y2z3:1-2-3", "y1:y_1", "xy:-"}, nil},

		// size exceeded
		{"x:" + strings.Repeat("_", 255), notset, nil, strconv.ErrSyntax},
		{"x:" + strings.Repeat("_", 254), notset, []string{"x:" + strings.Repeat("_", 254)}, nil},
	} {
		t.Run(testName(test.in), func(t *testing.T) {
			// Note: These tests are independent of sampling state,
			// so both are tested.
			otts, err := parseOTelTraceState(test.in)

			if test.expectErr != nil {
				require.True(t, errors.Is(err, test.expectErr), "%q: not expecting %v wanted %v", test.in, err, test.expectErr)
			} else {
				require.NoError(t, err)
			}
			if test.tval != notset {
				require.True(t, otts.hasTValue())
				require.Equal(t, "t:"+test.tval, otts.tvalueString)
			} else {

				require.False(t, otts.hasTValue(), "should have no t-value")
			}
			require.EqualValues(t, test.extra, otts.unknown)

			// on success w/o t-value, serialize() should not modify
			if !otts.hasTValue() && test.expectErr == nil {
				require.Equal(t, test.in, otts.serialize())
			}
		})
	}
}
