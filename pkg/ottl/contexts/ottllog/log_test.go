// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottllog

import (
	"context"
	"encoding/hex"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxlog"
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
	refLog, _, _ := createTelemetry("string")

	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")

	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newBodyMap := pcommon.NewMap()
	newBodyMap.PutStr("new", "value")

	newBodySlice := pcommon.NewSlice()
	newBodySlice.AppendEmpty().SetStr("data")

	newMap := make(map[string]any)
	newMap2 := make(map[string]any)
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		orig     any
		newVal   any
		modified func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map)
		bodyType string
	}{
		{
			name: "time",
			path: &pathtest.Path[TransformContext]{
				N: "time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 100000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "observed_time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "observed_time_unix_nano",
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "observed time",
			path: &pathtest.Path[TransformContext]{
				N: "observed_time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 500000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "severity_number",
			path: &pathtest.Path[TransformContext]{
				N: "severity_number",
			},
			orig:   int64(plog.SeverityNumberFatal),
			newVal: int64(3),
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetSeverityNumber(plog.SeverityNumberTrace3)
			},
		},
		{
			name: "severity_text",
			path: &pathtest.Path[TransformContext]{
				N: "severity_text",
			},
			orig:   "blue screen of death",
			newVal: "black screen of death",
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetSeverityText("black screen of death")
			},
		},
		{
			name: "body",
			path: &pathtest.Path[TransformContext]{
				N: "body",
			},
			orig:   "body",
			newVal: "head",
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Body().SetStr("head")
			},
		},
		{
			name: "map body",
			path: &pathtest.Path[TransformContext]{
				N: "body",
			},
			orig: func() pcommon.Map {
				log, _, _ := createTelemetry("map")
				return log.Body().Map()
			}(),
			newVal: newBodyMap,
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				newBodyMap.CopyTo(log.Body().Map())
			},
			bodyType: "map",
		},
		{
			name: "map body index",
			path: &pathtest.Path[TransformContext]{
				N: "body",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("key"),
					},
				},
			},
			orig:   "val",
			newVal: "val2",
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Body().Map().PutStr("key", "val2")
			},
			bodyType: "map",
		},
		{
			name: "slice body",
			path: &pathtest.Path[TransformContext]{
				N: "body",
			},
			orig: func() pcommon.Slice {
				log, _, _ := createTelemetry("slice")
				return log.Body().Slice()
			}(),
			newVal: newBodySlice,
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				newBodySlice.CopyTo(log.Body().Slice())
			},
			bodyType: "slice",
		},
		{
			name: "slice body index",
			path: &pathtest.Path[TransformContext]{
				N: "body",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						I: ottltest.Intp(0),
					},
				},
			},
			orig:   "body",
			newVal: "head",
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Body().Slice().At(0).SetStr("head")
			},
			bodyType: "slice",
		},
		{
			name: "body string",
			path: &pathtest.Path[TransformContext]{
				N: "body",
				NextPath: &pathtest.Path[TransformContext]{
					N: "string",
				},
			},
			orig:   "1",
			newVal: "2",
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Body().SetStr("2")
			},
			bodyType: "int",
		},
		{
			name: "flags",
			path: &pathtest.Path[TransformContext]{
				N: "flags",
			},
			orig:   int64(4),
			newVal: int64(5),
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetFlags(plog.LogRecordFlags(5))
			},
		},
		{
			name: "trace_id",
			path: &pathtest.Path[TransformContext]{
				N: "trace_id",
			},
			orig:   pcommon.TraceID(traceID),
			newVal: pcommon.TraceID(traceID2),
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetTraceID(traceID2)
			},
		},
		{
			name: "span_id",
			path: &pathtest.Path[TransformContext]{
				N: "span_id",
			},
			orig:   pcommon.SpanID(spanID),
			newVal: pcommon.SpanID(spanID2),
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetSpanID(spanID2)
			},
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
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetTraceID(traceID2)
			},
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
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetSpanID(spanID2)
			},
		},
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
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
			modified: func(_ plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
		},
		{
			name: "attributes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
			},
			orig:   refLog.Attributes(),
			newVal: newAttrs,
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				newAttrs.CopyTo(log.Attributes())
			},
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
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutStr("str", "newVal")
			},
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
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutBool("bool", false)
			},
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
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutInt("int", 20)
			},
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
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutDouble("double", 2.4)
			},
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
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
			},
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
				val, _ := refLog.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
			},
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
				val, _ := refLog.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
			},
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
				val, _ := refLog.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
			},
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
				val, _ := refLog.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
			},
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
				val, _ := refLog.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
			},
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
				val, _ := refLog.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				m := log.Attributes().PutEmptyMap("pMap")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
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
				val, _ := refLog.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				m := log.Attributes().PutEmptyMap("map")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
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
				val, _ := refLog.Attributes().Get("slice")
				val, _ = val.Slice().At(0).Map().Get("map")
				return val.Str()
			}(),
			newVal: "new",
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
			},
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
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				s := log.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
		},
		{
			name: "dropped_attributes_count",
			path: &pathtest.Path[TransformContext]{
				N: "dropped_attributes_count",
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(log plog.LogRecord, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				log.SetDroppedAttributesCount(20)
			},
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxlog.Name
		testWithContext.path = &pathWithContext
		tests = append(tests, testWithContext)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCache := pcommon.NewMap()
			cacheGetter := func(_ TransformContext) pcommon.Map {
				return testCache
			}
			accessor, err := pathExpressionParser(cacheGetter)(tt.path)
			assert.NoError(t, err)

			log, il, resource := createTelemetry(tt.bodyType)

			tCtx := NewTransformContext(log, il, resource, plog.NewScopeLogs(), plog.NewResourceLogs())
			got, err := accessor.Get(context.Background(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			tCtx = NewTransformContext(log, il, resource, plog.NewScopeLogs(), plog.NewResourceLogs())
			err = accessor.Set(context.Background(), tCtx, tt.newVal)
			assert.NoError(t, err)

			exLog, exIl, exRes := createTelemetry(tt.bodyType)
			exCache := pcommon.NewMap()
			tt.modified(exLog, exIl, exRes, exCache)

			assert.Equal(t, exLog, log)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exRes, resource)
			assert.Equal(t, exCache, testCache)
		})
	}
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	logRec, instrumentationScope, resource := createTelemetry("string")
	ctx := NewTransformContext(logRec, instrumentationScope, resource, plog.NewScopeLogs(), plog.NewResourceLogs())

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		expected any
	}{
		{
			name: "resource",
			path: &pathtest.Path[TransformContext]{C: "", N: "resource", NextPath: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("str"),
					},
				},
			}},
			expected: "val",
		},
		{
			name: "resource with context",
			path: &pathtest.Path[TransformContext]{C: "resource", N: "attributes", KeySlice: []ottl.Key[TransformContext]{
				&pathtest.Key[TransformContext]{
					S: ottltest.Strp("str"),
				},
			}},
			expected: "val",
		},
		{
			name:     "instrumentation_scope",
			path:     &pathtest.Path[TransformContext]{N: "instrumentation_scope", NextPath: &pathtest.Path[TransformContext]{N: "name"}},
			expected: instrumentationScope.Name(),
		},
		{
			name:     "instrumentation_scope with context",
			path:     &pathtest.Path[TransformContext]{C: "instrumentation_scope", N: "name"},
			expected: instrumentationScope.Name(),
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
}

