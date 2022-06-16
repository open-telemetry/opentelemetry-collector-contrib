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

func Test_newConditionEvaluator(t *testing.T) {
	tests := []struct {
		name string
		cond *Condition
		item interface{}
	}{
		{
			name: "literals match",
			cond: &Condition{
				Left: Value{
					String: testhelper.Strp("hello"),
				},
				Right: Value{
					String: testhelper.Strp("hello"),
				},
				Op: "==",
			},
		},
		{
			name: "literals don't match",
			cond: &Condition{
				Left: Value{
					String: testhelper.Strp("hello"),
				},
				Right: Value{
					String: testhelper.Strp("goodbye"),
				},
				Op: "!=",
			},
		},
		{
			name: "path expression matches",
			cond: &Condition{
				Left: Value{
					Path: &Path{
						Fields: []Field{
							{
								Name: "name",
							},
						},
					},
				},
				Right: Value{
					String: testhelper.Strp("bear"),
				},
				Op: "==",
			},
			item: "bear",
		},
		{
			name: "path expression not matches",
			cond: &Condition{
				Left: Value{
					Path: &Path{
						Fields: []Field{
							{
								Name: "name",
							},
						},
					},
				},
				Right: Value{
					String: testhelper.Strp("cat"),
				},
				Op: "!=",
			},
			item: "bear",
		},
		{
			name: "no condition",
			cond: nil,
		},
		{
			name: "like match",
			cond: &Condition{
				Left: Value{
					String: testhelper.Strp("hello world"),
				},
				Right: Value{
					String: testhelper.Strp("hello*"),
				},
				Op: "like",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := newConditionEvaluator(tt.cond, DefaultFunctions(), testParsePath)
			assert.NoError(t, err)
			assert.True(t, evaluate(testhelper.TestTransformContext{
				Item: tt.item,
			}))
		})
	}
}

func Test_newConditionEvaluator_Invalid(t *testing.T) {
	tests := []struct {
		name string
		cond *Condition
	}{
		{
			name: "invalid operation",
			cond: &Condition{
				Left: Value{
					String: testhelper.Strp("hello"),
				},
				Right: Value{
					String: testhelper.Strp("goodbye"),
				},
				Op: "<>",
			},
		},
		{
			name: "invalid literal on right side of like",
			cond: &Condition{
				Left: Value{
					String: testhelper.Strp("hello"),
				},
				Right: Value{
					Int: testhelper.Intp(1),
				},
				Op: "like",
			},
		},
		{
			name: "invalid path on right side of like",
			cond: &Condition{
				Left: Value{
					String: testhelper.Strp("hello"),
				},
				Right: Value{
					Path: &Path{
						Fields: []Field{
							{
								Name: "name",
							},
						},
					},
				},
				Op: "like",
			},
		},
		{
			name: "invalid function on right side of like",
			cond: &Condition{
				Left: Value{
					String: testhelper.Strp("hello"),
				},
				Right: Value{
					Invocation: &Invocation{
						Function: "functionThatReturnsAPattern",
					},
				},
				Op: "like",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			functions := DefaultFunctions()
			functions["functionThatReturnsAPattern"] = functionThatReturnsAPattern
			_, err := newConditionEvaluator(tt.cond, functions, testParsePath)
			assert.Error(t, err)
		})
	}
}

func functionThatReturnsAPattern() (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "hello*"
	}, nil
}

func Test_newConditionEvaluator_Panic(t *testing.T) {
	tests := []struct {
		name string
		cond *Condition
	}{
		{
			name: "panic when not string",
			cond: &Condition{
				Left: Value{
					Int: testhelper.Intp(1),
				},
				Right: Value{
					String: testhelper.Strp("hello*"),
				},
				Op: "like",
			},
		},
		{
			name: "panic when function returns not string",
			cond: &Condition{
				Left: Value{
					Invocation: &Invocation{
						Function: "functionThatReturnsAFloat",
					},
				},
				Right: Value{
					String: testhelper.Strp("hello*"),
				},
				Op: "like",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			functions := DefaultFunctions()
			functions["functionThatReturnsAFloat"] = functionThatReturnsAFloat
			assert.Panics(t, func() {
				evaluate, _ := newConditionEvaluator(tt.cond, functions, testParsePath)
				evaluate(testhelper.TestTransformContext{
					Item: nil,
				})
			})
		})
	}
}

func functionThatReturnsAFloat() (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return 1.0
	}, nil
}
