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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseW3CTraceState(t *testing.T) {
	type testCase struct {
		in        string
		rval      string
		sval      string
		tval      string
		expectErr error
	}
	const ns = ""
	for _, test := range []testCase{
		// correct cases
		{"ot=t:1", ns, ns, "1", nil},
		{"ot=t:100", ns, ns, "100", nil},
		{"ot=s:100;t:200", ns, "100", "200", nil},
		{"ot=r:1", "1", ns, ns, nil},
		{"ot=r:1,unknown:value,other=something", "1", ns, ns, nil},
	} {
		t.Run(testName(test.in), func(t *testing.T) {
			w3c, err := NewW3CTraceState(test.in)

			if test.expectErr != nil {
				require.True(t, errors.Is(err, test.expectErr),
					"%q: not expecting %v wanted %v", test.in, err, test.expectErr,
				)
			} else {
				require.NoError(t, err)
			}
			if test.rval != ns {
				require.True(t, w3c.HasOTelValue())
				require.True(t, w3c.OTelValue().HasRValue())
				require.Equal(t, test.rval, w3c.OTelValue().RValue())
			} else {
				require.False(t, w3c.OTelValue().HasRValue(), "should have no r-value")
			}
			if test.sval != ns {
				require.True(t, w3c.HasOTelValue())
				require.True(t, w3c.OTelValue().HasSValue())
				require.Equal(t, test.sval, w3c.OTelValue().SValue())
			} else {
				require.False(t, w3c.OTelValue().HasSValue(), "should have no s-value")
			}
			if test.tval != ns {
				require.True(t, w3c.HasOTelValue())
				require.True(t, w3c.OTelValue().HasTValue())
				require.Equal(t, test.tval, w3c.OTelValue().TValue())
			} else {
				require.False(t, w3c.OTelValue().HasTValue(), "should have no t-value")
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
