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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_isMatch(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.Getter
		pattern  string
		expected bool
	}{
		{
			name: "replace match true",
			target: &ottl.StandardGetSetter{
				Getter: func(ctx ottl.TransformContext) interface{} {
					return "hello world"
				},
			},
			pattern:  "hello.*",
			expected: true,
		},
		{
			name: "replace match false",
			target: &ottl.StandardGetSetter{
				Getter: func(ctx ottl.TransformContext) interface{} {
					return "goodbye world"
				},
			},
			pattern:  "hello.*",
			expected: false,
		},
		{
			name: "replace match complex",
			target: &ottl.StandardGetSetter{
				Getter: func(ctx ottl.TransformContext) interface{} {
					return "-12.001"
				},
			},
			pattern:  "[-+]?\\d*\\.\\d+([eE][-+]?\\d+)?",
			expected: true,
		},
		{
			name: "target not a string",
			target: &ottl.StandardGetSetter{
				Getter: func(ctx ottl.TransformContext) interface{} {
					return 1
				},
			},
			pattern:  "doesnt matter will be false",
			expected: false,
		},
		{
			name: "target nil",
			target: &ottl.StandardGetSetter{
				Getter: func(ctx ottl.TransformContext) interface{} {
					return nil
				},
			},
			pattern:  "doesnt matter will be false",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ottltest.TestTransformContext{}

			exprFunc, _ := IsMatch(tt.target, tt.pattern)
			actual := exprFunc(ctx)

			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_isMatch_validation(t *testing.T) {
	target := &ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return "anything"
		},
	}
	_, err := IsMatch(target, "\\K")
	assert.Error(t, err)
}
