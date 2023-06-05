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

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Int(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected interface{}
	}{
		{
			name:     "string",
			value:    "50",
			expected: int64(50),
		},
		{
			name:     "empty string",
			value:    "",
			expected: nil,
		},
		{
			name:     "not a number string",
			value:    "test",
			expected: nil,
		},
		{
			name:     "int64",
			value:    int64(333),
			expected: int64(333),
		},
		{
			name:     "float64",
			value:    float64(2.7),
			expected: int64(2),
		},
		{
			name:     "float64 without decimal",
			value:    float64(55),
			expected: int64(55),
		},
		{
			name:     "true",
			value:    true,
			expected: int64(1),
		},
		{
			name:     "false",
			value:    false,
			expected: int64(0),
		},
		{
			name:     "nil",
			value:    nil,
			expected: nil,
		},
		{
			name:     "some struct",
			value:    struct{}{},
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := intFunc[interface{}](&ottl.StandardGetSetter[interface{}]{
				Getter: func(context.Context, interface{}) (interface{}, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
