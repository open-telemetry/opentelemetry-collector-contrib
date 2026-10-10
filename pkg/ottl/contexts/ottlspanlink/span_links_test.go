// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlspanlink

import (
	"encoding/hex"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/cachetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspanlink"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

var (
	traceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2 = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func Test_newPathGetSetter(t *testing.T) {
	_, _, _, refSpanLink := createTelemetry()

	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")

	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	tests := []struct {
		name              string
		path              ottl.Path[*TransformContext]
		orig              any
		newVal            any
		expectSetterError bool
		modified          func(spanLink ptrace.SpanLink, cache pcommon.Map)
	}{
		{
			name: "cache",
			path: &pathtest.Path[*TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ ptrace.SpanLink, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
		},
		{
			name: "cache access",
			path: &pathtest.Path[*TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: new("temp"),
					},
				},
			},
			orig:   nil,
			newVal: "new value",
			modified: func(_ ptrace.SpanLink, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
		},
		{
			name: "trace_id",
			path: &pathtest.Path[*TransformContext]{
				N: "trace_id",
			},
			orig:   pcommon.TraceID(traceID),
			newVal: pcommon.TraceID(spanID2Trace),
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				spanLink.SetTraceID(spanID2Trace)
			},
		},
		{
			name: "trace_id string",
			path: &pathtest.Path[*TransformContext]{
				N:        "trace_id",
				NextPath: &pathtest.Path[*TransformContext]{N: "string"},
			},
			orig:   hex.EncodeToString(traceID[:]),
			newVal: hex.EncodeToString(spanID2Trace[:]),
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				spanLink.SetTraceID(spanID2Trace)
			},
		},
		{
			name: "span_id",
			path: &pathtest.Path[*TransformContext]{
				N: "span_id",
			},
			orig:   pcommon.SpanID(spanID),
			newVal: pcommon.SpanID(spanID2),
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				spanLink.SetSpanID(spanID2)
			},
		},
		{
			name: "span_id string",
			path: &pathtest.Path[*TransformContext]{
				N:        "span_id",
				NextPath: &pathtest.Path[*TransformContext]{N: "string"},
			},
			orig:   hex.EncodeToString(spanID[:]),
			newVal: hex.EncodeToString(spanID2[:]),
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				spanLink.SetSpanID(spanID2)
			},
		},
		{
			name: "trace_state",
			path: &pathtest.Path[*TransformContext]{
				N: "trace_state",
			},
			orig:   "key1=value1",
			newVal: "key1=value2",
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				spanLink.TraceState().FromRaw("key1=value2")
			},
		},
		{
			name: "attributes",
			path: &pathtest.Path[*TransformContext]{
				N: "attributes",
			},
			orig:   refSpanLink.Attributes(),
			newVal: newAttrs,
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				newAttrs.CopyTo(spanLink.Attributes())
			},
		},
		{
			name: "attributes raw map",
			path: &pathtest.Path[*TransformContext]{
				N: "attributes",
			},
			orig:   refSpanLink.Attributes(),
			newVal: newAttrs.AsRaw(),
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				_ = spanLink.Attributes().FromRaw(newAttrs.AsRaw())
			},
		},
		{
			name: "attributes string",
			path: &pathtest.Path[*TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: new("str"),
					},
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				spanLink.Attributes().PutStr("str", "newVal")
			},
		},
		{
			name: "attributes int",
			path: &pathtest.Path[*TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: new("int"),
					},
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				spanLink.Attributes().PutInt("int", 20)
			},
		},
		{
			name: "dropped_attributes_count",
			path: &pathtest.Path[*TransformContext]{
				N: "dropped_attributes_count",
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				spanLink.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "flags",
			path: &pathtest.Path[*TransformContext]{
				N: "flags",
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(spanLink ptrace.SpanLink, _ pcommon.Map) {
				spanLink.SetFlags(2)
			},
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[*TransformContext])
		pathWithContext.C = ctxspanlink.Name
		testWithContext.path = ottl.Path[*TransformContext](&pathWithContext)
		tests = append(tests, testWithContext)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCache := pcommon.NewMap()
			cacheGetter := func(*TransformContext) pcommon.Map {
				return testCache
			}

			accessor, err := pathExpressionParser(cacheGetter)(tt.path)
			require.NoError(t, err)

			rs, _, _, spanLink := createTelemetry()

			tCtx := NewTransformContext(rs, rs.ScopeSpans().At(0), rs.ScopeSpans().At(0).Spans().At(0), spanLink)
			defer tCtx.Close()

			got, err := accessor.Get(t.Context(), tCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(t.Context(), tCtx, tt.newVal)
			if tt.expectSetterError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			exRS, _, _, exSpanLink := createTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exSpanLink, exCache)

			assert.Equal(t, exRS, rs)
			assert.Equal(t, exCache, testCache)
		})
	}
}

var spanID2Trace = [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}

func Test_newPathGetSetter_InvalidPath(t *testing.T) {
	_, err := pathExpressionParser(getCache)(&pathtest.Path[*TransformContext]{N: "unknown_field"})
	assert.Error(t, err)
}

