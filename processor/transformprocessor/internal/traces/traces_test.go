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
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

var (
	traceID  = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	traceID2 = [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	spanID   = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2  = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func Test_newPathGetSetter(t *testing.T) {
	refSpan, _, _ := createTelemetry()

	newAttrs := pdata.NewAttributeMap()
	newAttrs.UpsertString("hello", "world")

	newEvents := pdata.NewSpanEventSlice()
	newEvents.AppendEmpty().SetName("new event")

	newLinks := pdata.NewSpanLinkSlice()
	newLinks.AppendEmpty().SetSpanID(pdata.NewSpanID(spanID2))

	newStatus := pdata.NewSpanStatus()
	newStatus.SetMessage("new status")

	newArrStr := pdata.NewAttributeValueArray()
	newArrStr.SliceVal().AppendEmpty().SetStringVal("new")

	newArrBool := pdata.NewAttributeValueArray()
	newArrBool.SliceVal().AppendEmpty().SetBoolVal(false)

	newArrInt := pdata.NewAttributeValueArray()
	newArrInt.SliceVal().AppendEmpty().SetIntVal(20)

	newArrFloat := pdata.NewAttributeValueArray()
	newArrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)

	newArrBytes := pdata.NewAttributeValueArray()
	newArrBytes.SliceVal().AppendEmpty().SetBytesVal([]byte{9, 6, 4})

	tests := []struct {
		name     string
		path     []common.Field
		orig     interface{}
		new      interface{}
		modified func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource)
	}{
		{
			name: "trace_id",
			path: []common.Field{
				{
					Name: "trace_id",
				},
			},
			orig: pdata.NewTraceID(traceID),
			new:  hex.EncodeToString(traceID2[:]),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.SetTraceID(pdata.NewTraceID(traceID2))
			},
		},
		{
			name: "span_id",
			path: []common.Field{
				{
					Name: "span_id",
				},
			},
			orig: pdata.NewSpanID(spanID),
			new:  hex.EncodeToString(spanID2[:]),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.SetSpanID(pdata.NewSpanID(spanID2))
			},
		},
		{
			name: "trace_state",
			path: []common.Field{
				{
					Name: "trace_state",
				},
			},
			orig: pdata.TraceState("state"),
			new:  "newstate",
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.SetTraceState("newstate")
			},
		},
		{
			name: "parent_span_id",
			path: []common.Field{
				{
					Name: "parent_span_id",
				},
			},
			orig: pdata.NewSpanID(spanID2),
			new:  hex.EncodeToString(spanID[:]),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.SetParentSpanID(pdata.NewSpanID(spanID))
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
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
			orig: pdata.SpanKindServer,
			new:  int64(3),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.SetKind(pdata.SpanKindClient)
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
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.SetStartTimestamp(pdata.NewTimestampFromTime(time.UnixMilli(200)))
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
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.SetEndTimestamp(pdata.NewTimestampFromTime(time.UnixMilli(200)))
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
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().Clear()
				newAttrs.CopyTo(span.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("str"),
				},
			},
			orig: "val",
			new:  "newVal",
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().UpsertString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bool"),
				},
			},
			orig: true,
			new:  false,
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().UpsertBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("int"),
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().UpsertInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("double"),
				},
			},
			orig: float64(1.2),
			new:  float64(2.4),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().UpsertDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bytes"),
				},
			},
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().UpsertBytes("bytes", []byte{2, 3, 4})
			},
		},
		{
			name: "attributes array string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_str"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			new: []string{"new"},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().Upsert("arr_str", newArrStr)
			},
		},
		{
			name: "attributes array bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bool"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			new: []bool{false},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().Upsert("arr_bool", newArrBool)
			},
		},
		{
			name: "attributes array int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_int"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			new: []int64{20},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().Upsert("arr_int", newArrInt)
			},
		},
		{
			name: "attributes array float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_float"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			new: []float64{2.0},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Attributes().Upsert("arr_float", newArrFloat)
			},
		},
		{
			name: "attributes array bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bytes"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			new: [][]byte{{9, 6, 4}},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
			orig: uint32(10),
			new:  int64(20),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Events().RemoveIf(func(_ pdata.SpanEvent) bool {
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
			orig: uint32(20),
			new:  int64(30),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Links().RemoveIf(func(_ pdata.SpanLink) bool {
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
			orig: uint32(30),
			new:  int64(40),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
			orig: pdata.StatusCodeOk,
			new:  int64(pdata.StatusCodeError),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				span.Status().SetCode(pdata.StatusCodeError)
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
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
					MapKey: strp("str"),
				},
			},
			orig: "val",
			new:  "newVal",
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
					MapKey: strp("bool"),
				},
			},
			orig: true,
			new:  false,
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
					MapKey: strp("int"),
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
					MapKey: strp("double"),
				},
			},
			orig: float64(1.2),
			new:  float64(2.4),
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
					MapKey: strp("bytes"),
				},
			},
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				resource.Attributes().UpsertBytes("bytes", []byte{2, 3, 4})
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
					MapKey: strp("arr_str"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			new: []string{"new"},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
					MapKey: strp("arr_bool"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			new: []bool{false},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
					MapKey: strp("arr_int"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			new: []int64{20},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
					MapKey: strp("arr_float"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			new: []float64{2.0},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
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
					MapKey: strp("arr_bytes"),
				},
			},
			orig: func() pdata.AttributeValueSlice {
				val, _ := refSpan.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			new: [][]byte{{9, 6, 4}},
			modified: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) {
				resource.Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			span, il, resource := createTelemetry()

			got := accessor.get(spanTransformContext{
				span:     span,
				il:       il,
				resource: resource,
			})
			assert.Equal(t, tt.orig, got)

			accessor.set(spanTransformContext{
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

func createTelemetry() (pdata.Span, pdata.InstrumentationLibrary, pdata.Resource) {
	span := pdata.NewSpan()
	span.SetTraceID(pdata.NewTraceID(traceID))
	span.SetSpanID(pdata.NewSpanID(spanID))
	span.SetTraceState("state")
	span.SetParentSpanID(pdata.NewSpanID(spanID2))
	span.SetName("bear")
	span.SetKind(pdata.SpanKindServer)
	span.SetStartTimestamp(pdata.NewTimestampFromTime(time.UnixMilli(100)))
	span.SetEndTimestamp(pdata.NewTimestampFromTime(time.UnixMilli(500)))
	span.Attributes().UpsertString("str", "val")
	span.Attributes().UpsertBool("bool", true)
	span.Attributes().UpsertInt("int", 10)
	span.Attributes().UpsertDouble("double", 1.2)
	span.Attributes().UpsertBytes("bytes", []byte{1, 3, 2})

	arrStr := pdata.NewAttributeValueArray()
	arrStr.SliceVal().AppendEmpty().SetStringVal("one")
	arrStr.SliceVal().AppendEmpty().SetStringVal("two")
	span.Attributes().Upsert("arr_str", arrStr)

	arrBool := pdata.NewAttributeValueArray()
	arrBool.SliceVal().AppendEmpty().SetBoolVal(true)
	arrBool.SliceVal().AppendEmpty().SetBoolVal(false)
	span.Attributes().Upsert("arr_bool", arrBool)

	arrInt := pdata.NewAttributeValueArray()
	arrInt.SliceVal().AppendEmpty().SetIntVal(2)
	arrInt.SliceVal().AppendEmpty().SetIntVal(3)
	span.Attributes().Upsert("arr_int", arrInt)

	arrFloat := pdata.NewAttributeValueArray()
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(1.0)
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)
	span.Attributes().Upsert("arr_float", arrFloat)

	arrBytes := pdata.NewAttributeValueArray()
	arrBytes.SliceVal().AppendEmpty().SetBytesVal([]byte{1, 2, 3})
	arrBytes.SliceVal().AppendEmpty().SetBytesVal([]byte{2, 3, 4})
	span.Attributes().Upsert("arr_bytes", arrBytes)

	span.SetDroppedAttributesCount(10)

	span.Events().AppendEmpty().SetName("event")
	span.SetDroppedEventsCount(20)

	span.Links().AppendEmpty().SetTraceID(pdata.NewTraceID(traceID))
	span.SetDroppedLinksCount(30)

	span.Status().SetCode(pdata.StatusCodeOk)
	span.Status().SetMessage("good span")

	il := pdata.NewInstrumentationLibrary()
	il.SetName("library")
	il.SetVersion("version")

	resource := pdata.NewResource()
	span.Attributes().CopyTo(resource.Attributes())

	return span, il, resource
}
