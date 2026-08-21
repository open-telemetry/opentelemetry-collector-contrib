// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxexemplar_test

import (
	"encoding/hex"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxexemplar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

var (
	traceID  = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID   = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	traceID2 = [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	spanID2  = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func TestPathGetSetter(t *testing.T) {
	refExemplar := createTelemetry()

	newFilteredAttrs := pcommon.NewMap()
	newFilteredAttrs.PutStr("hello", "world")

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newMap := make(map[string]any)
	newMap2 := make(map[string]any)
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name              string
		path              ottl.Path[*testContext]
		orig              any
		newVal            any
		expectSetterError bool
		nilNoError        bool
		modified          func(exemplar pmetric.Exemplar)
	}{
		{
			name: "time_unix_nano",
			path: &pathtest.Path[*testContext]{
				N: "time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time",
			path: &pathtest.Path[*testContext]{
				N: "time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 100000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "double_value",
			path: &pathtest.Path[*testContext]{
				N: "double_value",
			},
			orig:   float64(1.5),
			newVal: float64(2.5),
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.SetDoubleValue(2.5)
			},
		},
		{
			name: "int_value",
			path: &pathtest.Path[*testContext]{
				N: "int_value",
			},
			orig:   int64(0),
			newVal: int64(42),
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.SetIntValue(42)
			},
		},
		{
			name: "trace_id",
			path: &pathtest.Path[*testContext]{
				N: "trace_id",
			},
			orig:   pcommon.TraceID(traceID),
			newVal: pcommon.TraceID(traceID2),
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.SetTraceID(pcommon.TraceID(traceID2))
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
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.SetTraceID(pcommon.TraceID(traceID2))
			},
		},
		{
			name: "span_id",
			path: &pathtest.Path[*testContext]{
				N: "span_id",
			},
			orig:   pcommon.SpanID(spanID),
			newVal: pcommon.SpanID(spanID2),
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.SetSpanID(pcommon.SpanID(spanID2))
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
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.SetSpanID(pcommon.SpanID(spanID2))
			},
		},
		{
			name: "filtered_attributes",
			path: &pathtest.Path[*testContext]{
				N: "filtered_attributes",
			},
			orig:       refExemplar.FilteredAttributes(),
			newVal:     newFilteredAttrs,
			nilNoError: true,
			modified: func(exemplar pmetric.Exemplar) {
				newFilteredAttrs.CopyTo(exemplar.FilteredAttributes())
			},
		},
		{
			name: "filtered_attributes raw map",
			path: &pathtest.Path[*testContext]{
				N: "filtered_attributes",
			},
			orig:       refExemplar.FilteredAttributes(),
			newVal:     newFilteredAttrs.AsRaw(),
			nilNoError: true,
			modified: func(exemplar pmetric.Exemplar) {
				_ = exemplar.FilteredAttributes().FromRaw(newFilteredAttrs.AsRaw())
			},
		},
		{
			name: "filtered_attributes string",
			path: &pathtest.Path[*testContext]{
				N: "filtered_attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
						S: ottltest.Strp("str"),
					},
				},
			},
			orig:       "val",
			newVal:     "newVal",
			nilNoError: true,
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.FilteredAttributes().PutStr("str", "newVal")
			},
		},
		{
			name: "filtered_attributes int",
			path: &pathtest.Path[*testContext]{
				N: "filtered_attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
						S: ottltest.Strp("int"),
					},
				},
			},
			orig:       int64(10),
			newVal:     int64(20),
			nilNoError: true,
			modified: func(exemplar pmetric.Exemplar) {
				exemplar.FilteredAttributes().PutInt("int", 20)
			},
		},
	}

	// Also test with explicit context prefix on the path.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[*testContext])
		pathWithContext.C = ctxexemplar.Name
		testWithContext.path = ottl.Path[*testContext](&pathWithContext)
		tests = append(tests, testWithContext)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := ctxexemplar.PathGetSetter(tt.path)
			require.NoError(t, err)

			exemplar := createTelemetry()
			tCtx := newTestContext(exemplar)

			got, err := accessor.Get(t.Context(), tCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(t.Context(), tCtx, tt.newVal)
			if tt.expectSetterError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			exExemplar := createTelemetry()
			tt.modified(exExemplar)
			assert.Equal(t, exExemplar, exemplar)

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
	_, err := ctxexemplar.PathGetSetter[*testContext](&pathtest.Path[*testContext]{N: "unknown_field"})
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
			_, err := ctxexemplar.PathGetSetter(tt.path)
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
			accessor, err := ctxexemplar.PathGetSetter(tt.path)
			require.NoError(t, err)

			err = accessor.Set(t.Context(), newTestContext(createTelemetry()), "invalid")
			assert.Error(t, err)
		})
	}
}

func TestPathGetSetter_NilPath(t *testing.T) {
	_, err := ctxexemplar.PathGetSetter[*testContext](nil)
	assert.Error(t, err)
}

func createTelemetry() pmetric.Exemplar {
	exemplar := pmetric.NewExemplar()
	exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	exemplar.SetDoubleValue(1.5)
	exemplar.SetTraceID(pcommon.TraceID(traceID))
	exemplar.SetSpanID(pcommon.SpanID(spanID))
	exemplar.FilteredAttributes().PutStr("str", "val")
	exemplar.FilteredAttributes().PutInt("int", 10)
	return exemplar
}

type testContext struct {
	exemplar pmetric.Exemplar
}

func (t *testContext) GetExemplar() pmetric.Exemplar {
	return t.exemplar
}

func newTestContext(exemplar pmetric.Exemplar) *testContext {
	return &testContext{exemplar: exemplar}
}