func createTelemetry(bodyType string) (plog.LogRecord, pcommon.InstrumentationScope, pcommon.Resource) {
	log := plog.NewLogRecord()
	log.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	log.SetSeverityNumber(plog.SeverityNumberFatal)
	log.SetSeverityText("blue screen of death")
	log.Attributes().PutStr("str", "val")
	log.Attributes().PutBool("bool", true)
	log.Attributes().PutInt("int", 10)
	log.Attributes().PutDouble("double", 1.2)
	log.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	arrStr := log.Attributes().PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStr("one")
	arrStr.AppendEmpty().SetStr("two")

	arrBool := log.Attributes().PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBool(true)
	arrBool.AppendEmpty().SetBool(false)

	arrInt := log.Attributes().PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetInt(2)
	arrInt.AppendEmpty().SetInt(3)

	arrFloat := log.Attributes().PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDouble(1.0)
	arrFloat.AppendEmpty().SetDouble(2.0)

	arrBytes := log.Attributes().PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{2, 3, 4})

	pMap := log.Attributes().PutEmptyMap("pMap")
	pMap.PutStr("original", "map")

	m := log.Attributes().PutEmptyMap("map")
	m.PutStr("original", "map")

	s := log.Attributes().PutEmptySlice("slice")
	s.AppendEmpty().SetEmptyMap().PutStr("map", "pass")

	switch bodyType {
	case "map":
		log.Body().SetEmptyMap().PutStr("key", "val")
	case "slice":
		log.Body().SetEmptySlice().AppendEmpty().SetStr("body")
	case "int":
		log.Body().SetInt(1)
	case "string":
		fallthrough
	default:
		log.Body().SetStr("body")
	}

	log.SetDroppedAttributesCount(10)

	log.SetFlags(plog.LogRecordFlags(4))

	log.SetTraceID(traceID)
	log.SetSpanID(spanID)

	il := pcommon.NewInstrumentationScope()
	il.SetName("library")
	il.SetVersion("version")

	resource := pcommon.NewResource()
	log.Attributes().CopyTo(resource.Attributes())

	return log, il, resource
}

