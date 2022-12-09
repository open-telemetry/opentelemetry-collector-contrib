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

package ottlcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckValuesHomogeneous(t *testing.T) {
	tcs := []struct {
		name   string
		input  interface{}
		isHomo bool
	}{
		{
			name:   "non-slice values",
			input:  "value",
			isHomo: true,
		},
		{
			name:   "empty slice",
			input:  []interface{}{},
			isHomo: true,
		},
		{
			name:   "all string",
			input:  []string{"1", "2", "3"},
			isHomo: true,
		},
		{
			name:   "all boolean",
			input:  []bool{true, false},
			isHomo: true,
		},
		{
			name:   "all int64",
			input:  []int64{1, 2, 3},
			isHomo: true,
		},
		{
			name:   "all float64",
			input:  []float64{1.1, 2.2, 3.3},
			isHomo: true,
		},
		{
			name:   "all []byte",
			input:  [][]byte{[]byte("1"), []byte("2"), []byte("3")},
			isHomo: true,
		},
		{
			name:   "heterogeneous case 1",
			input:  []interface{}{[]byte("2"), 1, 1.2},
			isHomo: false,
		},
		{
			name:   "heterogeneous case 2",
			input:  []interface{}{1, 1.2},
			isHomo: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckValuesHomogeneous(tc.input)
			if tc.isHomo {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
