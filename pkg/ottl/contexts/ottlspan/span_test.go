// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlspan

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"

	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
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
	newAttrs.PutStr("hello", "world")

	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	newEvents := ptrace.NewSpanEventSlice()
	newEvents.AppendEmpty().SetName("new event")

	newLinks := ptrace.NewSpanLinkSlice()
	newLinks.AppendEmpty().SetSpanID(spanID2)

	newStatus := ptrace.NewStatus()
	newStatus.SetMessage("new status")

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newMap := make(map[string]any)
	newMap2 := make(map[string]any)
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name         string
		path         ottl.Path[TransformContext]
		orig         any
		newVal       any
		modified     func(span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map)
		setStatement string
		getStatement string
	}{
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
			setStatement: `set(cache, {"temp": "value"})`,
			getStatement: `cache`,
		},
		{
			name: "cache access",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("temp"),
					},
				},
			},
			orig:   nil,
			newVal: "new value",
			modified: func(_ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
			setStatement: `set(cache["temp"], "new value")`,
			getStatement: `cache["temp"]`,
		},
		{
			name: "trace_id",
			path: &pathtest.Path[TransformContext]{
				N: "trace_id",
			},
			orig:   pcommon.TraceID(traceID),
			newVal: pcommon.TraceID(traceID2),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetTraceID(traceID2)
			},
			setStatement: `set(trace_id, TraceID(0x100f0e0d0c0b0a090807060504030201))`,
			getStatement: `trace_id`,
		},
		{
			name: "span_id",
			path: &pathtest.Path[TransformContext]{
				N: "span_id",
			},
			orig:   pcommon.SpanID(spanID),
			newVal: pcommon.SpanID(spanID2),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetSpanID(spanID2)
			},
			setStatement: `set(span_id, SpanID(0x0807060504030201))`,
			getStatement: `span_id`,
		},
		{
			name: "trace_id string",
			path: &pathtest.Path[TransformContext]{
				N: "trace_id",
				NextPath: &pathtest.Path[TransformContext]{
					N: "string",
				},
			},
			orig:   hex.EncodeToString(traceID[:]),
			newVal: hex.EncodeToString(traceID2[:]),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetTraceID(traceID2)
			},
			setStatement: `set(trace_id.string, "100f0e0d0c0b0a090807060504030201")`,
			getStatement: `trace_id.string`,
		},
		{
			name: "span_id string",
			path: &pathtest.Path[TransformContext]{
				N: "span_id",
				NextPath: &pathtest.Path[TransformContext]{
					N: "string",
				},
			},
			orig:   hex.EncodeToString(spanID[:]),
			newVal: hex.EncodeToString(spanID2[:]),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetSpanID(spanID2)
			},
			setStatement: `set(span_id.string, "0807060504030201")`,
			getStatement: `span_id.string`,
		},
		{
			name: "trace_state",
			path: &pathtest.Path[TransformContext]{
				N: "trace_state",
			},
			orig:   "key1=val1,key2=val2",
			newVal: "key=newVal",
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.TraceState().FromRaw("key=newVal")
			},
			setStatement: `set(trace_state, "key=newVal")`,
			getStatement: `trace_state`,
		},
		{
			name: "trace_state key",
			path: &pathtest.Path[TransformContext]{
				N: "trace_state",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("key1"),
					},
				},
			},
			orig:   "val1",
			newVal: "newVal",
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.TraceState().FromRaw("key1=newVal,key2=val2")
			},
			setStatement: `set(trace_state["key1"], "newVal")`,
			getStatement: `trace_state["key1"]`,
		},
		{
			name: "parent_span_id",
			path: &pathtest.Path[TransformContext]{
				N: "parent_span_id",
			},
			orig:   pcommon.SpanID(spanID2),
			newVal: pcommon.SpanID(spanID),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetParentSpanID(spanID)
			},
			setStatement: `set(parent_span_id, SpanID(0x0807060504030201))`,
			getStatement: `parent_span_id`,
		},
		{
			name: "name",
			path: &pathtest.Path[TransformContext]{
				N: "name",
			},
			orig:   "bear",
			newVal: "cat",
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetName("cat")
			},
			setStatement: `set(name, "cat")`,
			getStatement: `name`,
		},
		{
			name: "kind",
			path: &pathtest.Path[TransformContext]{
				N: "kind",
			},
			orig:   int64(2),
			newVal: int64(3),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetKind(ptrace.SpanKindClient)
			},
			setStatement: fmt.Sprintf("set(kind, %d)", int64(ptrace.SpanKindClient)),
			getStatement: `kind`,
		},
		{
			name: "string kind",
			path: &pathtest.Path[TransformContext]{
				N: "kind",
				NextPath: &pathtest.Path[TransformContext]{
					N: "string",
				},
			},
			orig:   "Server",
			newVal: "Client",
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetKind(ptrace.SpanKindClient)
			},
			setStatement: `set(kind.string, "Client")`,
			getStatement: `kind.string`,
		},
		{
			name: "deprecated string kind",
			path: &pathtest.Path[TransformContext]{
				N: "kind",
				NextPath: &pathtest.Path[TransformContext]{
					N: "deprecated_string",
				},
			},
			orig:   "SPAN_KIND_SERVER",
			newVal: "SPAN_KIND_CLIENT",
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetKind(ptrace.SpanKindClient)
			},
			setStatement: `set(kind.deprecated_string, "SPAN_KIND_CLIENT")`,
			getStatement: `kind.deprecated_string`,
		},
		{
			name: "start_time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "start_time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
			setStatement: "set(start_time_unix_nano, 200000000)",
			getStatement: "start_time_unix_nano",
		},
		{
			name: "end_time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "end_time_unix_nano",
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
			setStatement: "set(end_time_unix_nano, 200000000)",
			getStatement: "end_time_unix_nano",
		},
		{
			name: "start_time",
			path: &pathtest.Path[TransformContext]{
				N: "start_time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 100000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
			setStatement: `set(start_time, Time("1970-01-01T00:00:00.2Z", "%Y-%m-%dT%H:%M:%S.%f%z"))`,
			getStatement: `start_time`,
		},
		{
			name: "end_time",
			path: &pathtest.Path[TransformContext]{
				N: "end_time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 500000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
			setStatement: `set(end_time, Time("1970-01-01T00:00:00.5Z", "%Y-%m-%dT%H:%M:%S.%f%z"))`,
			getStatement: `end_time`,
		},
		{
			name: "attributes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
			},
			orig:   refSpan.Attributes(),
			newVal: newAttrs,
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				newAttrs.CopyTo(span.Attributes())
			},
			setStatement: `set(attributes, {"hello": "world"})`,
			getStatement: `attributes`,
		},
		{
			name: "attributes string",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("str"),
					},
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutStr("str", "newVal")
			},
			setStatement: `set(attributes["str"], "newVal")`,
			getStatement: `attributes["str"]`,
		},
		{
			name: "attributes bool",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("bool"),
					},
				},
			},
			orig:   true,
			newVal: false,
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutBool("bool", false)
			},
			setStatement: `set(attributes["bool"], false)`,
			getStatement: `attributes["bool"]`,
		},
		{
			name: "attributes int",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("int"),
					},
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutInt("int", 20)
			},
			setStatement: `set(attributes["int"], 20)`,
			getStatement: `attributes["int"]`,
		},
		{
			name: "attributes float",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("double"),
					},
				},
			},
			orig:   float64(1.2),
			newVal: float64(2.4),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutDouble("double", 2.4)
			},
			setStatement: `set(attributes["double"], 2.4)`,
			getStatement: `attributes["double"]`,
		},
		{
			name: "attributes bytes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("bytes"),
					},
				},
			},
			orig:   []byte{1, 3, 2},
			newVal: []byte{2, 3, 4},
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
			},
			setStatement: `set(attributes["bytes"], 0x020304)`,
			getStatement: `attributes["bytes"]`,
		},
		{
			name: "attributes array string",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_str"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
			},
			setStatement: `set(attributes["arr_str"], ["new"])`,
			getStatement: `attributes["arr_str"]`,
		},
		{
			name: "attributes array bool",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_bool"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
			},
			setStatement: `set(attributes["arr_bool"], [false])`,
			getStatement: `attributes["arr_bool"]`,
		},
		{
			name: "attributes array int",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_int"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
			},
			setStatement: `set(attributes["arr_int"], [20])`,
			getStatement: `attributes["arr_int"]`,
		},
		{
			name: "attributes array float",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_float"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
			},
			setStatement: `set(attributes["arr_float"], [2.0])`,
			getStatement: `attributes["arr_float"]`,
		},
		{
			name: "attributes array bytes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_bytes"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refSpan.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
			},
			setStatement: `set(attributes["arr_bytes"], [0x090604])`,
			getStatement: `attributes["arr_bytes"]`,
		},
		{
			name: "attributes pcommon.Map",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("pMap"),
					},
				},
			},
			orig: func() pcommon.Map {
				val, _ := refSpan.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				m := span.Attributes().PutEmptyMap("pMap")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
			setStatement: `set(attributes["pMap"], {"k2": {"k1": "string"}})`,
			getStatement: `attributes["pMap"]`,
		},
		{
			name: "attributes map[string]any",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("map"),
					},
				},
			},
			orig: func() pcommon.Map {
				val, _ := refSpan.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				m := span.Attributes().PutEmptyMap("map")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
			setStatement: `set(attributes["map"], {"k2": {"k1": "string"}})`,
			getStatement: `attributes["map"]`,
		},
		{
			name: "attributes nested",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("slice"),
					},
					&pathtest.Key[TransformContext]{
						I: ottltest.Intp(0),
					},
					&pathtest.Key[TransformContext]{
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
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
			},
			setStatement: `set(attributes["slice"], [{"map": "new"}])`,
			getStatement: `attributes["slice"][0]["map"]`,
		},
		{
			name: "attributes nested new values",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("new"),
					},
					&pathtest.Key[TransformContext]{
						I: ottltest.Intp(2),
					},
					&pathtest.Key[TransformContext]{
						I: ottltest.Intp(0),
					},
				},
			},
			orig: func() any {
				return nil
			}(),
			newVal: "new",
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				s := span.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
			setStatement: `set(attributes["new"], [nil, nil, ["new"]])`,
			getStatement: `attributes["new"][2][0]`,
		},
		{
			name: "dropped_attributes_count",
			path: &pathtest.Path[TransformContext]{
				N: "dropped_attributes_count",
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetDroppedAttributesCount(20)
			},
			setStatement: `set(dropped_attributes_count, 20)`,
			getStatement: `dropped_attributes_count`,
		},
		{
			name: "events",
			path: &pathtest.Path[TransformContext]{
				N: "events",
			},
			orig:   refSpan.Events(),
			newVal: newEvents,
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Events().RemoveIf(func(_ ptrace.SpanEvent) bool {
					return true
				})
				newEvents.CopyTo(span.Events())
			},
		},
		{
			name: "dropped_events_count",
			path: &pathtest.Path[TransformContext]{
				N: "dropped_events_count",
			},
			orig:   int64(20),
			newVal: int64(30),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetDroppedEventsCount(30)
			},
			setStatement: `set(dropped_events_count, 30)`,
			getStatement: `dropped_events_count`,
		},
		{
			name: "links",
			path: &pathtest.Path[TransformContext]{
				N: "links",
			},
			orig:   refSpan.Links(),
			newVal: newLinks,
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Links().RemoveIf(func(_ ptrace.SpanLink) bool {
					return true
				})
				newLinks.CopyTo(span.Links())
			},
		},
		{
			name: "dropped_links_count",
			path: &pathtest.Path[TransformContext]{
				N: "dropped_links_count",
			},
			orig:   int64(30),
			newVal: int64(40),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.SetDroppedLinksCount(40)
			},
			setStatement: `set(dropped_links_count, 40)`,
			getStatement: `dropped_links_count`,
		},
		{
			name: "status",
			path: &pathtest.Path[TransformContext]{
				N: "status",
			},
			orig:   refSpan.Status(),
			newVal: newStatus,
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				newStatus.CopyTo(span.Status())
			},
		},
		{
			name: "status code",
			path: &pathtest.Path[TransformContext]{
				N: "status",
				NextPath: &pathtest.Path[TransformContext]{
					N: "code",
				},
			},
			orig:   int64(ptrace.StatusCodeOk),
			newVal: int64(ptrace.StatusCodeError),
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Status().SetCode(ptrace.StatusCodeError)
			},
			setStatement: fmt.Sprintf("set(status.code, %d)", int64(ptrace.StatusCodeError)),
			getStatement: `status.code`,
		},
		{
			name: "status message",
			path: &pathtest.Path[TransformContext]{
				N: "status",
				NextPath: &pathtest.Path[TransformContext]{
					N: "message",
				},
			},
			orig:   "good span",
			newVal: "bad span",
			modified: func(span ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				span.Status().SetMessage("bad span")
			},
			setStatement: `set(status.message, "bad span")`,
			getStatement: `status.message`,
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxspan.Name
		testWithContext.path = ottl.Path[TransformContext](&pathWithContext)
		tests = append(tests, testWithContext)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a controlled cache map for testing
			testCache := pcommon.NewMap()
			cacheGetter := func(_ TransformContext) pcommon.Map {
				return testCache
			}
			accessor, err := pathExpressionParser(cacheGetter)(tt.path)
			assert.NoError(t, err)

			span, il, resource := createTelemetry()

			tCtx := NewTransformContext(span, il, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())

			got, err := accessor.Get(context.Background(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), tCtx, tt.newVal)
			assert.NoError(t, err)

			exSpan, exIl, exRes := createTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exSpan, exIl, exRes, exCache)

			assert.Equal(t, exSpan, span)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exRes, resource)
			assert.Equal(t, exCache, testCache)
		})
	}

	stmtParser := createParser(t)

	for _, tt := range tests {
		t.Run(tt.name+"_conversion", func(t *testing.T) {
			if tt.setStatement != "" {
				statement, err := stmtParser.ParseStatement(tt.setStatement)
				require.NoError(t, err)

				span, il, resource := createTelemetry()

				ctx := NewTransformContext(span, il, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())

				_, executed, err := statement.Execute(context.Background(), ctx)
				require.NoError(t, err)
				assert.True(t, executed)

				getStatement, err := stmtParser.ParseValueExpression(tt.getStatement)
				require.NoError(t, err)

				span, il, resource = createTelemetry()

				ctx = NewTransformContext(span, il, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())

				getResult, err := getStatement.Eval(context.Background(), ctx)

				assert.NoError(t, err)
				assert.Equal(t, tt.orig, getResult)
			}
		})
	}
}

