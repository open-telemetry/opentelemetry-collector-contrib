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

package tqlotel

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
)

func Test_split(t *testing.T) {
	target := &tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(pcommon.Value).StringVal()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			switch tv := val.(type) {
			case string:
				ctx.GetItem().(pcommon.Value).SetStringVal(tv)
			case []string:
				newSlice := pcommon.NewValueSlice()
				for i := 0; i < len(tv); i++ {
					newSlice.SliceVal().AppendEmpty().SetStringVal(tv[i])
				}
				newSlice.CopyTo(ctx.GetItem().(pcommon.Value))
			default:
				fmt.Printf("<Invalid value type %T>", tv)
			}
		},
	}

	tests := []struct {
		name      string
		target    tql.GetSetter
		value     string
		delimiter string
		want      func(pcommon.Value)
	}{
		{
			name:      "split",
			target:    target,
			value:     "A|B|C",
			delimiter: "|",
			want: func(expectedValue pcommon.Value) {
				expectedValue.SliceVal().AppendEmpty().SetStringVal("A")
				expectedValue.SliceVal().AppendEmpty().SetStringVal("B")
				expectedValue.SliceVal().AppendEmpty().SetStringVal("C")
			},
		},
		{
			name:      "split empty string",
			target:    target,
			value:     "",
			delimiter: "|",
			want: func(expectedValue pcommon.Value) {
				expectedValue.SliceVal().AppendEmpty().SetStringVal("")
			},
		},
		{
			name:      "delimiter empty",
			target:    target,
			value:     "A|B|C",
			delimiter: "",
			want: func(expectedValue pcommon.Value) {
				expectedValue.SliceVal().AppendEmpty().SetStringVal("A")
				expectedValue.SliceVal().AppendEmpty().SetStringVal("|")
				expectedValue.SliceVal().AppendEmpty().SetStringVal("B")
				expectedValue.SliceVal().AppendEmpty().SetStringVal("|")
				expectedValue.SliceVal().AppendEmpty().SetStringVal("C")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueString(tt.value)
			ctx := tqltest.TestTransformContext{
				Item: scenarioValue,
			}

			exprFunc, _ := Split(tt.target, tt.delimiter)
			exprFunc(ctx)

			expected := pcommon.NewValueSlice()
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}
