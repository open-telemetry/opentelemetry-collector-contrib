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

package traces

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
)

var (
	traceID  = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	traceID2 = [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	spanID   = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2  = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func Test_newPathGetSetter(t *testing.T) {
	refSpan, _, _ := createTelemetry()

	newAttrs := pcommon.NewMap()
	newAttrs.UpsertString("hello", "world")

	newEvents := ptrace.NewSpanEventSlice()
	newEvents.AppendEmpty().SetName("new event")

	newLinks := ptrace.NewSpanLinkSlice()
	newLinks.AppendEmpty().SetSpanID(pcommon.NewSpanID(spanID2))

	newStatus := ptrace.NewSpanStatus()
	newStatus.SetMessage("new status")

	newArrStr := pcommon.NewValueSlice()
	newArrStr.SliceVal().AppendEmpty().SetStringVal("new")

	newArrBool := pcommon.NewValueSlice()
	newArrBool.SliceVal().AppendEmpty().SetBoolVal(false)

	newArrInt := pcommon.NewValueSlice()
	newArrInt.SliceVal().AppendEmpty().SetIntVal(20)

	newArrFloat := pcommon.NewValueSlice()
	newArrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)

	newArrBytes := pcommon.NewValueSlice()
	newArrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{9, 6, 4}))

	tests := []struct {
		name     string
		path     []common.Field
		orig     interface{}
		new      interface{}
		modified func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource)
	}{
		{
			name: "trace_id",
			path: []common.Field{
				{
					Name: "trace_id",
				},
			},
			orig: pcommon.NewTraceID(traceID),
			new:  pcommon.NewTraceID(traceID2),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetTraceID(pcommon.NewTraceID(traceID2))
			},
		},
		{
			name: "span_id",
			path: []common.Field{
				{
					Name: "span_id",
				},
			},
			orig: pcommon.NewSpanID(spanID),
			new:  pcommon.NewSpanID(spanID2),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetSpanID(pcommon.NewSpanID(spanID2))
			},
		},
		{
			name: "trace_state",
			path: []common.Field{
				{
					Name: "trace_state",
				},
			},
			orig: "key1=val1,key2=val2",
			new:  "key=newVal",
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetTraceState("key=newVal")
			},
		},
		{
			name: "trace_state key",
			path: []common.Field{
				{
					Name:   "trace_state",
					MapKey: testhelper.Strp("key1"),
				},
			},
			orig: "val1",
			new:  "newVal",
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetTraceState("key1=newVal,key2=val2")
			},
		},
		{
			name: "parent_span_id",
			path: []common.Field{
				{
					Name: "parent_span_id",
				},
			},
			orig: pcommon.NewSpanID(spanID2),
			new:  pcommon.NewSpanID(spanID),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetParentSpanID(pcommon.NewSpanID(spanID))
			},
		},
		{
			name: "name",
			path: []common.Field{
				{
					Name: "name",
				},
			},
			orig: "bear",
			new:  "cat",
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetName("cat")
			},
		},
		{
			name: "kind",
			path: []common.Field{
				{
					Name: "kind",
				},
			},
			orig: ptrace.SpanKindServer,
			new:  int64(3),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetKind(ptrace.SpanKindClient)
			},
		},
		{
			name: "start_time_unix_nano",
			path: []common.Field{
				{
					Name: "start_time_unix_nano",
				},
			},
			orig: int64(100_000_000),
			new:  int64(200_000_000),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "end_time_unix_nano",
			path: []common.Field{
				{
					Name: "end_time_unix_nano",
				},
			},
			orig: int64(500_000_000),
			new:  int64(200_000_000),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "attributes",
			path: []common.Field{
				{
					Name: "attributes",
				},
			},
			orig: refSpan.Attributes(),
			new:  newAttrs,
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().Clear()
				newAttrs.CopyTo(span.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("str"),
				},
			},
			orig: "val",
			new:  "newVal",
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().UpsertString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("bool"),
				},
			},
			orig: true,
			new:  false,
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().UpsertBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("int"),
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().UpsertInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("double"),
				},
			},
			orig: float64(1.2),
			new:  float64(2.4),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().UpsertDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("bytes"),
				},
			},
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
			},
		},
		{
			name: "attributes array string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			new: []string{"new"},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().Upsert("arr_str", newArrStr)
			},
		},
		{
			name: "attributes array bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			new: []bool{false},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().Upsert("arr_bool", newArrBool)
			},
		},
		{
			name: "attributes array int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			new: []int64{20},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().Upsert("arr_int", newArrInt)
			},
		},
		{
			name: "attributes array float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			new: []float64{2.0},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().Upsert("arr_float", newArrFloat)
			},
		},
		{
			name: "attributes array bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			new: [][]byte{{9, 6, 4}},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
		{
			name: "dropped_attributes_count",
			path: []common.Field{
				{
					Name: "dropped_attributes_count",
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "events",
			path: []common.Field{
				{
					Name: "events",
				},
			},
			orig: refSpan.Events(),
			new:  newEvents,
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Events().RemoveIf(func(_ ptrace.SpanEvent) bool {
					return true
				})
				newEvents.CopyTo(span.Events())
			},
		},
		{
			name: "dropped_events_count",
			path: []common.Field{
				{
					Name: "dropped_events_count",
				},
			},
			orig: int64(20),
			new:  int64(30),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetDroppedEventsCount(30)
			},
		},
		{
			name: "links",
			path: []common.Field{
				{
					Name: "links",
				},
			},
			orig: refSpan.Links(),
			new:  newLinks,
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Links().RemoveIf(func(_ ptrace.SpanLink) bool {
					return true
				})
				newLinks.CopyTo(span.Links())
			},
		},
		{
			name: "dropped_links_count",
			path: []common.Field{
				{
					Name: "dropped_links_count",
				},
			},
			orig: int64(30),
			new:  int64(40),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.SetDroppedLinksCount(40)
			},
		},
		{
			name: "status",
			path: []common.Field{
				{
					Name: "status",
				},
			},
			orig: refSpan.Status(),
			new:  newStatus,
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				newStatus.CopyTo(span.Status())
			},
		},
		{
			name: "status code",
			path: []common.Field{
				{
					Name: "status",
				},
				{
					Name: "code",
				},
			},
			orig: ptrace.StatusCodeOk,
			new:  int64(ptrace.StatusCodeError),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Status().SetCode(ptrace.StatusCodeError)
			},
		},
		{
			name: "status message",
			path: []common.Field{
				{
					Name: "status",
				},
				{
					Name: "message",
				},
			},
			orig: "good span",
			new:  "bad span",
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				span.Status().SetMessage("bad span")
			},
		},
		{
			name: "resource attributes",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name: "attributes",
				},
			},
			orig: refSpan.Attributes(),
			new:  newAttrs,
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Clear()
				newAttrs.CopyTo(resource.Attributes())
			},
		},
		{
			name: "resource attributes string",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("str"),
				},
			},
			orig: "val",
			new:  "newVal",
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertString("str", "newVal")
			},
		},
		{
			name: "resource attributes bool",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("bool"),
				},
			},
			orig: true,
			new:  false,
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertBool("bool", false)
			},
		},
		{
			name: "resource attributes int",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("int"),
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertInt("int", 20)
			},
		},
		{
			name: "resource attributes float",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("double"),
				},
			},
			orig: float64(1.2),
			new:  float64(2.4),
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertDouble("double", 2.4)
			},
		},
		{
			name: "resource attributes bytes",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("bytes"),
				},
			},
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
			},
		},
		{
			name: "resource attributes array string",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			new: []string{"new"},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_str", newArrStr)
			},
		},
		{
			name: "resource attributes array bool",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			new: []bool{false},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_bool", newArrBool)
			},
		},
		{
			name: "resource attributes array int",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			new: []int64{20},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_int", newArrInt)
			},
		},
		{
			name: "resource attributes array float",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			new: []float64{2.0},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_float", newArrFloat)
			},
		},
		{
			name: "resource attributes array bytes",
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			new: [][]byte{{9, 6, 4}},
			modified: func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			span, il, resource := createTelemetry()

			got := accessor.Get(spanTransformContext{
				span:     span,
				il:       il,
				resource: resource,
			})
			assert.Equal(t, tt.orig, got)

			accessor.Set(spanTransformContext{
				span:     span,
				il:       il,
				resource: resource,
			}, tt.new)

			exSpan, exIl, exRes := createTelemetry()
			tt.modified(exSpan, exIl, exRes)

			assert.Equal(t, exSpan, span)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exRes, resource)
		})
	}
}