func createParser(t *testing.T) ottl.Parser[TransformContext] {
	settings := componenttest.NewNopTelemetrySettings()
	// stmtParser, err := NewParser(ottlfuncs.StandardFuncs[TransformContext](), settings)
	stmtParser, err := NewParser(MinimalTestFuncs[TransformContext](), settings)
	// stmtParser, err := NewParser(common.Functions[ottlspan.TransformContext](), settings)
	// stmtParser, err := NewParser(map[string]ottl.Factory[TransformContext]{}, settings)
	require.NoError(t, err)
	return stmtParser
}

func MinimalTestFuncs[K any]() map[string]ottl.Factory[K] {
	f := []ottl.Factory[K]{
		// Editors
		NewSetFactory[K](),
	}
	f = append(f, converters[K]()...)

	return ottl.CreateFactoryMap(f...)
}

// This is really ugly but avoid a cyclic import problem.
// This test resides in ottlspan package, but the Parser requires ottlfuncs.StandardFuncs. One of the functions in
// ottlfuncs (func_is_root_span.go) import itself the ottlspan package, so we can't refer any member present in ottlfuncs
// without creating a loop, so the nasty copy of code.

func converters[K any]() []ottl.Factory[K] {
	return []ottl.Factory[K]{
		// Converters
		NewSpanIDFactory[K](),
		NewTimeFactory[K](),
		NewTraceIDFactory[K](),
	}
}

