// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalText(t *testing.T) {
	tests := []struct {
		str  []string
		unit Unit
		err  bool
	}{
		{
			str:  []string{"ms", "Ms", "MS"},
			unit: Milliseconds,
		},
		{
			str:  []string{"s", "S"},
			unit: Seconds,
		},
		{
			str: []string{"h", "H"},
			err: true,
		},
		{
			str: []string{""},
			err: true,
		},
	}

	for _, tt := range tests {
		for _, str := range tt.str {
			t.Run(str, func(t *testing.T) {
				var u Unit
				err := u.UnmarshalText([]byte(str))
				if tt.err {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.unit, u)
				}
			})
		}
	}
}

func TestUnmarshalTextNilUnit(t *testing.T) {
	lvl := (*Unit)(nil)
	assert.Error(t, lvl.UnmarshalText([]byte(MillisecondsStr)))
}

func TestUnitStringMarshal(t *testing.T) {
	tests := []struct {
		str  string
		unit Unit
		err  bool
	}{
		{
			str:  SecondsStr,
			unit: Seconds,
		},
		{
			str:  MillisecondsStr,
			unit: Milliseconds,
		},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			assert.Equal(t, tt.str, tt.unit.String())
			got, err := tt.unit.MarshalText()
			assert.NoError(t, err)
			assert.Equal(t, tt.str, string(got))
		})
	}
}
