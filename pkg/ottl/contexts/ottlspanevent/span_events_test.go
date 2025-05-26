// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlspanevent

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

var spanID2 = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}

func Test_newPathGetSetter(t *testing.T) {
	refSpanEvent, _, _, _ := createTelemetry()

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
		name              string
		path              ottl.Path[TransformContext]
		orig              any
		newVal            any
		expectSetterError bool
		modified          func(spanEvent ptrace.SpanEvent, span ptrace.Span, il pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map)
	}{
		{
			name: "span event time",
			path: &pathtest.Path[TransformContext]{
				N: "time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 100000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, cache pcommon.Map) {
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
			modified: func(_ ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
		},
		{
			name: "name",
			path: &pathtest.Path[TransformContext]{
				N: "name",
			},
			orig:   "bear",
			newVal: "cat",
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.SetName("cat")
			},
		},
		{
			name: "time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "attributes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
			},
			orig:   refSpanEvent.Attributes(),
			newVal: newAttrs,
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				newAttrs.CopyTo(spanEvent.Attributes())
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
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutStr("str", "newVal")
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
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutBool("bool", false)
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
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutInt("int", 20)
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
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutDouble("double", 2.4)
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
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
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
				val, _ := refSpanEvent.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
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
				val, _ := refSpanEvent.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
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
				val, _ := refSpanEvent.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
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
				val, _ := refSpanEvent.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
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
				val, _ := refSpanEvent.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
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
				val, _ := refSpanEvent.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				m := spanEvent.Attributes().PutEmptyMap("pMap")
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
				val, _ := refSpanEvent.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				m := spanEvent.Attributes().PutEmptyMap("map")
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
				val, _ := refSpanEvent.Attributes().Get("slice")
				val, _ = val.Slice().At(0).Map().Get("map")
				return val.Str()
			}(),
			newVal: "new",
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
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
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				s := spanEvent.Attributes().PutEmptySlice("new")
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
			modified: func(spanEvent ptrace.SpanEvent, _ ptrace.Span, _ pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				spanEvent.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "event_index",
			path: &pathtest.Path[TransformContext]{
				N: "event_index",
			},
			orig:              int64(1),
			newVal:            int64(1),
			expectSetterError: true,
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxspanevent.Name
		testWithContext.path = ottl.Path[TransformContext](&pathWithContext)
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

			spanEvent, span, il, resource := createTelemetry()

			tCtx := NewTransformContext(spanEvent, span, il, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans(), WithEventIndex(1))

			got, err := accessor.Get(context.Background(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), tCtx, tt.newVal)
			if tt.expectSetterError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			exSpanEvent, exSpan, exIl, exRes := createTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exSpanEvent, exSpan, exIl, exRes, exCache)

			assert.Equal(t, exSpan, span)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exRes, resource)
			assert.Equal(t, exCache, testCache)
		})
	}
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("foo", "bar")

	span := ptrace.NewSpan()
	span.SetName("span")

	instrumentationScope := pcommon.NewInstrumentationScope()
	instrumentationScope.SetName("instrumentation_scope")

	ctx := NewTransformContext(ptrace.NewSpanEvent(), span, instrumentationScope, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())

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
						S: ottltest.Strp("foo"),
					},
				},
			}},
			expected: "bar",
		},
		{
			name: "resource with context",
			path: &pathtest.Path[TransformContext]{C: "resource", N: "attributes", KeySlice: []ottl.Key[TransformContext]{
				&pathtest.Key[TransformContext]{
					S: ottltest.Strp("foo"),
				},
			}},
			expected: "bar",
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
		{
			name:     "span",
			path:     &pathtest.Path[TransformContext]{N: "span", NextPath: &pathtest.Path[TransformContext]{N: "name"}},
			expected: span.Name(),
		},
		{
			name:     "span with context",
			path:     &pathtest.Path[TransformContext]{C: "span", N: "name"},
			expected: span.Name(),
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

func Test_setAndGetEventIndex(t *testing.T) {
	tests := []struct {
		name             string
		setEventIndex    bool
		eventIndexValue  int64
		expected         any
		expectedErrorMsg string
	}{
		{
			name:            "event index set",
			setEventIndex:   true,
			eventIndexValue: 1,
			expected:        int64(1),
		},
		{
			name:             "invalid value for event index",
			setEventIndex:    true,
			eventIndexValue:  -1,
			expectedErrorMsg: "found invalid value for 'event_index'",
		},
		{
			name:             "no value for event index",
			setEventIndex:    false,
			expectedErrorMsg: "no 'event_index' property has been set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spanEvent, span, il, resource := createTelemetry()

			var tCtx TransformContext
			if tt.setEventIndex {
				tCtx = NewTransformContext(spanEvent, span, il, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans(), WithEventIndex(tt.eventIndexValue))
			} else {
				tCtx = NewTransformContext(spanEvent, span, il, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())
			}

			accessor, err := pathExpressionParser(getCache)(&pathtest.Path[TransformContext]{
				N: "event_index",
			})
			assert.NoError(t, err)

			got, err := accessor.Get(context.Background(), tCtx)
			if tt.expectedErrorMsg != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.expectedErrorMsg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestHigherContextCacheAccessError(t *testing.T) {
	higherContexts := []string{
		ctxresource.Name,
		ctxscope.Name,
		ctxscope.LegacyName,
		ctxspan.Name,
	}
	for _, higherContext := range higherContexts {
		t.Run(higherContext, func(t *testing.T) {
			path := &pathtest.Path[TransformContext]{
				N: "cache",
				C: higherContext,
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("key"),
					},
				},
				FullPath: fmt.Sprintf("%s.cache[key]", higherContext),
			}

			_, err := pathExpressionParser(getCache)(path)
			require.Error(t, err)
			expectError := fmt.Sprintf(`replace "%s.cache[key]" with "spanevent.cache[key]"`, higherContext)
			require.Contains(t, err.Error(), expectError)
		})
	}
}

func createTelemetry() (ptrace.SpanEvent, ptrace.Span, pcommon.InstrumentationScope, pcommon.Resource) {
	span := ptrace.NewSpan()
	span.SetName("test")

	spanEvent := span.Events().AppendEmpty()

	spanEvent.SetName("bear")
	spanEvent.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	spanEvent.SetDroppedAttributesCount(10)

	spanEvent.Attributes().PutStr("str", "val")
	spanEvent.Attributes().PutBool("bool", true)
	spanEvent.Attributes().PutInt("int", 10)
	spanEvent.Attributes().PutDouble("double", 1.2)
	spanEvent.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	arrStr := spanEvent.Attributes().PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStr("one")
	arrStr.AppendEmpty().SetStr("two")

	arrBool := spanEvent.Attributes().PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBool(true)
	arrBool.AppendEmpty().SetBool(false)

	arrInt := spanEvent.Attributes().PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetInt(2)
	arrInt.AppendEmpty().SetInt(3)

	arrFloat := spanEvent.Attributes().PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDouble(1.0)
	arrFloat.AppendEmpty().SetDouble(2.0)

	arrBytes := spanEvent.Attributes().PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{2, 3, 4})

	pMap := spanEvent.Attributes().PutEmptyMap("pMap")
	pMap.PutStr("original", "map")

	m := spanEvent.Attributes().PutEmptyMap("map")
	m.PutStr("original", "map")

	s := spanEvent.Attributes().PutEmptySlice("slice")
	s.AppendEmpty().SetEmptyMap().PutStr("map", "pass")

	il := pcommon.NewInstrumentationScope()
	il.SetName("library")
	il.SetVersion("version")

	resource := pcommon.NewResource()
	span.Attributes().CopyTo(resource.Attributes())

	return spanEvent, span, il, resource
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