// Set function
type SetArguments[K any] struct {
	Target ottl.Setter[K]
	Value  ottl.Getter[K]
}

func NewSetFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("set", &SetArguments[K]{}, createSetFunction[K])
}

func createSetFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SetArguments[K])

	if !ok {
		return nil, errors.New("SetFactory args must be of type *SetArguments[K]")
	}

	return set(args.Target, args.Value), nil
}

func set[K any](target ottl.Setter[K], value ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := value.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		// No fields currently support `null` as a valid type.
		if val != nil {
			err = target.Set(ctx, tCtx, val)
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
}

// TraceID converter
type TraceIDArguments[K any] struct {
	Bytes []byte
}

func NewTraceIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TraceID", &TraceIDArguments[K]{}, createTraceIDFunction[K])
}

func createTraceIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TraceIDArguments[K])

	if !ok {
		return nil, errors.New("TraceIDFactory args must be of type *TraceIDArguments[K]")
	}

	return traceIDFun[K](args.Bytes)
}

func traceIDFun[K any](bytes []byte) (ottl.ExprFunc[K], error) {
	if len(bytes) != 16 {
		return nil, errors.New("traces ids must be 16 bytes")
	}
	var idArr [16]byte
	copy(idArr[:16], bytes)
	id := pcommon.TraceID(idArr)
	return func(context.Context, K) (any, error) {
		return id, nil
	}, nil
}

