// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlexemplar

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxdatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxexemplar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter(t *testing.T) {
	_, _, _, _, refExemplar := createTelemetry()

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
		modified          func(exemplar pmetric.Exemplar, cache pcommon.Map)
	}{
		{
			name: "time_unix_nano",
			path: &pathtest.Path[*TransformContext]{
				N: "time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(exemplar pmetric.Exemplar, _ pcommon.Map) {
				exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time",
			path: &pathtest.Path[*TransformContext]{
				N: "time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 100000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(exemplar pmetric.Exemplar, _ pcommon.Map) {
				exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "double_value",
			path: &pathtest.Path[*TransformContext]{
				N: "double_value",
			},
			orig:   float64(1.5),
			newVal: float64(3.0),
			modified: func(exemplar pmetric.Exemplar, _ pcommon.Map) {
				exemplar.SetDoubleValue(3.0)
			},
		},
		{
			name: "int_value",
			path: &pathtest.Path[*TransformContext]{
				N: "int_value",
			},
			orig:   int64(0),
			newVal: int64(42),
			modified: func(exemplar pmetric.Exemplar, _ pcommon.Map) {
				exemplar.SetIntValue(42)
			},
		},
		{
			name: "trace_id",
			path: &pathtest.Path[*TransformContext]{
				N: "trace_id",
			},
			orig:   pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
			newVal: pcommon.TraceID([16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}),
			modified: func(exemplar pmetric.Exemplar, _ pcommon.Map) {
				exemplar.SetTraceID([16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1})
			},
		},
		{
			name: "span_id",
			path: &pathtest.Path[*TransformContext]{
				N: "span_id",
			},
			orig:   pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
			newVal: pcommon.SpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1}),
			modified: func(exemplar pmetric.Exemplar, _ pcommon.Map) {
				exemplar.SetSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})
			},
		},
		{
			name: "filtered_attributes",
			path: &pathtest.Path[*TransformContext]{
				N: "filtered_attributes",
			},
			orig:   refExemplar.FilteredAttributes(),
			newVal: newAttrs,
			modified: func(exemplar pmetric.Exemplar, _ pcommon.Map) {
				newAttrs.CopyTo(exemplar.FilteredAttributes())
			},
		},
		{
			name: "filtered_attributes string",
			path: &pathtest.Path[*TransformContext]{
				N: "filtered_attributes",
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: ottltest.Strp("str"),
					},
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(exemplar pmetric.Exemplar, _ pcommon.Map) {
				exemplar.FilteredAttributes().PutStr("str", "newVal")
			},
		},
		{
			name: "cache",
			path: &pathtest.Path[*TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ pmetric.Exemplar, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
		},
		{
			name: "cache access",
			path: &pathtest.Path[*TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: ottltest.Strp("temp"),
					},
				},
			},
			orig:   nil,
			newVal: "new value",
			modified: func(_ pmetric.Exemplar, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
		},
	}
	// Copy all test cases and set the path.Context value to ctxexemplar.Name.
	// It ensures all existing field access also works when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[*TransformContext])
		pathWithContext.C = ctxexemplar.Name
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

			rm, sm, metric, dp, exemplar := createTelemetry()

			tCtx := NewTransformContextPtr(rm, sm, metric, dp, exemplar)
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

			_, _, _, _, exExemplar := createTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exExemplar, exCache)

			assert.Equal(t, exExemplar, exemplar)
			assert.Equal(t, exCache, testCache)
		})
	}
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	rm := pmetric.NewResourceMetrics()
	rm.Resource().Attributes().PutStr("foo", "bar")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("scope")

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("my.metric")

	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.5)

	exemplar := dp.Exemplars().AppendEmpty()
	exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))

	ctx := NewTransformContextPtr(rm, sm, metric, dp, exemplar)
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
						S: ottltest.Strp("foo"),
					},
				},
			}},
			expected: "bar",
		},
		{
			name: "resource with context",
			path: &pathtest.Path[*TransformContext]{C: "resource", N: "attributes", KeySlice: []ottl.Key[*TransformContext]{
				&pathtest.Key[*TransformContext]{
					S: ottltest.Strp("foo"),
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
			name:     "instrumentation_scope with context",
			path:     &pathtest.Path[*TransformContext]{C: "instrumentation_scope", N: "name"},
			expected: "scope",
		},
		{
			name:     "scope",
			path:     &pathtest.Path[*TransformContext]{N: "scope", NextPath: &pathtest.Path[*TransformContext]{N: "name"}},
			expected: "scope",
		},
		{
			name:     "scope with context",
			path:     &pathtest.Path[*TransformContext]{C: "scope", N: "name"},
			expected: "scope",
		},
		{
			name:     "metric",
			path:     &pathtest.Path[*TransformContext]{N: "metric", NextPath: &pathtest.Path[*TransformContext]{N: "name"}},
			expected: "my.metric",
		},
		{
			name:     "metric with context",
			path:     &pathtest.Path[*TransformContext]{C: "metric", N: "name"},
			expected: "my.metric",
		},
		{
			name:     "datapoint value_double with context",
			path:     &pathtest.Path[*TransformContext]{C: "datapoint", N: "value_double"},
			expected: float64(1.5),
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
		ctxmetric.Name,
		ctxdatapoint.Name,
	}
	for _, higherContext := range higherContexts {
		t.Run(higherContext, func(t *testing.T) {
			path := &pathtest.Path[*TransformContext]{
				N: "cache",
				C: higherContext,
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: ottltest.Strp("key"),
					},
				},
				FullPath: fmt.Sprintf("%s.cache[key]", higherContext),
			}

			_, err := pathExpressionParser(getCache)(path)
			require.Error(t, err)
			expectError := fmt.Sprintf(`replace "%s.cache[key]" with "exemplar.cache[key]"`, higherContext)
			require.ErrorContains(t, err, expectError)
		})
	}
}

func createTelemetry() (pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric, pmetric.NumberDataPoint, pmetric.Exemplar) {
	rm := pmetric.NewResourceMetrics()
	rm.Resource().Attributes().PutStr("resource_attr", "resource_val")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("library")
	sm.Scope().SetVersion("version")

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("my.metric")
	metric.SetDescription("A test metric")

	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.5)
	dp.Attributes().PutStr("dp_attr", "dp_val")

	exemplar := dp.Exemplars().AppendEmpty()
	exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	exemplar.SetDoubleValue(1.5)
	exemplar.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	exemplar.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	exemplar.FilteredAttributes().PutStr("str", "val")

	return rm, sm, metric, dp, exemplar
}