func createTelemetry() (ptrace.Span, pcommon.InstrumentationScope, pcommon.Resource) {
	span := ptrace.NewSpan()
	span.SetTraceID(pcommon.NewTraceID(traceID))
	span.SetSpanID(pcommon.NewSpanID(spanID))
	span.SetTraceState("key1=val1,key2=val2")
	span.SetParentSpanID(pcommon.NewSpanID(spanID2))
	span.SetName("bear")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	span.Attributes().UpsertString("str", "val")
	span.Attributes().UpsertBool("bool", true)
	span.Attributes().UpsertInt("int", 10)
	span.Attributes().UpsertDouble("double", 1.2)
	span.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{1, 3, 2}))

	arrStr := pcommon.NewValueSlice()
	arrStr.SliceVal().AppendEmpty().SetStringVal("one")
	arrStr.SliceVal().AppendEmpty().SetStringVal("two")
	span.Attributes().Upsert("arr_str", arrStr)

	arrBool := pcommon.NewValueSlice()
	arrBool.SliceVal().AppendEmpty().SetBoolVal(true)
	arrBool.SliceVal().AppendEmpty().SetBoolVal(false)
	span.Attributes().Upsert("arr_bool", arrBool)

	arrInt := pcommon.NewValueSlice()
	arrInt.SliceVal().AppendEmpty().SetIntVal(2)
	arrInt.SliceVal().AppendEmpty().SetIntVal(3)
	span.Attributes().Upsert("arr_int", arrInt)

	arrFloat := pcommon.NewValueSlice()
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(1.0)
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)
	span.Attributes().Upsert("arr_float", arrFloat)

	arrBytes := pcommon.NewValueSlice()
	arrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{1, 2, 3}))
	arrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
	span.Attributes().Upsert("arr_bytes", arrBytes)

	span.SetDroppedAttributesCount(10)

	span.Events().AppendEmpty().SetName("event")
	span.SetDroppedEventsCount(20)

	span.Links().AppendEmpty().SetTraceID(pcommon.NewTraceID(traceID))
	span.SetDroppedLinksCount(30)

	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Status().SetMessage("good span")

	il := pcommon.NewInstrumentationScope()
	il.SetName("library")
	il.SetVersion("version")

	resource := pcommon.NewResource()
	span.Attributes().CopyTo(resource.Attributes())

	return span, il, resource
}