// SpanID converter
type SpanIDArguments[K any] struct {
	Bytes []byte
}

func NewSpanIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SpanID", &SpanIDArguments[K]{}, createSpanIDFunction[K])
}

func createSpanIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SpanIDArguments[K])

	if !ok {
		return nil, errors.New("SpanIDFactory args must be of type *SpanIDArguments[K]")
	}

	return spanIDFun[K](args.Bytes)
}

func spanIDFun[K any](bytes []byte) (ottl.ExprFunc[K], error) {
	if len(bytes) != 8 {
		return nil, errors.New("span ids must be 8 bytes")
	}
	var idArr [8]byte
	copy(idArr[:8], bytes)
	id := pcommon.SpanID(idArr)
	return func(context.Context, K) (any, error) {
		return id, nil
	}, nil
}

// Time converter
type TimeArguments[K any] struct {
	Time     ottl.StringGetter[K]
	Format   string
	Location ottl.Optional[string]
	Locale   ottl.Optional[string]
}

func NewTimeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Time", &TimeArguments[K]{}, createTimeFunction[K])
}

func createTimeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TimeArguments[K])

	if !ok {
		return nil, errors.New("TimeFactory args must be of type *TimeArguments[K]")
	}

	return Time(args.Time, args.Format, args.Location, args.Locale)
}

