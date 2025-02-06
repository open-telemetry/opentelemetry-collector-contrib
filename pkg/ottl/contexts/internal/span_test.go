// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

var (
	traceID  = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	traceID2 = [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	spanID   = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2  = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func TestSpanPathGetSetter(t *testing.T) {
	refSpan := createSpan()

	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")

	newEvents := ptrace.NewSpanEventSlice()
	newEvents.AppendEmpty().SetName("new event")

	newLinks := ptrace.NewSpanLinkSlice()
	newLinks.AppendEmpty().SetSpanID(spanID2)

	newStatus := ptrace.NewStatus()
	newStatus.SetMessage("new status")

	tests := []struct {
		name     string
		path     ottl.Path[*spanContext]
		orig     any
		newVal   any
		modified func(span ptrace.Span)
	}{
		{
			name: "trace_id",
			path: &TestPath[*spanContext]{
				N: "trace_id",
			},
			orig:   pcommon.TraceID(traceID),
			newVal: pcommon.TraceID(traceID2),
			modified: func(span ptrace.Span) {
				span.SetTraceID(traceID2)
			},
		},
		{
			name: "span_id",
			path: &TestPath[*spanContext]{
				N: "span_id",
			},
			orig:   pcommon.SpanID(spanID),
			newVal: pcommon.SpanID(spanID2),
			modified: func(span ptrace.Span) {
				span.SetSpanID(spanID2)
			},
		},
		{
			name: "trace_id string",
			path: &TestPath[*spanContext]{
				N: "trace_id",
				NextPath: &TestPath[*spanContext]{
					N: "string",
				},
			},
			orig:   hex.EncodeToString(traceID[:]),
			newVal: hex.EncodeToString(traceID2[:]),
			modified: func(span ptrace.Span) {
				span.SetTraceID(traceID2)
			},
		},
		{
			name: "span_id string",
			path: &TestPath[*spanContext]{
				N: "span_id",
				NextPath: &TestPath[*spanContext]{
					N: "string",
				},
			},
			orig:   hex.EncodeToString(spanID[:]),
			newVal: hex.EncodeToString(spanID2[:]),
			modified: func(span ptrace.Span) {
				span.SetSpanID(spanID2)
			},
		},
		{
			name: "trace_state",
			path: &TestPath[*spanContext]{
				N: "trace_state",
			},
			orig:   "key1=val1,key2=val2",
			newVal: "key=newVal",
			modified: func(span ptrace.Span) {
				span.TraceState().FromRaw("key=newVal")
			},
		},
		{
			name: "trace_state key",
			path: &TestPath[*spanContext]{
				N: "trace_state",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("key1"),
					},
				},
			},
			orig:   "val1",
			newVal: "newVal",
			modified: func(span ptrace.Span) {
				span.TraceState().FromRaw("key1=newVal,key2=val2")
			},
		},
		{
			name: "parent_span_id",
			path: &TestPath[*spanContext]{
				N: "parent_span_id",
			},
			orig:   pcommon.SpanID(spanID2),
			newVal: pcommon.SpanID(spanID),
			modified: func(span ptrace.Span) {
				span.SetParentSpanID(spanID)
			},
		},
		{
			name: "parent_span_id string",
			path: &TestPath[*spanContext]{
				N: "parent_span_id",
				NextPath: &TestPath[*spanContext]{
					N: "string",
				},
			},
			orig:   hex.EncodeToString(spanID2[:]),
			newVal: hex.EncodeToString(spanID[:]),
			modified: func(span ptrace.Span) {
				span.SetParentSpanID(spanID)
			},
		},
		{
			name: "name",
			path: &TestPath[*spanContext]{
				N: "name",
			},
			orig:   "bear",
			newVal: "cat",
			modified: func(span ptrace.Span) {
				span.SetName("cat")
			},
		},
		{
			name: "kind",
			path: &TestPath[*spanContext]{
				N: "kind",
			},
			orig:   int64(2),
			newVal: int64(3),
			modified: func(span ptrace.Span) {
				span.SetKind(ptrace.SpanKindClient)
			},
		},
		{
			name: "string kind",
			path: &TestPath[*spanContext]{
				N: "kind",
				NextPath: &TestPath[*spanContext]{
					N: "string",
				},
			},
			orig:   "Server",
			newVal: "Client",
			modified: func(span ptrace.Span) {
				span.SetKind(ptrace.SpanKindClient)
			},
		},
		{
			name: "deprecated string kind",
			path: &TestPath[*spanContext]{
				N: "kind",
				NextPath: &TestPath[*spanContext]{
					N: "deprecated_string",
				},
			},
			orig:   "SPAN_KIND_SERVER",
			newVal: "SPAN_KIND_CLIENT",
			modified: func(span ptrace.Span) {
				span.SetKind(ptrace.SpanKindClient)
			},
		},
		{
			name: "start_time_unix_nano",
			path: &TestPath[*spanContext]{
				N: "start_time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(span ptrace.Span) {
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "end_time_unix_nano",
			path: &TestPath[*spanContext]{
				N: "end_time_unix_nano",
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(span ptrace.Span) {
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "attributes",
			path: &TestPath[*spanContext]{
				N: "attributes",
			},
			orig:   refSpan.Attributes(),
			newVal: newAttrs,
			modified: func(span ptrace.Span) {
				newAttrs.CopyTo(span.Attributes())
			},
		},
		{
			name: "attributes string",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("str"),
					},
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(span ptrace.Span) {
				span.Attributes().PutStr("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("bool"),
					},
				},
			},
			orig:   true,
			newVal: false,
			modified: func(span ptrace.Span) {
				span.Attributes().PutBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("int"),
					},
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(span ptrace.Span) {
				span.Attributes().PutInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("double"),
					},
				},
			},
			orig:   float64(1.2),
			newVal: float64(2.4),
			modified: func(span ptrace.Span) {
				span.Attributes().PutDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("bytes"),
					},
				},
			},
			orig:   []byte{1, 3, 2},
			newVal: []byte{2, 3, 4},
			modified: func(span ptrace.Span) {
				span.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
			},
		},
		{
			name: "attributes array empty",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("arr_empty"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_empty")
				return val.Slice()
			}(),
			newVal: []any{},
			modified: func(_ ptrace.Span) {
				// no-op
			},
		},
		{
			name: "attributes array string",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("arr_str"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(span ptrace.Span) {
				span.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
			},
		},
		{
			name: "attributes array bool",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("arr_bool"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(span ptrace.Span) {
				span.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
			},
		},
		{
			name: "attributes array int",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("arr_int"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(span ptrace.Span) {
				span.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
			},
		},
		{
			name: "attributes array float",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("arr_float"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(span ptrace.Span) {
				span.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
			},
		},
		{
			name: "attributes array bytes",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("arr_bytes"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(span ptrace.Span) {
				span.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
			},
		},
		{
			name: "attributes nested",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("slice"),
					},
					&TestKey[*spanContext]{
						I: ottltest.Intp(0),
					},
					&TestKey[*spanContext]{
						S: ottltest.Strp("map"),
					},
				},
			},
			orig: func() string {
				val, _ := refSpan.Attributes().Get("slice")
				val, _ = val.Slice().At(0).Map().Get("map")
				return val.Str()
			}(),
			newVal: "new",
			modified: func(span ptrace.Span) {
				span.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
			},
		},
		{
			name: "attributes nested new values",
			path: &TestPath[*spanContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*spanContext]{
					&TestKey[*spanContext]{
						S: ottltest.Strp("new"),
					},
					&TestKey[*spanContext]{
						I: ottltest.Intp(2),
					},
					&TestKey[*spanContext]{
						I: ottltest.Intp(0),
					},
				},
			},
			orig: func() any {
				return nil
			}(),
			newVal: "new",
			modified: func(span ptrace.Span) {
				s := span.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
		},
		{
			name: "dropped_attributes_count",
			path: &TestPath[*spanContext]{
				N: "dropped_attributes_count",
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(span ptrace.Span) {
				span.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "events",
			path: &TestPath[*spanContext]{
				N: "events",
			},
			orig:   refSpan.Events(),
			newVal: newEvents,
			modified: func(span ptrace.Span) {
				span.Events().RemoveIf(func(_ ptrace.SpanEvent) bool {
					return true
				})
				newEvents.CopyTo(span.Events())
			},
		},
		{
			name: "dropped_events_count",
			path: &TestPath[*spanContext]{
				N: "dropped_events_count",
			},
			orig:   int64(20),
			newVal: int64(30),
			modified: func(span ptrace.Span) {
				span.SetDroppedEventsCount(30)
			},
		},
		{
			name: "links",
			path: &TestPath[*spanContext]{
				N: "links",
			},
			orig:   refSpan.Links(),
			newVal: newLinks,
			modified: func(span ptrace.Span) {
				span.Links().RemoveIf(func(_ ptrace.SpanLink) bool {
					return true
				})
				newLinks.CopyTo(span.Links())
			},
		},
		{
			name: "dropped_links_count",
			path: &TestPath[*spanContext]{
				N: "dropped_links_count",
			},
			orig:   int64(30),
			newVal: int64(40),
			modified: func(span ptrace.Span) {
				span.SetDroppedLinksCount(40)
			},
		},
		{
			name: "status",
			path: &TestPath[*spanContext]{
				N: "status",
			},
			orig:   refSpan.Status(),
			newVal: newStatus,
			modified: func(span ptrace.Span) {
				newStatus.CopyTo(span.Status())
			},
		},
		{
			name: "status code",
			path: &TestPath[*spanContext]{
				N: "status",
				NextPath: &TestPath[*spanContext]{
					N: "code",
				},
			},
			orig:   int64(ptrace.StatusCodeOk),
			newVal: int64(ptrace.StatusCodeError),
			modified: func(span ptrace.Span) {
				span.Status().SetCode(ptrace.StatusCodeError)
			},
		},
		{
			name: "status message",
			path: &TestPath[*spanContext]{
				N: "status",
				NextPath: &TestPath[*spanContext]{
					N: "message",
				},
			},
			orig:   "good span",
			newVal: "bad span",
			modified: func(span ptrace.Span) {
				span.Status().SetMessage("bad span")
			},
		},
		{
			name: "start_time",
			path: &TestPath[*spanContext]{
				N: "start_time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 100000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(span ptrace.Span) {
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "end_time",
			path: &TestPath[*spanContext]{
				N: "end_time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 500000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(span ptrace.Span) {
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := SpanPathGetSetter[*spanContext](tt.path.Context(), tt.path)
			assert.NoError(t, err)

			span := createSpan()

			got, err := accessor.Get(context.Background(), newSpanContext(span))
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), newSpanContext(span), tt.newVal)
			assert.NoError(t, err)

			expectedSpan := createSpan()
			tt.modified(expectedSpan)

			assert.Equal(t, expectedSpan, span)
		})
	}
}

func TestSpanPathGetSetterCacheAccessError(t *testing.T) {
	path := &TestPath[*spanContext]{
		N: "cache",
		C: "span",
		KeySlice: []ottl.Key[*spanContext]{
			&TestKey[*spanContext]{
				S: ottltest.Strp("key"),
			},
		},
		FullPath: "span.cache[key]",
	}

	_, err := SpanPathGetSetter[*spanContext]("spanevent", path)
	require.Error(t, err)
	require.Contains(t, err.Error(), `replace "span.cache[key]" with "spanevent.cache[key]"`)
}

func createSpan() ptrace.Span {
	span := ptrace.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.TraceState().FromRaw("key1=val1,key2=val2")
	span.SetParentSpanID(spanID2)
	span.SetName("bear")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	span.Attributes().PutStr("str", "val")
	span.Attributes().PutBool("bool", true)
	span.Attributes().PutInt("int", 10)
	span.Attributes().PutDouble("double", 1.2)
	span.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	span.Attributes().PutEmptySlice("arr_empty")

	arrStr := span.Attributes().PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStr("one")
	arrStr.AppendEmpty().SetStr("two")

	arrBool := span.Attributes().PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBool(true)
	arrBool.AppendEmpty().SetBool(false)

	arrInt := span.Attributes().PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetInt(2)
	arrInt.AppendEmpty().SetInt(3)

	arrFloat := span.Attributes().PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDouble(1.0)
	arrFloat.AppendEmpty().SetDouble(2.0)

	arrBytes := span.Attributes().PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{2, 3, 4})

	s := span.Attributes().PutEmptySlice("slice")
	s.AppendEmpty().SetEmptyMap().PutStr("map", "pass")

	span.SetDroppedAttributesCount(10)

	span.Events().AppendEmpty().SetName("event")
	span.SetDroppedEventsCount(20)

	span.Links().AppendEmpty().SetTraceID(traceID)
	span.SetDroppedLinksCount(30)

	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Status().SetMessage("good span")

	return span
}

type spanContext struct {
	span ptrace.Span
}

func (r *spanContext) GetSpan() ptrace.Span {
	return r.span
}

func newSpanContext(span ptrace.Span) *spanContext {
	return &spanContext{span: span}
}
