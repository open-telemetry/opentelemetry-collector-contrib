// Copyright  The OpenTelemetry Authors
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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
)

func Test_isMatch(t *testing.T) {
	tests := []struct {
		name     string
		target   Getter
		pattern  string
		expected bool
	}{
		{
			name: "replace match true",
			target: &testGetSetter{
				getter: func(ctx TransformContext) interface{} {
					return "hello world"
				},
			},
			pattern:  "hello.*",
			expected: true,
		},
		{
			name: "replace match false",
			target: &testGetSetter{
				getter: func(ctx TransformContext) interface{} {
					return "goodbye world"
				},
			},
			pattern:  "hello.*",
			expected: false,
		},
		{
			name: "replace match complex",
			target: &testGetSetter{
				getter: func(ctx TransformContext) interface{} {
					return "-12.001"
				},
			},
			pattern:  "[-+]?\\d*\\.\\d+([eE][-+]?\\d+)?",
			expected: true,
		},
		{
			name: "target not a string",
			target: &testGetSetter{
				getter: func(ctx TransformContext) interface{} {
					return 1
				},
			},
			pattern:  "doesnt matter will be false",
			expected: false,
		},
		{
			name: "target nil",
			target: &testGetSetter{
				getter: func(ctx TransformContext) interface{} {
					return nil
				},
			},
			pattern:  "doesnt matter will be false",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testhelper.TestTransformContext{}

			exprFunc, _ := isMatch(tt.target, tt.pattern)
			actual := exprFunc(ctx)

			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_isMatch_validation(t *testing.T) {
	target := &testGetSetter{
		getter: func(ctx TransformContext) interface{} {
			return "anything"
		},
	}
	_, err := isMatch(target, "\\K")
	assert.Error(t, err)
}
