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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func Test_newConditionEvaluator(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetName("bear")
	tests := []struct {
		name     string
		cond     *Condition
		matching ptrace.Span
	}{
		{
			name: "literals match",
			cond: &Condition{
				Left: Value{
					String: strp("hello"),
				},
				Right: Value{
					String: strp("hello"),
				},
				Op: "==",
			},
			matching: span,
		},
		{
			name: "literals don't match",
			cond: &Condition{
				Left: Value{
					String: strp("hello"),
				},
				Right: Value{
					String: strp("goodbye"),
				},
				Op: "!=",
			},
			matching: span,
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
					String: strp("bear"),
				},
				Op: "==",
			},
			matching: span,
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
					String: strp("cat"),
				},
				Op: "!=",
			},
			matching: span,
		},
		{
			name:     "no condition",
			cond:     nil,
			matching: span,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := newConditionEvaluator(tt.cond, DefaultFunctions(), testParsePath)
			assert.NoError(t, err)
			assert.True(t, evaluate(testTransformContext{
				span:     tt.matching,
				il:       pcommon.NewInstrumentationScope(),
				resource: pcommon.NewResource(),
			}))
		})
	}

	t.Run("invalid", func(t *testing.T) {
		_, err := newConditionEvaluator(&Condition{
			Left: Value{
				String: strp("bear"),
			},
			Op: "<>",
			Right: Value{
				String: strp("cat"),
			},
		}, DefaultFunctions(), testParsePath)
		assert.Error(t, err)
	})
}

// Small copy of traces data model for use in common tests

type testTransformContext struct {
	span     ptrace.Span
	il       pcommon.InstrumentationScope
	resource pcommon.Resource
}

func (ctx testTransformContext) GetItem() interface{} {
	return ctx.span
}

func (ctx testTransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.il
}

func (ctx testTransformContext) GetResource() pcommon.Resource {
	return ctx.resource
}

func (ctx testTransformContext) GetDescriptor() interface{} {
	return nil
}

// pathGetSetter is a getSetter which has been resolved using a path expression provided by a user.
type testGetSetter struct {
	getter ExprFunc
	setter func(ctx TransformContext, val interface{})
}

func (path testGetSetter) Get(ctx TransformContext) interface{} {
	return path.getter(ctx)
}

func (path testGetSetter) Set(ctx TransformContext, val interface{}) {
	path.setter(ctx, val)
}

func testParsePath(val *Path) (GetSetter, error) {
	if val != nil && len(val.Fields) > 0 && val.Fields[0].Name == "name" {
		return &testGetSetter{
			getter: func(ctx TransformContext) interface{} {
				return ctx.GetItem().(ptrace.Span).Name()
			},
			setter: func(ctx TransformContext, val interface{}) {
				ctx.GetItem().(ptrace.Span).SetName(val.(string))
			},
		}, nil
	}
	return nil, fmt.Errorf("bad path %v", val)
}