func Time[K any](inputTime ottl.StringGetter[K], format string, location ottl.Optional[string], locale ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	if format == "" {
		return nil, errors.New("format cannot be nil")
	}
	gotimeFormat, err := timeutils.StrptimeToGotime(format)
	if err != nil {
		return nil, err
	}

	var defaultLocation *string
	if !location.IsEmpty() {
		l := location.Get()
		defaultLocation = &l
	}

	loc, err := timeutils.GetLocation(defaultLocation, &format)
	if err != nil {
		return nil, err
	}

	var inputTimeLocale *string
	if !locale.IsEmpty() {
		l := locale.Get()
		if err = timeutils.ValidateLocale(l); err != nil {
			return nil, err
		}
		inputTimeLocale = &l
	}

	ctimeSubstitutes := timeutils.GetStrptimeNativeSubstitutes(format)

	return func(ctx context.Context, tCtx K) (any, error) {
		t, err := inputTime.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if t == "" {
			return nil, errors.New("time cannot be nil")
		}
		var timestamp time.Time
		if inputTimeLocale != nil {
			timestamp, err = timeutils.ParseLocalizedGotime(gotimeFormat, t, loc, *inputTimeLocale)
		} else {
			timestamp, err = timeutils.ParseGotime(gotimeFormat, t, loc)
		}
		if err != nil {
			var timeErr *time.ParseError
			if errors.As(err, &timeErr) {
				return nil, timeutils.ToStrptimeParseError(timeErr, format, ctimeSubstitutes)
			}
			return nil, err
		}
		return timestamp, nil
	}, nil
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("foo", "bar")

	instrumentationScope := pcommon.NewInstrumentationScope()
	instrumentationScope.SetName("instrumentation_scope")

	ctx := NewTransformContext(ptrace.NewSpan(), instrumentationScope, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())

	tests := []struct {
		name         string
		path         ottl.Path[TransformContext]
		expected     any
		getStatement string
	}{
		{
			name: "resource",
			path: &pathtest.Path[TransformContext]{C: "", N: "resource", NextPath: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("foo"),
					},
				},
			}},
			expected:     "bar",
			getStatement: `resource.attributes["foo"]`,
		},
		{
			name: "resource with context",
			path: &pathtest.Path[TransformContext]{C: "resource", N: "attributes", KeySlice: []ottl.Key[TransformContext]{
				&pathtest.Key[TransformContext]{
					S: ottltest.Strp("foo"),
				},
			}},
			expected:     "bar",
			getStatement: `resource.attributes["foo"]`,
		},
		{
			name:         "instrumentation_scope",
			path:         &pathtest.Path[TransformContext]{N: "instrumentation_scope", NextPath: &pathtest.Path[TransformContext]{N: "name"}},
			expected:     instrumentationScope.Name(),
			getStatement: `instrumentation_scope.name`,
		},
		{
			name:         "instrumentation_scope with context",
			path:         &pathtest.Path[TransformContext]{C: "instrumentation_scope", N: "name"},
			expected:     instrumentationScope.Name(),
			getStatement: `instrumentation_scope.name`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := pathExpressionParser(getCache)(tt.path)
			require.NoError(t, err)

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}

	stmtParser := createParser(t)
	for _, tt := range tests {
		t.Run(tt.name+"_conversion", func(t *testing.T) {
			getExpression, err := stmtParser.ParseValueExpression(tt.getStatement)
			require.NoError(t, err)
			require.NotNil(t, getExpression)
			getResult, err := getExpression.Eval(context.Background(), ctx)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, getResult)
		})
	}
}

