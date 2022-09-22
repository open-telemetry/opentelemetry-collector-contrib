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

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottl/ottltest"
)

func Test_set(t *testing.T) {
	input := pcommon.NewValueString("original name")

	target := &ottl.StandardGetSetter{
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			ctx.GetItem().(pcommon.Value).SetStringVal(val.(string))
		},
	}

	tests := []struct {
		name   string
		setter ottl.Setter
		getter ottl.Getter
		want   func(pcommon.Value)
	}{
		{
			name:   "set name",
			setter: target,
			getter: ottl.StandardGetSetter{
				Getter: func(ctx ottl.TransformContext) interface{} {
					return "new name"
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStringVal("new name")
			},
		},
		{
			name:   "set nil value",
			setter: target,
			getter: ottl.StandardGetSetter{
				Getter: func(ctx ottl.TransformContext) interface{} {
					return nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStringVal("original name")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueString(input.StringVal())

			ctx := ottltest.TestTransformContext{
				Item: scenarioValue,
			}

			exprFunc, _ := Set(tt.setter, tt.getter)
			exprFunc(ctx)

			expected := pcommon.NewValueString("")
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_set_get_nil(t *testing.T) {
	ctx := ottltest.TestTransformContext{
		Item: nil,
	}

	setter := &ottl.StandardGetSetter{
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	getter := &ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem()
		},
	}

	exprFunc, _ := Set(setter, getter)
	exprFunc(ctx)
}