func Test_newPathGetSetter_NilPath(t *testing.T) {
	_, err := pathExpressionParser(getCache)(nil)
	assert.Error(t, err)
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	rs := ptrace.NewResourceSpans()
	rs.Resource().Attributes().PutStr("foo", "bar")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("scope")

	span := ss.Spans().AppendEmpty()
	span.SetName("span")

	ctx := NewTransformContext(rs, ss, span, ptrace.NewSpanLink())
	defer ctx.Close()

	tests := []struct {
		name     string
		path     ottl.Path[*TransformContext]
		expected any
	}{
		{
			name: "resource",
			path: &pathtest.Path[*TransformContext]{C: "", N: "resource", NextPath: &pathtest.Path[*TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: new("foo"),
					},
				},
			}},
			expected: "bar",
		},
		{
			name:     "instrumentation_scope",
			path:     &pathtest.Path[*TransformContext]{N: "instrumentation_scope", NextPath: &pathtest.Path[*TransformContext]{N: "name"}},
			expected: "scope",
		},
		{
			name:     "span",
			path:     &pathtest.Path[*TransformContext]{N: "span", NextPath: &pathtest.Path[*TransformContext]{N: "name"}},
			expected: span.Name(),
		},
		{
			name:     "span with context",
			path:     &pathtest.Path[*TransformContext]{C: "span", N: "name"},
			expected: span.Name(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := pathExpressionParser(getCache)(tt.path)
			require.NoError(t, err)

			got, err := accessor.Get(t.Context(), ctx)
			require.NoError(t, err)
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
			path := &pathtest.Path[*TransformContext]{
				N: "cache",
				C: higherContext,
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: new("key"),
					},
				},
				FullPath: fmt.Sprintf("%s.cache[key]", higherContext),
			}

			_, err := pathExpressionParser(getCache)(path)
			require.Error(t, err)
			expectError := fmt.Sprintf(`replace "%s.cache[key]" with "spanlink.cache[key]"`, higherContext)
			require.ErrorContains(t, err, expectError)
		})
	}
}

func Test_ParseEnum(t *testing.T) {
	tests := []struct {
		name string
		want ottl.Enum
	}{
		{name: "SPAN_KIND_CLIENT", want: ottl.Enum(ptrace.SpanKindClient)},
		{name: "STATUS_CODE_ERROR", want: ottl.Enum(ptrace.StatusCodeError)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := parseEnum((*ottl.EnumSymbol)(new(tt.name)))
			require.NoError(t, err)
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
			enumSymbol: (*ottl.EnumSymbol)(new("not an enum")),
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

func Test_WithCache(t *testing.T) {
	cachetest.TestWithCache(t, cachetest.Context[*TransformContext, TransformContextOption]{
		Name:                 ContextName,
		PathExpressionParser: pathExpressionParser(getCache),
		NewTransformContext: func(options ...TransformContextOption) *TransformContext {
			return NewTransformContext(ptrace.NewResourceSpans(), ptrace.NewScopeSpans(), ptrace.NewSpan(), ptrace.NewSpanLink(), options...)
		},
		WithCache: WithCache,
		LocalCache: func(tCtx *TransformContext) pcommon.Map {
			return tCtx.cache
		},
		ExternalCache: func(tCtx *TransformContext) *pcommon.Map {
			return tCtx.externalCache
		},
	})
}

func TestMarshalLogObject(t *testing.T) {
	rs, ss, span, spanLink := createTelemetry()
	tCtx := NewTransformContext(rs, ss, span, spanLink)
	defer tCtx.Close()

	encoder := zapcore.NewMapObjectEncoder()
	require.NoError(t, tCtx.MarshalLogObject(encoder))

	spanLinkFields, ok := encoder.Fields["spanlink"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, hex.EncodeToString(spanID[:]), spanLinkFields["span_id"])
}

// Test_multipleLinksDoNotLeakBetweenContexts guards against the pooled TransformContext
// carrying state from one link's transform into another link's on the same span.
func Test_multipleLinksDoNotLeakBetweenContexts(t *testing.T) {
	rs := ptrace.NewResourceSpans()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	link1 := span.Links().AppendEmpty()
	link1.SetSpanID(spanID)
	link2 := span.Links().AppendEmpty()
	link2.SetSpanID(spanID2)

	tCtx1 := NewTransformContext(rs, ss, span, link1)
	require.NoError(t, tCtx1.GetSpanLink().Attributes().FromRaw(map[string]any{"only_on_link1": true}))
	tCtx1.Close()

	tCtx2 := NewTransformContext(rs, ss, span, link2)
	defer tCtx2.Close()
	_, found := tCtx2.GetSpanLink().Attributes().Get("only_on_link1")
	assert.False(t, found, "attribute set on link1 must not leak into link2's pooled context")
	assert.Equal(t, pcommon.SpanID(spanID2), tCtx2.GetSpanLink().SpanID())
}

func createTelemetry() (ptrace.ResourceSpans, ptrace.ScopeSpans, ptrace.Span, ptrace.SpanLink) {
	rs := ptrace.NewResourceSpans()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test")

	spanLink := span.Links().AppendEmpty()
	spanLink.SetTraceID(traceID)
	spanLink.SetSpanID(spanID)
	spanLink.TraceState().FromRaw("key1=value1")
	spanLink.SetDroppedAttributesCount(10)
	spanLink.SetFlags(1)

	spanLink.Attributes().PutStr("str", "val")
	spanLink.Attributes().PutInt("int", 10)

	ss.Scope().SetName("library")
	ss.Scope().SetVersion("version")

	span.Attributes().CopyTo(rs.Resource().Attributes())

	return rs, ss, span, spanLink
}
