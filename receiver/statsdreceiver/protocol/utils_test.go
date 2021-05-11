// Copyright 2020, OpenTelemetry Authors
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

package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Contains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name: "contain 1",
			slice: []string{
				"m",
				"g",
			},
			element:  "m",
			expected: true,
		},
		{
			name: "contain 2",
			slice: []string{
				"m",
				"g",
			},
			element:  "g",
			expected: true,
		},
		{
			name: "does not contain",
			slice: []string{
				"m",
				"g",
			},
			element:  "t",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answer := Contains(tt.slice, tt.element)
			assert.Equal(t, tt.expected, answer)
		})
	}
}