func Test_InvalidBodyIndexing(t *testing.T) {
	path := pathtest.Path[TransformContext]{
		N: "body",
		KeySlice: []ottl.Key[TransformContext]{
			&pathtest.Key[TransformContext]{
				S: ottltest.Strp("key"),
			},
		},
	}

	accessor, err := pathExpressionParser(getCache)(&path)
	assert.NoError(t, err)

	log, il, resource := createTelemetry("string")

	tCtx := NewTransformContext(log, il, resource, plog.NewScopeLogs(), plog.NewResourceLogs())
	_, err = accessor.Get(context.Background(), tCtx)
	assert.Error(t, err)

	tCtx = NewTransformContext(log, il, resource, plog.NewScopeLogs(), plog.NewResourceLogs())
	err = accessor.Set(context.Background(), tCtx, nil)
	assert.Error(t, err)
}

func Test_ParseEnum(t *testing.T) {
	tests := []struct {
		name string
		want ottl.Enum
	}{
		{
			name: "SEVERITY_NUMBER_UNSPECIFIED",
			want: ottl.Enum(plog.SeverityNumberUnspecified),
		},
		{
			name: "SEVERITY_NUMBER_TRACE",
			want: ottl.Enum(plog.SeverityNumberTrace),
		},
		{
			name: "SEVERITY_NUMBER_TRACE2",
			want: ottl.Enum(plog.SeverityNumberTrace2),
		},
		{
			name: "SEVERITY_NUMBER_TRACE3",
			want: ottl.Enum(plog.SeverityNumberTrace3),
		},
		{
			name: "SEVERITY_NUMBER_TRACE4",
			want: ottl.Enum(plog.SeverityNumberTrace4),
		},
		{
			name: "SEVERITY_NUMBER_DEBUG",
			want: ottl.Enum(plog.SeverityNumberDebug),
		},
		{
			name: "SEVERITY_NUMBER_DEBUG2",
			want: ottl.Enum(plog.SeverityNumberDebug2),
		},
		{
			name: "SEVERITY_NUMBER_DEBUG3",
			want: ottl.Enum(plog.SeverityNumberDebug3),
		},
		{
			name: "SEVERITY_NUMBER_DEBUG4",
			want: ottl.Enum(plog.SeverityNumberDebug4),
		},
		{
			name: "SEVERITY_NUMBER_INFO",
			want: ottl.Enum(plog.SeverityNumberInfo),
		},
		{
			name: "SEVERITY_NUMBER_INFO2",
			want: ottl.Enum(plog.SeverityNumberInfo2),
		},
		{
			name: "SEVERITY_NUMBER_INFO3",
			want: ottl.Enum(plog.SeverityNumberInfo3),
		},
		{
			name: "SEVERITY_NUMBER_INFO4",
			want: ottl.Enum(plog.SeverityNumberInfo4),
		},
		{
			name: "SEVERITY_NUMBER_WARN",
			want: ottl.Enum(plog.SeverityNumberWarn),
		},
		{
			name: "SEVERITY_NUMBER_WARN2",
			want: ottl.Enum(plog.SeverityNumberWarn2),
		},
		{
			name: "SEVERITY_NUMBER_WARN3",
			want: ottl.Enum(plog.SeverityNumberWarn3),
		},
		{
			name: "SEVERITY_NUMBER_WARN4",
			want: ottl.Enum(plog.SeverityNumberWarn4),
		},
		{
			name: "SEVERITY_NUMBER_ERROR",
			want: ottl.Enum(plog.SeverityNumberError),
		},
		{
			name: "SEVERITY_NUMBER_ERROR2",
			want: ottl.Enum(plog.SeverityNumberError2),
		},
		{
			name: "SEVERITY_NUMBER_ERROR3",
			want: ottl.Enum(plog.SeverityNumberError3),
		},
		{
			name: "SEVERITY_NUMBER_ERROR4",
			want: ottl.Enum(plog.SeverityNumberError4),
		},
		{
			name: "SEVERITY_NUMBER_FATAL",
			want: ottl.Enum(plog.SeverityNumberFatal),
		},
		{
			name: "SEVERITY_NUMBER_FATAL2",
			want: ottl.Enum(plog.SeverityNumberFatal2),
		},
		{
			name: "SEVERITY_NUMBER_FATAL3",
			want: ottl.Enum(plog.SeverityNumberFatal3),
		},
		{
			name: "SEVERITY_NUMBER_FATAL4",
			want: ottl.Enum(plog.SeverityNumberFatal4),
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
