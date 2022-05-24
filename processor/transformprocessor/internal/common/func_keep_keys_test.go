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

func Test_keep_keys(t *testing.T) {
	input := ptrace.NewSpan()
	input.SetName("bear")
	attrs := pcommon.NewMap()
	attrs.InsertString("test", "hello world")
	attrs.InsertInt("test2", 3)
	attrs.InsertBool("test3", true)
	attrs.CopyTo(input.Attributes())

	target := &testGetSetter{
		getter: func(ctx TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Attributes()
		},
		setter: func(ctx TransformContext, val interface{}) {
			ctx.GetItem().(ptrace.Span).Attributes().Clear()
			val.(pcommon.Map).CopyTo(ctx.GetItem().(ptrace.Span).Attributes())
		},
	}

	tests := []struct {
		name   string
		target GetSetter
		keys   []string
		want   func(ptrace.Span)
	}{
		{
			name:   "keep one",
			target: target,
			keys:   []string{"test"},
			want: func(span ptrace.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(span.Attributes())
			},
		},
		{
			name:   "keep two",
			target: target,
			keys:   []string{"test", "test2"},
			want: func(span ptrace.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.CopyTo(span.Attributes())
			},
		},
		{
			name:   "keep none",
			target: target,
			keys:   []string{},
			want: func(span ptrace.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.CopyTo(span.Attributes())
			},
		},
		{
			name:   "no match",
			target: target,
			keys:   []string{"no match"},
			want: func(span ptrace.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.CopyTo(span.Attributes())
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

			exprFunc, _ := keepKeys(tt.target, tt.keys)
			exprFunc(ctx)

			expected := ptrace.NewSpan()
			tt.want(expected)

			assert.Equal(t, expected, span)
		})
	}
}
