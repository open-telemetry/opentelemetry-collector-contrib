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
)

func Test_convertCase(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.Getter[interface{}]
		toCase   string
		expected interface{}
	}{
		// snake case
		{
			name: "snake simple convert",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "simpleString", nil
				},
			},
			toCase:   "snake",
			expected: "simple_string",
		},
		{
			name: "snake noop already snake case",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "simple_string", nil
				},
			},
			toCase:   "snake",
			expected: "simple_string",
		},
		{
			name: "snake multiple uppercase",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "CPUUtilizationMetric", nil
				},
			},
			toCase:   "snake",
			expected: "cpu_utilization_metric",
		},
		{
			name: "snake hyphens",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "simple-string", nil
				},
			},
			toCase:   "snake",
			expected: "simple_string",
		},
		{
			name: "snake nil",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			toCase:   "snake",
			expected: nil,
		},
		{
			name: "snake empty string",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			toCase:   "snake",
			expected: "",
		},
		// upper case
		{
			name: "upper simple",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "simple", nil
				},
			},
			toCase:   "upper",
			expected: "SIMPLE",
		},
		{
			name: "upper complex",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "complex_SET-of.WORDS1234", nil
				},
			},
			toCase:   "upper",
			expected: "COMPLEX_SET-OF.WORDS1234",
		},
		{
			name: "upper empty string",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			toCase:   "upper",
			expected: "",
		},
		// lower case
		{
			name: "lower simple",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "SIMPLE", nil
				},
			},
			toCase:   "lower",
			expected: "simple",
		},
		{
			name: "lower complex",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "complex_SET-of.WORDS1234", nil
				},
			},
			toCase:   "lower",
			expected: "complex_set-of.words1234",
		},
		{
			name: "lower empty string",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			toCase:   "lower",
			expected: "",
		},
		// no case
		{
			name: "no case defined",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "simpleName", nil
				},
			},
			toCase:   "",
			expected: "simpleName",
		},
		// unconfigured case
		{
			name: "unconfigured case",
			target: &ottl.StandardGetSetter[interface{}]{
				Getter: func(ctx interface{}) (interface{}, error) {
					return "simpleName", nil
				},
			},
			toCase:   "unconfigured",
			expected: "simpleName",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := ConvertCase(tt.target, tt.toCase)
			assert.NoError(t, err)
			result, err := exprFunc(nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
