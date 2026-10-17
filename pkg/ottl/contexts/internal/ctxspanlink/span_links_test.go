// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxspanlink_test

import (
	"encoding/hex"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspanlink"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

var (
	traceID  = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID   = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	traceID2 = [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	spanID2  = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func TestPathGetSetter(t *testing.T) {
	refSpanLink := createTelemetry()

	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")

	tests := []struct {
		name              string
		path              ottl.Path[*testContext]
		orig              any
		newVal            any
		expectSetterError bool
		nilNoError        bool
		modified          func(spanLink ptrace.SpanLink)
	}{
		{
			name: "trace_id",
			path: &pathtest.Path[*testContext]{
				N: "trace_id",
			},
			orig:   pcommon.TraceID(traceID),
			newVal: pcommon.TraceID(traceID2),
			modified: func(spanLink ptrace.SpanLink) {
				spanLink.SetTraceID(traceID2)
			},
		},
		{
			name: "trace_id string",
			path: &pathtest.Path[*testContext]{
				N:        "trace_id",
				NextPath: &pathtest.Path[*testContext]{N: "string"},
			},
			orig:   hex.EncodeToString(traceID[:]),
			newVal: hex.EncodeToString(traceID2[:]),
			modified: func(spanLink ptrace.SpanLink) {
				spanLink.SetTraceID(traceID2)
			},
		},
		{
			name: "span_id",
			path: &pathtest.Path[*testContext]{
				N: "span_id",
			},
			orig:   pcommon.SpanID(spanID),
			newVal: pcommon.SpanID(spanID2),
			modified: func(spanLink ptrace.SpanLink) {
				spanLink.SetSpanID(spanID2)
			},
		},
		{
			name: "span_id string",
			path: &pathtest.Path[*testContext]{
				N:        "span_id",
				NextPath: &pathtest.Path[*testContext]{N: "string"},
			},
			orig:   hex.EncodeToString(spanID[:]),
			newVal: hex.EncodeToString(spanID2[:]),
			modified: func(spanLink ptrace.SpanLink) {
				spanLink.SetSpanID(spanID2)
			},
		},
		{
			name: "trace_state",
			path: &pathtest.Path[*testContext]{
				N: "trace_state",
			},
			orig:   "key1=value1",
			newVal: "key1=value2",
			modified: func(spanLink ptrace.SpanLink) {
				spanLink.TraceState().FromRaw("key1=value2")
			},
		},
		{
			name: "attributes",
			path: &pathtest.Path[*testContext]{
				N: "attributes",
			},
			orig:       refSpanLink.Attributes(),
			newVal:     newAttrs,
			nilNoError: true,
			modified: func(spanLink ptrace.SpanLink) {
				newAttrs.CopyTo(spanLink.Attributes())
			},
		},
		{
			name: "attributes raw map",
			path: &pathtest.Path[*testContext]{
				N: "attributes",
			},
			orig:       refSpanLink.Attributes(),
			newVal:     newAttrs.AsRaw(),
			nilNoError: true,
			modified: func(spanLink ptrace.SpanLink) {
				_ = spanLink.Attributes().FromRaw(newAttrs.AsRaw())
			},
		},
		{
			name: "attributes string",
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
						S: new("str"),
					},
				},
			},
			orig:       "val",
			newVal:     "newVal",
			nilNoError: true,
			modified: func(spanLink ptrace.SpanLink) {
				spanLink.Attributes().PutStr("str", "newVal")
			},
		},
		{
			name: "attributes int",
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
						S: new("int"),
					},
				},
			},
			orig:       int64(10),
			newVal:     int64(20),
			nilNoError: true,
			modified: func(spanLink ptrace.SpanLink) {
				spanLink.Attributes().PutInt("int", 20)
			},
		},
		{
			name: "dropped_attributes_count",
			path: &pathtest.Path[*testContext]{
				N: "dropped_attributes_count",
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(spanLink ptrace.SpanLink) {
				spanLink.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "flags",
			path: &pathtest.Path[*testContext]{
				N: "flags",
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(spanLink ptrace.SpanLink) {
				spanLink.SetFlags(2)
			},
		},
	}

	// Also test with explicit context prefix on the path.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[*testContext])
		pathWithContext.C = ctxspanlink.Name
		testWithContext.path = ottl.Path[*testContext](&pathWithContext)
		tests = append(tests, testWithContext)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := ctxspanlink.PathGetSetter(tt.path)
			require.NoError(t, err)

			spanLink := createTelemetry()
			tCtx := newTestContext(spanLink)

			got, err := accessor.Get(t.Context(), tCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(t.Context(), tCtx, tt.newVal)
			if tt.expectSetterError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			exSpanLink := createTelemetry()
			tt.modified(exSpanLink)
			assert.Equal(t, exSpanLink, spanLink)

			err = accessor.Set(t.Context(), tCtx, struct{}{})
			require.Error(t, err)

			err = accessor.Set(t.Context(), tCtx, nil)
			if tt.nilNoError {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestPathGetSetter_InvalidPath(t *testing.T) {
	_, err := ctxspanlink.PathGetSetter[*testContext](&pathtest.Path[*testContext]{N: "unknown_field"})
	assert.Error(t, err)
}

func Test_PathGetSetter_InvalidSubPath(t *testing.T) {
	tests := []struct {
		name string
		path ottl.Path[*testContext]
	}{
		{
			name: "trace_id",
			path: &pathtest.Path[*testContext]{
				N:        "trace_id",
				NextPath: &pathtest.Path[*testContext]{N: "unknown_field"},
			},
		},
		{
			name: "span_id",
			path: &pathtest.Path[*testContext]{
				N:        "span_id",
				NextPath: &pathtest.Path[*testContext]{N: "unknown_field"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctxspanlink.PathGetSetter(tt.path)
			assert.Error(t, err)
		})
	}
}

func Test_PathGetSetter_InvalidIDString(t *testing.T) {
	tests := []struct {
		name string
		path ottl.Path[*testContext]
	}{
		{
			name: "trace_id string",
			path: &pathtest.Path[*testContext]{
				N:        "trace_id",
				NextPath: &pathtest.Path[*testContext]{N: "string"},
			},
		},
		{
			name: "span_id string",
			path: &pathtest.Path[*testContext]{
				N:        "span_id",
				NextPath: &pathtest.Path[*testContext]{N: "string"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := ctxspanlink.PathGetSetter(tt.path)
			require.NoError(t, err)

			err = accessor.Set(t.Context(), newTestContext(createTelemetry()), "invalid")
			assert.Error(t, err)
		})
	}
}

func Test_PathGetSetter_FlagsOutOfRange(t *testing.T) {
	accessor, err := ctxspanlink.PathGetSetter[*testContext](&pathtest.Path[*testContext]{N: "flags"})
	require.NoError(t, err)

	err = accessor.Set(t.Context(), newTestContext(createTelemetry()), int64(-1))
	assert.Error(t, err)

	err = accessor.Set(t.Context(), newTestContext(createTelemetry()), int64(math.MaxUint32)+1)
	assert.Error(t, err)
}

func TestPathGetSetter_NilPath(t *testing.T) {
	_, err := ctxspanlink.PathGetSetter[*testContext](nil)
	assert.Error(t, err)
}

func createTelemetry() ptrace.SpanLink {
	spanLink := ptrace.NewSpanLink()
	spanLink.SetTraceID(traceID)
	spanLink.SetSpanID(spanID)
	spanLink.TraceState().FromRaw("key1=value1")
	spanLink.Attributes().PutStr("str", "val")
	spanLink.Attributes().PutInt("int", 10)
	spanLink.SetDroppedAttributesCount(10)
	spanLink.SetFlags(1)
	return spanLink
}

type testContext struct {
	spanLink ptrace.SpanLink
}

func (t *testContext) GetSpanLink() ptrace.SpanLink {
	return t.spanLink
}

func newTestContext(spanLink ptrace.SpanLink) *testContext {
	return &testContext{spanLink: spanLink}
}
