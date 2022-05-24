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

func Test_limit(t *testing.T) {
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
		limit  int64
		want   func(ptrace.Span)
	}{
		{
			name:   "limit to 1",
			target: target,
			limit:  int64(1),
			want: func(span ptrace.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(span.Attributes())
			},
		},
		{
			name:   "limit to zero",
			target: target,
			limit:  int64(0),
			want: func(span ptrace.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.CopyTo(span.Attributes())
			},
		},
		{
			name:   "limit nothing",
			target: target,
			limit:  int64(100),
			want: func(span ptrace.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(span.Attributes())
			},
		},
		{
			name:   "limit exact",
			target: target,
			limit:  int64(3),
			want: func(span ptrace.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
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

			exprFunc, _ := limit(tt.target, tt.limit)
			exprFunc(ctx)

			expected := ptrace.NewSpan()
			tt.want(expected)

			assert.Equal(t, expected, span)
		})
	}
}

func Test_limit_validation(t *testing.T) {
	tests := []struct {
		name   string
		target GetSetter
		limit  int64
	}{
		{
			name:   "limit less than zero",
			target: &testGetSetter{},
			limit:  int64(-1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := limit(tt.target, tt.limit)
			assert.Error(t, err, "invalid limit for limit function, -1 cannot be negative")
		})
	}
}
