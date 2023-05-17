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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseW3CTraceState(t *testing.T) {
	type testCase struct {
		in        string
		otval     string
		expectErr error
	}
	const notset = ""
	for _, test := range []testCase{
		// correct cases
		{"ot=t:1", "t:1", nil},
		{"ot=t:100", "t:100", nil},
	} {
		t.Run(testName(test.in), func(t *testing.T) {
			wts, err := w3cSyntax.parse(test.in)

			if test.expectErr != nil {
				require.True(t, errors.Is(err, test.expectErr), "%q: not expecting %v wanted %v", test.in, err, test.expectErr)
			} else {
				require.NoError(t, err)
			}
			if test.otval != notset {
				require.True(t, wts.hasOTelValue())
				require.Equal(t, "ot="+test.otval, wts.otelString)
			} else {

				require.False(t, wts.hasOTelValue(), "should have no otel value")
			}

			// on success w/o t-value, serialize() should not modify
			if !wts.hasOTelValue() && test.expectErr == nil {
				require.Equal(t, test.in, wts.serialize())
			}
		})
	}
}