func TestHigherContextCacheAccessError(t *testing.T) {
	path := &pathtest.Path[TransformContext]{
		N: "cache",
		C: ctxresource.Name,
		KeySlice: []ottl.Key[TransformContext]{
			&pathtest.Key[TransformContext]{
				S: ottltest.Strp("key"),
			},
		},
		FullPath: fmt.Sprintf("%s.cache[key]", ctxresource.Name),
	}

	_, err := pathExpressionParser(getCache)(path)
	require.Error(t, err)
	expectError := fmt.Sprintf(`replace "%s.cache[key]" with "span.cache[key]"`, ctxresource.Name)
	require.Contains(t, err.Error(), expectError)
}

func createTelemetry() (ptrace.Span, pcommon.InstrumentationScope, pcommon.Resource) {
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

	pMap := span.Attributes().PutEmptyMap("pMap")
	pMap.PutStr("original", "map")

	m := span.Attributes().PutEmptyMap("map")
	m.PutStr("original", "map")

	s := span.Attributes().PutEmptySlice("slice")
	s.AppendEmpty().SetEmptyMap().PutStr("map", "pass")

	span.SetDroppedAttributesCount(10)

	span.Events().AppendEmpty().SetName("event")
	span.SetDroppedEventsCount(20)

	span.Links().AppendEmpty().SetTraceID(traceID)
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

func Test_ParseEnum(t *testing.T) {
	tests := []struct {
		name string
		want ottl.Enum
	}{
		{
			name: "SPAN_KIND_UNSPECIFIED",
			want: ottl.Enum(ptrace.SpanKindUnspecified),
		},
		{
			name: "SPAN_KIND_INTERNAL",
			want: ottl.Enum(ptrace.SpanKindInternal),
		},
		{
			name: "SPAN_KIND_SERVER",
			want: ottl.Enum(ptrace.SpanKindServer),
		},
		{
			name: "SPAN_KIND_CLIENT",
			want: ottl.Enum(ptrace.SpanKindClient),
		},
		{
			name: "SPAN_KIND_PRODUCER",
			want: ottl.Enum(ptrace.SpanKindProducer),
		},
		{
			name: "SPAN_KIND_CONSUMER",
			want: ottl.Enum(ptrace.SpanKindConsumer),
		},
		{
			name: "STATUS_CODE_UNSET",
			want: ottl.Enum(ptrace.StatusCodeUnset),
		},
		{
			name: "STATUS_CODE_OK",
			want: ottl.Enum(ptrace.StatusCodeOk),
		},
		{
			name: "STATUS_CODE_ERROR",
			want: ottl.Enum(ptrace.StatusCodeError),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := parseEnum((*ottl.EnumSymbol)(ottltest.Strp(tt.name)))
			assert.NoError(t, err)
			assert.Equal(t, tt.want, *actual)
		})
	}
}

func Test_ParseEnum_False(t *testing.T) {
	tests := []struct {
		name       string
		enumSymbol *ottl.EnumSymbol
	}{
		{
			name:       "unknown enum symbol",
			enumSymbol: (*ottl.EnumSymbol)(ottltest.Strp("not an enum")),
		},
		{
			name:       "nil enum symbol",
			enumSymbol: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := parseEnum(tt.enumSymbol)
			assert.Error(t, err)
			assert.Nil(t, actual)
		})
	}
}
