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

func TestEmptyOTelTraceState(t *testing.T) {
	// Empty value is invalid
	_, err := NewOTelTraceState("")
	require.Error(t, err)
}

func TestOTelTraceStateTValueSerialize(t *testing.T) {
	const orig = "r:1;s:2;t:3;a:b;c:d"
	otts, err := NewOTelTraceState(orig)
	require.NoError(t, err)
	require.True(t, otts.HasTValue())
	require.Equal(t, "3", otts.TValue())

	require.True(t, otts.HasSValue())
	require.Equal(t, "2", otts.SValue())

	require.True(t, otts.HasRValue())
	require.Equal(t, "1", otts.RValue())

	require.True(t, otts.HasAnyValue())
	var w strings.Builder
	otts.Serialize(&w)
	require.Equal(t, orig, w.String())
}

func TestParseOTelTraceState(t *testing.T) {
	type testCase struct {
		in        string
		rval      string
		sval      string
		tval      string
		extra     []string
		expectErr error
	}
	const ns = ""
	for _, test := range []testCase{
		// t-value correct cases
		{"t:2", ns, ns, "2", nil, nil},
		{"t:1", ns, ns, "1", nil, nil},
		{"t:1", ns, ns, "1", nil, nil},
		{"t:10", ns, ns, "10", nil, nil},
		{"t:33", ns, ns, "33", nil, nil},
		{"t:61", ns, ns, "61", nil, nil},
		{"t:72057594037927936", ns, ns, "72057594037927936", nil, nil}, // max t-value = 0x1p+56
		{"t:0x1p-56", ns, ns, "0x1p-56", nil, nil},                     // min t-value

		// syntax errors
		{"", ns, ns, ns, nil, strconv.ErrSyntax},
		{"t:1;", ns, ns, ns, nil, strconv.ErrSyntax},
		{"t:1=p:2", ns, ns, ns, nil, strconv.ErrSyntax},
		{"t:1;p:2=s:3", ns, ns, ns, nil, strconv.ErrSyntax},
		{":1;p:2=s:3", ns, ns, ns, nil, strconv.ErrSyntax},
		{":;p:2=s:3", ns, ns, ns, nil, strconv.ErrSyntax},
		{":;:", ns, ns, ns, nil, strconv.ErrSyntax},
		{":", ns, ns, ns, nil, strconv.ErrSyntax},
		{"t:;p=1", ns, ns, ns, nil, strconv.ErrSyntax},
		{"t:$", ns, ns, ns, nil, strconv.ErrSyntax},      // not-hexadecimal
		{"t:0x1p+3", ns, ns, ns, nil, strconv.ErrSyntax}, // + is invalid

		// range errors
		{"t:14.5", ns, ns, ns, nil, ErrAdjustedCountOnlyInteger},        // integer syntax
		{"t:72057594037927937", ns, ns, ns, nil, ErrAdjustedCountRange}, // out-of-range
		{"t:-1", ns, ns, ns, nil, ErrProbabilityRange},                  // non-negative

		// one field
		{"e100:1", ns, ns, ns, []string{"e100:1"}, nil},

		// two fields
		{"e1:1;e2:2", ns, ns, ns, []string{"e1:1", "e2:2"}, nil},
		{"e1:1;e2:2", ns, ns, ns, []string{"e1:1", "e2:2"}, nil},

		// one extra key, two ways
		{"t:2;extra:stuff", ns, ns, "2", []string{"extra:stuff"}, nil},
		{"extra:stuff;t:2", ns, ns, "2", []string{"extra:stuff"}, nil},

		// two extra fields
		{"e100:100;t:1;e101:101", ns, ns, "1", []string{"e100:100", "e101:101"}, nil},
		{"t:1;e100:100;e101:101", ns, ns, "1", []string{"e100:100", "e101:101"}, nil},
		{"e100:100;e101:101;t:1", ns, ns, "1", []string{"e100:100", "e101:101"}, nil},

		// parse error prevents capturing unrecognized keys
		{"1:1;u:V", ns, ns, ns, nil, strconv.ErrSyntax},
		{"X:1;u:V", ns, ns, ns, nil, strconv.ErrSyntax},
		{"x:1;u:V", ns, ns, ns, []string{"x:1", "u:V"}, nil},

		// s-value
		{"s:2;extra:stuff", ns, "2", ns, []string{"extra:stuff"}, nil},
		{"extra:stuff;s:2", ns, "2", ns, []string{"extra:stuff"}, nil},

		// s-value range error
		{"s:0x1p-58", ns, ns, ns, nil, ErrProbabilityRange},
		{"s:-1", ns, ns, ns, nil, ErrProbabilityRange},

		// r-value
		{"r:2;extra:stuff", "2", ns, ns, []string{"extra:stuff"}, nil},
		{"extra:stuff;r:2", "2", ns, ns, []string{"extra:stuff"}, nil},
		{"r:ffffffffffffff", "ffffffffffffff", ns, ns, nil, nil},
		{"r:8888", "8888", ns, ns, nil, nil},
		{"r:0", "0", ns, ns, nil, nil},

		// r-value range error (15 bytes of hex or more)
		{"r:100000000000000", ns, ns, ns, nil, ErrRandomValueRange},
		{"r:fffffffffffffffff", ns, ns, ns, nil, strconv.ErrRange},

		// no trailing ;
		{"x:1;", ns, ns, ns, nil, strconv.ErrSyntax},

		// empty key
		{"x:", ns, ns, ns, []string{"x:"}, nil},

		// charset test
		{"x:0X1FFF;y:.-_-.;z:", ns, ns, ns, []string{"x:0X1FFF", "y:.-_-.", "z:"}, nil},
		{"x1y2z3:1-2-3;y1:y_1;xy:-;t:50", ns, ns, "50", []string{"x1y2z3:1-2-3", "y1:y_1", "xy:-"}, nil},

		// size exceeded
		{"x:" + strings.Repeat("_", 255), ns, ns, ns, nil, ErrTraceStateSize},
		{"x:" + strings.Repeat("_", 254), ns, ns, ns, []string{"x:" + strings.Repeat("_", 254)}, nil},
	} {
		t.Run(testName(test.in), func(t *testing.T) {
			otts, err := NewOTelTraceState(test.in)

			if test.expectErr != nil {
				require.True(t, errors.Is(err, test.expectErr), "%q: not expecting %v wanted %v", test.in, err, test.expectErr)
			} else {
				require.NoError(t, err)
			}
			if test.rval != ns {
				require.True(t, otts.HasRValue())
				require.Equal(t, test.rval, otts.RValue())
			} else {
				require.False(t, otts.HasRValue(), "should have no r-value: %s", otts.RValue())
			}
			if test.sval != ns {
				require.True(t, otts.HasSValue())
				require.Equal(t, test.sval, otts.SValue())
			} else {
				require.False(t, otts.HasSValue(), "should have no s-value: %s", otts.SValue())
			}
			if test.tval != ns {
				require.True(t, otts.HasTValue())
				require.Equal(t, test.tval, otts.TValue())
			} else {
				require.False(t, otts.HasTValue(), "should have no t-value: %s", otts.TValue())
			}
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
			otts.Serialize(&w)
			cpy, err := NewOTelTraceState(w.String())
			require.NoError(t, err)
			require.Equal(t, otts, cpy)
		})
	}
}
