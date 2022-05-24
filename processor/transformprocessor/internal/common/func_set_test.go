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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type ExprSetter struct {
	Setter func(ctx TransformContext, val interface{})
}

func (path ExprSetter) Set(ctx TransformContext, val interface{}) {
	path.Setter(ctx, val)
}

func Test_set(t *testing.T) {
	input := ptrace.NewSpan()
	input.SetName("bear")

	target := &testGetSetter{
		setter: func(ctx TransformContext, val interface{}) {
			ctx.GetItem().(ptrace.Span).SetName(val.(string))
		},
	}

	tests := []struct {
		name   string
		setter Setter
		getter Getter
		want   func(ptrace.Span)
	}{
		{
			name:   "set name",
			setter: target,
			getter: literal{value: "new name"},
			want: func(span ptrace.Span) {
				input.CopyTo(span)
				span.SetName("new name")
			},
		},
		{
			name:   "set nil value",
			setter: target,
			getter: literal{value: nil},
			want: func(span ptrace.Span) {
				input.CopyTo(span)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			input.CopyTo(span)

			ctx := testTransformContext{
				span:     span,
				il:       pcommon.NewInstrumentationScope(),
				resource: pcommon.NewResource(),
			}

			exprFunc, _ := set(tt.setter, tt.getter)
			exprFunc(ctx)

			expected := ptrace.NewSpan()
			tt.want(expected)

			assert.Equal(t, expected, span)
		})
	}
}
