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
	otts := otelTraceState{}
	require.False(t, otts.hasTValue())
	require.Equal(t, "", otts.serialize())
}

func TestTraceStatePRValueSerialize(t *testing.T) {
	otts := otelTraceState{}
	otts.tvalueString = "t:3"
	otts.fields = []string{"a:b", "c:d"}
	require.True(t, otts.hasTValue())
	require.Equal(t, "t:3;a:b;c:d", otts.serialize())
}

func TestTraceStateSerializeOverflow(t *testing.T) {
	long := "x:" + strings.Repeat(".", 254)
	otts := otelTraceState{}
	otts.fields = []string{long}
	// this drops the extra key, sorry!
	require.Equal(t, long, otts.serialize())
	otts.tvalueString = "t:1"
	require.Equal(t, "t:1", otts.serialize())
}

func TestParseOTelTraceState(t *testing.T) {
	type testCase struct {
		in        string
		tval      string
		extra     []string
		expectErr error
	}
	const notset = ""
	for _, test := range []testCase{
		// correct cases
		{"", notset, nil, nil},
		{"t:2", "2", nil, nil},
		{"t:1", "1", nil, nil},
		{"t:1", "1", nil, nil},
		{"t:10", "10", nil, nil},
		{"t:33", "33", nil, nil},
		{"t:61", "61", nil, nil},
		{"t:72057594037927936", "72057594037927936", nil, nil}, // max t-value = 0x1p+56
		{"t:0x1p-56", "0x1p-56", nil, nil},                     // min t-value

		// syntax errors
		{"t:1;", notset, nil, strconv.ErrSyntax},
		{"t:1=p:2", notset, nil, strconv.ErrSyntax},
		{"t:1;p:2=s:3", notset, nil, strconv.ErrSyntax},
		{":1;p:2=s:3", notset, nil, strconv.ErrSyntax},
		{":;p:2=s:3", notset, nil, strconv.ErrSyntax},
		{":;:", notset, nil, strconv.ErrSyntax},
		{":", notset, nil, strconv.ErrSyntax},
		{"t:;p=1", notset, nil, strconv.ErrSyntax},
		{"t:$", notset, nil, strconv.ErrSyntax}, // not-hexadecimal

		// range errors
		{"t:0x1p+57", notset, nil, ErrAdjustedCountOnlyInteger},     // integer syntax
		{"t:72057594037927937", notset, nil, ErrAdjustedCountRange}, // out-of-range
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
			otts, err := otelSyntax.parse(test.in)

			if test.expectErr != nil {
				require.True(t, errors.Is(err, test.expectErr), "%q: not expecting %v wanted %v", test.in, err, test.expectErr)
			} else {
				require.NoError(t, err)
			}
			if test.tval != notset {
				require.True(t, otts.hasTValue())
				require.Equal(t, "t:"+test.tval, otts.tvalueString)
			} else {
				require.False(t, otts.hasTValue(), "should have no t-value: %s", otts.tvalueString)
			}
			require.EqualValues(t, test.extra, otts.fields)

			// on success w/o t-value, serialize() should not modify
			if !otts.hasTValue() && test.expectErr == nil {
				require.Equal(t, test.in, otts.serialize())
			}
		})
	}
}
