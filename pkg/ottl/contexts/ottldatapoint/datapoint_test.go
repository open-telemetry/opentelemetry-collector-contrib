// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottldatapoint

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxdatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter_Cache(t *testing.T) {
	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	tests := []struct {
		name      string
		path      ottl.Path[TransformContext]
		orig      any
		newVal    any
		modified  func(cache pcommon.Map)
		valueType pmetric.NumberDataPointValueType
	}{
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(cache pcommon.Map) {
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
			modified: func(cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxdatapoint.Name
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

			numberDataPoint := createNumberDataPointTelemetry(tt.valueType)

			ctx := NewTransformContext(numberDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics(), WithCache(&testCache))

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.NoError(t, err)

			exCache := pcommon.NewMap()
			tt.modified(exCache)

			assert.Equal(t, exCache, testCache)
		})
	}
}

func Test_newPathGetSetter_WithCache(t *testing.T) {
	cacheValue := pcommon.NewMap()
	cacheValue.PutStr("test", "pass")

	tCtx := NewTransformContext(
		pmetric.NewNumberDataPoint(),
		pmetric.NewMetric(),
		pmetric.NewMetricSlice(),
		pcommon.NewInstrumentationScope(),
		pcommon.NewResource(),
		pmetric.NewScopeMetrics(),
		pmetric.NewResourceMetrics(),
		WithCache(&cacheValue),
	)

	assert.Equal(t, cacheValue, getCache(tCtx))
}

func Test_newPathGetSetter_NumberDataPoint(t *testing.T) {
	refNumberDataPoint := createNumberDataPointTelemetry(pmetric.NumberDataPointValueTypeInt)

	newExemplars, newAttrs := createNewTelemetry()

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newMap := make(map[string]any)
	newMap2 := make(map[string]any)
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name      string
		path      ottl.Path[TransformContext]
		orig      any
		newVal    any
		modified  func(pmetric.NumberDataPoint)
		valueType pmetric.NumberDataPointValueType
	}{
		{
			name: "start_time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "start_time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "start_time",
			path: &pathtest.Path[TransformContext]{
				N: "start_time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 100000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 86400000000000, time.UTC),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(86400, 0)))
			},
		},
		{
			name: "time",
			path: &pathtest.Path[TransformContext]{
				N: "time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 500000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "time_unix_nano",
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "value_double",
			path: &pathtest.Path[TransformContext]{
				N: "value_double",
			},
			orig:   1.1,
			newVal: 2.2,
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetDoubleValue(2.2)
			},
			valueType: pmetric.NumberDataPointValueTypeDouble,
		},
		{
			name: "value_int",
			path: &pathtest.Path[TransformContext]{
				N: "value_int",
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetIntValue(2)
			},
		},
		{
			name: "flags",
			path: &pathtest.Path[TransformContext]{
				N: "flags",
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "exemplars",
			path: &pathtest.Path[TransformContext]{
				N: "exemplars",
			},
			orig:   refNumberDataPoint.Exemplars(),
			newVal: newExemplars,
			modified: func(datapoint pmetric.NumberDataPoint) {
				newExemplars.CopyTo(datapoint.Exemplars())
			},
		},
		{
			name: "attributes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
			},
			orig:   refNumberDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.NumberDataPoint) {
				newAttrs.CopyTo(datapoint.Attributes())
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
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutStr("str", "newVal")
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
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutBool("bool", false)
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
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutInt("int", 20)
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
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutDouble("double", 2.4)
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
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
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
				val, _ := refNumberDataPoint.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
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
				val, _ := refNumberDataPoint.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
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
				val, _ := refNumberDataPoint.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
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
				val, _ := refNumberDataPoint.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
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
				val, _ := refNumberDataPoint.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
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
				val, _ := refNumberDataPoint.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(datapoint pmetric.NumberDataPoint) {
				m := datapoint.Attributes().PutEmptyMap("pMap")
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
				val, _ := refNumberDataPoint.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(datapoint pmetric.NumberDataPoint) {
				m := datapoint.Attributes().PutEmptyMap("map")
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
				val, _ := refNumberDataPoint.Attributes().Get("slice")
				val, _ = val.Slice().At(0).Map().Get("map")
				return val.Str()
			}(),
			newVal: "new",
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
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
			modified: func(datapoint pmetric.NumberDataPoint) {
				s := datapoint.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxdatapoint.Name
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

			numberDataPoint := createNumberDataPointTelemetry(tt.valueType)

			ctx := NewTransformContext(numberDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics(), WithCache(&testCache))

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.NoError(t, err)

			exNumberDataPoint := createNumberDataPointTelemetry(tt.valueType)
			tt.modified(exNumberDataPoint)

			assert.Equal(t, exNumberDataPoint, numberDataPoint)
		})
	}
}

func createNumberDataPointTelemetry(valueType pmetric.NumberDataPointValueType) pmetric.NumberDataPoint {
	numberDataPoint := pmetric.NewNumberDataPoint()
	numberDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	numberDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))

	if valueType == pmetric.NumberDataPointValueTypeDouble {
		numberDataPoint.SetDoubleValue(1.1)
	} else {
		numberDataPoint.SetIntValue(1)
	}

	createAttributeTelemetry(numberDataPoint.Attributes())

	numberDataPoint.Exemplars().AppendEmpty().SetIntValue(0)

	return numberDataPoint
}

func Test_newPathGetSetter_HistogramDataPoint(t *testing.T) {
	refHistogramDataPoint := createHistogramDataPointTelemetry()

	newExemplars, newAttrs := createNewTelemetry()

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newMap := make(map[string]any)
	newMap2 := make(map[string]any)
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		orig     any
		newVal   any
		modified func(pmetric.HistogramDataPoint)
	}{
		{
			name: "start_time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "start_time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "time_unix_nano",
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "flags",
			path: &pathtest.Path[TransformContext]{
				N: "flags",
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "count",
			path: &pathtest.Path[TransformContext]{
				N: "count",
			},
			orig:   int64(2),
			newVal: int64(3),
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetCount(3)
			},
		},
		{
			name: "sum",
			path: &pathtest.Path[TransformContext]{
				N: "sum",
			},
			orig:   10.1,
			newVal: 10.2,
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetSum(10.2)
			},
		},
		{
			name: "bucket_counts",
			path: &pathtest.Path[TransformContext]{
				N: "bucket_counts",
			},
			orig:   []uint64{1, 1},
			newVal: []uint64{1, 2},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.BucketCounts().FromRaw([]uint64{1, 2})
			},
		},
		{
			name: "explicit_bounds",
			path: &pathtest.Path[TransformContext]{
				N: "explicit_bounds",
			},
			orig:   []float64{1, 2},
			newVal: []float64{1, 2, 3},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.ExplicitBounds().FromRaw([]float64{1, 2, 3})
			},
		},
		{
			name: "exemplars",
			path: &pathtest.Path[TransformContext]{
				N: "exemplars",
			},
			orig:   refHistogramDataPoint.Exemplars(),
			newVal: newExemplars,
			modified: func(datapoint pmetric.HistogramDataPoint) {
				newExemplars.CopyTo(datapoint.Exemplars())
			},
		},
		{
			name: "attributes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
			},
			orig:   refHistogramDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.HistogramDataPoint) {
				newAttrs.CopyTo(datapoint.Attributes())
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
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutStr("str", "newVal")
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
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutBool("bool", false)
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
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutInt("int", 20)
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
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutDouble("double", 2.4)
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
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
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
				val, _ := refHistogramDataPoint.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
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
				val, _ := refHistogramDataPoint.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
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
				val, _ := refHistogramDataPoint.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
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
				val, _ := refHistogramDataPoint.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
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
				val, _ := refHistogramDataPoint.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
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
				val, _ := refHistogramDataPoint.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(datapoint pmetric.HistogramDataPoint) {
				m := datapoint.Attributes().PutEmptyMap("pMap")
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
				val, _ := refHistogramDataPoint.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(datapoint pmetric.HistogramDataPoint) {
				m := datapoint.Attributes().PutEmptyMap("map")
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
				val, _ := refHistogramDataPoint.Attributes().Get("slice")
				val, _ = val.Slice().At(0).Map().Get("map")
				return val.Str()
			}(),
			newVal: "new",
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
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
			modified: func(datapoint pmetric.HistogramDataPoint) {
				s := datapoint.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxdatapoint.Name
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

			histogramDataPoint := createHistogramDataPointTelemetry()

			ctx := NewTransformContext(histogramDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics(), WithCache(&testCache))

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.NoError(t, err)

			exNumberDataPoint := createHistogramDataPointTelemetry()
			tt.modified(exNumberDataPoint)

			assert.Equal(t, exNumberDataPoint, histogramDataPoint)
		})
	}
}

func createHistogramDataPointTelemetry() pmetric.HistogramDataPoint {
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	histogramDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	histogramDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	histogramDataPoint.SetCount(2)
	histogramDataPoint.SetSum(10.1)
	histogramDataPoint.BucketCounts().FromRaw([]uint64{1, 1})
	histogramDataPoint.ExplicitBounds().FromRaw([]float64{1, 2})

	createAttributeTelemetry(histogramDataPoint.Attributes())

	histogramDataPoint.Exemplars().AppendEmpty().SetIntValue(0)

	return histogramDataPoint
}

func Test_newPathGetSetter_ExpoHistogramDataPoint(t *testing.T) {
	refExpoHistogramDataPoint := createExpoHistogramDataPointTelemetry()

	newExemplars, newAttrs := createNewTelemetry()

	newPositive := pmetric.NewExponentialHistogramDataPointBuckets()
	newPositive.SetOffset(10)
	newPositive.BucketCounts().FromRaw([]uint64{4, 5})

	newNegative := pmetric.NewExponentialHistogramDataPointBuckets()
	newNegative.SetOffset(10)
	newNegative.BucketCounts().FromRaw([]uint64{4, 5})

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newMap := make(map[string]any)
	newMap2 := make(map[string]any)
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		orig     any
		newVal   any
		modified func(pmetric.ExponentialHistogramDataPoint)
	}{
		{
			name: "start_time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "start_time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "time_unix_nano",
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "flags",
			path: &pathtest.Path[TransformContext]{
				N: "flags",
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "count",
			path: &pathtest.Path[TransformContext]{
				N: "count",
			},
			orig:   int64(2),
			newVal: int64(3),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetCount(3)
			},
		},
		{
			name: "sum",
			path: &pathtest.Path[TransformContext]{
				N: "sum",
			},
			orig:   10.1,
			newVal: 10.2,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetSum(10.2)
			},
		},
		{
			name: "scale",
			path: &pathtest.Path[TransformContext]{
				N: "scale",
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetScale(2)
			},
		},
		{
			name: "zero_count",
			path: &pathtest.Path[TransformContext]{
				N: "zero_count",
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetZeroCount(2)
			},
		},
		{
			name: "positive",
			path: &pathtest.Path[TransformContext]{
				N: "positive",
			},
			orig:   refExpoHistogramDataPoint.Positive(),
			newVal: newPositive,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				newPositive.CopyTo(datapoint.Positive())
			},
		},
		{
			name: "positive offset",
			path: &pathtest.Path[TransformContext]{
				N: "positive",
				NextPath: &pathtest.Path[TransformContext]{
					N: "offset",
				},
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Positive().SetOffset(2)
			},
		},
		{
			name: "positive bucket_counts",
			path: &pathtest.Path[TransformContext]{
				N: "positive",
				NextPath: &pathtest.Path[TransformContext]{
					N: "bucket_counts",
				},
			},
			orig:   []uint64{1, 1},
			newVal: []uint64{0, 1, 2},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Positive().BucketCounts().FromRaw([]uint64{0, 1, 2})
			},
		},
		{
			name: "negative",
			path: &pathtest.Path[TransformContext]{
				N: "negative",
			},
			orig:   refExpoHistogramDataPoint.Negative(),
			newVal: newPositive,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				newPositive.CopyTo(datapoint.Negative())
			},
		},
		{
			name: "negative offset",
			path: &pathtest.Path[TransformContext]{
				N: "negative",
				NextPath: &pathtest.Path[TransformContext]{
					N: "offset",
				},
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Negative().SetOffset(2)
			},
		},
		{
			name: "negative bucket_counts",
			path: &pathtest.Path[TransformContext]{
				N: "negative",
				NextPath: &pathtest.Path[TransformContext]{
					N: "bucket_counts",
				},
			},
			orig:   []uint64{1, 1},
			newVal: []uint64{0, 1, 2},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Negative().BucketCounts().FromRaw([]uint64{0, 1, 2})
			},
		},
		{
			name: "exemplars",
			path: &pathtest.Path[TransformContext]{
				N: "exemplars",
			},
			orig:   refExpoHistogramDataPoint.Exemplars(),
			newVal: newExemplars,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				newExemplars.CopyTo(datapoint.Exemplars())
			},
		},
		{
			name: "attributes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
			},
			orig:   refExpoHistogramDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				newAttrs.CopyTo(datapoint.Attributes())
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
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutStr("str", "newVal")
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
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutBool("bool", false)
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
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutInt("int", 20)
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
			orig:   1.2,
			newVal: 2.4,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutDouble("double", 2.4)
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
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
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
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
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
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
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
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
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
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
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
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
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
				val, _ := refExpoHistogramDataPoint.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				m := datapoint.Attributes().PutEmptyMap("pMap")
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
				val, _ := refExpoHistogramDataPoint.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				m := datapoint.Attributes().PutEmptyMap("map")
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
				val, _ := refExpoHistogramDataPoint.Attributes().Get("slice")
				val, _ = val.Slice().At(0).Map().Get("map")
				return val.Str()
			}(),
			newVal: "new",
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
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
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				s := datapoint.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxdatapoint.Name
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

			expoHistogramDataPoint := createExpoHistogramDataPointTelemetry()

			ctx := NewTransformContext(expoHistogramDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics(), WithCache(&testCache))

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.NoError(t, err)

			exNumberDataPoint := createExpoHistogramDataPointTelemetry()
			tt.modified(exNumberDataPoint)

			assert.Equal(t, exNumberDataPoint, expoHistogramDataPoint)
		})
	}
}

func createExpoHistogramDataPointTelemetry() pmetric.ExponentialHistogramDataPoint {
	expoHistogramDataPoint := pmetric.NewExponentialHistogramDataPoint()
	expoHistogramDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	expoHistogramDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	expoHistogramDataPoint.SetCount(2)
	expoHistogramDataPoint.SetSum(10.1)
	expoHistogramDataPoint.SetScale(1)
	expoHistogramDataPoint.SetZeroCount(1)

	expoHistogramDataPoint.Positive().BucketCounts().FromRaw([]uint64{1, 1})
	expoHistogramDataPoint.Positive().SetOffset(1)

	expoHistogramDataPoint.Negative().BucketCounts().FromRaw([]uint64{1, 1})
	expoHistogramDataPoint.Negative().SetOffset(1)

	createAttributeTelemetry(expoHistogramDataPoint.Attributes())

	expoHistogramDataPoint.Exemplars().AppendEmpty().SetIntValue(0)

	return expoHistogramDataPoint
}

func Test_newPathGetSetter_SummaryDataPoint(t *testing.T) {
	refSummaryDataPoint := createSummaryDataPointTelemetry()

	_, newAttrs := createNewTelemetry()

	newQuartileValues := pmetric.NewSummaryDataPointValueAtQuantileSlice()
	newQuartileValues.AppendEmpty().SetValue(100)

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newMap := make(map[string]any)
	newMap2 := make(map[string]any)
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		orig     any
		newVal   any
		modified func(pmetric.SummaryDataPoint)
	}{
		{
			name: "start_time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "start_time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "time_unix_nano",
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "flags",
			path: &pathtest.Path[TransformContext]{
				N: "flags",
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "count",
			path: &pathtest.Path[TransformContext]{
				N: "count",
			},
			orig:   int64(2),
			newVal: int64(3),
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetCount(3)
			},
		},
		{
			name: "sum",
			path: &pathtest.Path[TransformContext]{
				N: "sum",
			},
			orig:   10.1,
			newVal: 10.2,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetSum(10.2)
			},
		},
		{
			name: "quantile_values",
			path: &pathtest.Path[TransformContext]{
				N: "quantile_values",
			},
			orig:   refSummaryDataPoint.QuantileValues(),
			newVal: newQuartileValues,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				newQuartileValues.CopyTo(datapoint.QuantileValues())
			},
		},
		{
			name: "attributes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
			},
			orig:   refSummaryDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				newAttrs.CopyTo(datapoint.Attributes())
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
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutStr("str", "newVal")
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
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutBool("bool", false)
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
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutInt("int", 20)
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
			orig:   1.2,
			newVal: 2.4,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutDouble("double", 2.4)
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
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
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
				val, _ := refSummaryDataPoint.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
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
				val, _ := refSummaryDataPoint.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
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
				val, _ := refSummaryDataPoint.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
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
				val, _ := refSummaryDataPoint.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
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
				val, _ := refSummaryDataPoint.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
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
				val, _ := refSummaryDataPoint.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				m := datapoint.Attributes().PutEmptyMap("pMap")
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
				val, _ := refSummaryDataPoint.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				m := datapoint.Attributes().PutEmptyMap("map")
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
				val, _ := refSummaryDataPoint.Attributes().Get("slice")
				val, _ = val.Slice().At(0).Map().Get("map")
				return val.Str()
			}(),
			newVal: "new",
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
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
			modified: func(datapoint pmetric.SummaryDataPoint) {
				s := datapoint.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxdatapoint.Name
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

			summaryDataPoint := createSummaryDataPointTelemetry()

			ctx := NewTransformContext(summaryDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics(), WithCache(&testCache))

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.NoError(t, err)

			exNumberDataPoint := createSummaryDataPointTelemetry()
			tt.modified(exNumberDataPoint)

			assert.Equal(t, exNumberDataPoint, summaryDataPoint)
		})
	}
}

func createSummaryDataPointTelemetry() pmetric.SummaryDataPoint {
	summaryDataPoint := pmetric.NewSummaryDataPoint()
	summaryDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	summaryDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	summaryDataPoint.SetCount(2)
	summaryDataPoint.SetSum(10.1)

	summaryDataPoint.QuantileValues().AppendEmpty().SetValue(1)

	createAttributeTelemetry(summaryDataPoint.Attributes())

	return summaryDataPoint
}

func createAttributeTelemetry(attributes pcommon.Map) {
	attributes.PutStr("str", "val")
	attributes.PutBool("bool", true)
	attributes.PutInt("int", 10)
	attributes.PutDouble("double", 1.2)
	attributes.PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	arrStr := attributes.PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStr("one")
	arrStr.AppendEmpty().SetStr("two")

	arrBool := attributes.PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBool(true)
	arrBool.AppendEmpty().SetBool(false)

	arrInt := attributes.PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetInt(2)
	arrInt.AppendEmpty().SetInt(3)

	arrFloat := attributes.PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDouble(1.0)
	arrFloat.AppendEmpty().SetDouble(2.0)

	arrBytes := attributes.PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{2, 3, 4})

	s := attributes.PutEmptySlice("slice")
	s.AppendEmpty().SetEmptyMap().PutStr("map", "pass")

	pMap := attributes.PutEmptyMap("pMap")
	pMap.PutStr("original", "map")

	m := attributes.PutEmptyMap("map")
	m.PutStr("original", "map")
}

func Test_newPathGetSetter_Metric(t *testing.T) {
	newMetric := pmetric.NewMetric()
	newMetric.SetName("new name")

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		orig     any
		newVal   any
		modified func(metric pmetric.Metric)
	}{
		{
			name: "metric name",
			path: &pathtest.Path[TransformContext]{
				N: "metric",
				NextPath: &pathtest.Path[TransformContext]{
					N: "name",
				},
			},
			orig:   "name",
			newVal: "new name",
			modified: func(metric pmetric.Metric) {
				metric.SetName("new name")
			},
		},
		{
			name: "metric description",
			path: &pathtest.Path[TransformContext]{
				N: "metric",
				NextPath: &pathtest.Path[TransformContext]{
					N: "description",
				},
			},
			orig:   "description",
			newVal: "new description",
			modified: func(metric pmetric.Metric) {
				metric.SetDescription("new description")
			},
		},
		{
			name: "metric unit",
			path: &pathtest.Path[TransformContext]{
				N: "metric",
				NextPath: &pathtest.Path[TransformContext]{
					N: "unit",
				},
			},
			orig:   "unit",
			newVal: "new unit",
			modified: func(metric pmetric.Metric) {
				metric.SetUnit("new unit")
			},
		},
		{
			name: "metric type",
			path: &pathtest.Path[TransformContext]{
				N: "metric",
				NextPath: &pathtest.Path[TransformContext]{
					N: "type",
				},
			},
			orig:   int64(pmetric.MetricTypeSum),
			newVal: int64(pmetric.MetricTypeSum),
			modified: func(_ pmetric.Metric) {
			},
		},
		{
			name: "metric aggregation_temporality",
			path: &pathtest.Path[TransformContext]{
				N: "metric",
				NextPath: &pathtest.Path[TransformContext]{
					N: "aggregation_temporality",
				},
			},
			orig:   int64(2),
			newVal: int64(1),
			modified: func(metric pmetric.Metric) {
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			},
		},
		{
			name: "metric is_monotonic",
			path: &pathtest.Path[TransformContext]{
				N: "metric",
				NextPath: &pathtest.Path[TransformContext]{
					N: "is_monotonic",
				},
			},
			orig:   true,
			newVal: false,
			modified: func(metric pmetric.Metric) {
				metric.Sum().SetIsMonotonic(false)
			},
		},
		{
			name: "metric field with context",
			path: &pathtest.Path[TransformContext]{
				C: "metric",
				N: "type",
			},
			orig:   int64(pmetric.MetricTypeSum),
			newVal: int64(pmetric.MetricTypeSum),
			modified: func(_ pmetric.Metric) {
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := pathExpressionParser(getCache)(tt.path)
			assert.NoError(t, err)

			metric := createMetricTelemetry()

			ctx := NewTransformContext(pmetric.NewNumberDataPoint(), metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.NoError(t, err)

			exMetric := createMetricTelemetry()
			tt.modified(exMetric)

			assert.Equal(t, exMetric, metric)
		})
	}
}

func createMetricTelemetry() pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("name")
	metric.SetDescription("description")
	metric.SetUnit("unit")
	metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	metric.Sum().SetIsMonotonic(true)
	return metric
}

func createNewTelemetry() (pmetric.ExemplarSlice, pcommon.Map) {
	newExemplars := pmetric.NewExemplarSlice()
	newExemplars.AppendEmpty().SetIntValue(4)

	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")

	return newExemplars, newAttrs
}

func Test_ParseEnum(t *testing.T) {
	tests := []struct {
		name string
		want ottl.Enum
	}{
		{
			name: "AGGREGATION_TEMPORALITY_UNSPECIFIED",
			want: ottl.Enum(pmetric.AggregationTemporalityUnspecified),
		},
		{
			name: "AGGREGATION_TEMPORALITY_DELTA",
			want: ottl.Enum(pmetric.AggregationTemporalityDelta),
		},
		{
			name: "AGGREGATION_TEMPORALITY_CUMULATIVE",
			want: ottl.Enum(pmetric.AggregationTemporalityCumulative),
		},
		{
			name: "FLAG_NONE",
			want: 0,
		},
		{
			name: "FLAG_NO_RECORDED_VALUE",
			want: 1,
		},
		{
			name: "METRIC_DATA_TYPE_NONE",
			want: ottl.Enum(pmetric.MetricTypeEmpty),
		},
		{
			name: "METRIC_DATA_TYPE_GAUGE",
			want: ottl.Enum(pmetric.MetricTypeGauge),
		},
		{
			name: "METRIC_DATA_TYPE_SUM",
			want: ottl.Enum(pmetric.MetricTypeSum),
		},
		{
			name: "METRIC_DATA_TYPE_HISTOGRAM",
			want: ottl.Enum(pmetric.MetricTypeHistogram),
		},
		{
			name: "METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM",
			want: ottl.Enum(pmetric.MetricTypeExponentialHistogram),
		},
		{
			name: "METRIC_DATA_TYPE_SUMMARY",
			want: ottl.Enum(pmetric.MetricTypeSummary),
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

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("foo", "bar")

	metric := pmetric.NewMetric()
	metric.SetName("metric")

	instrumentationScope := pcommon.NewInstrumentationScope()
	instrumentationScope.SetName("instrumentation_scope")

	ctx := NewTransformContext(
		pmetric.NewNumberDataPoint(),
		metric,
		pmetric.NewMetricSlice(),
		instrumentationScope,
		resource,
		pmetric.NewScopeMetrics(),
		pmetric.NewResourceMetrics())

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
			name:     "metric",
			path:     &pathtest.Path[TransformContext]{N: "metric", NextPath: &pathtest.Path[TransformContext]{N: "name"}},
			expected: metric.Name(),
		},
		{
			name:     "metric with context",
			path:     &pathtest.Path[TransformContext]{C: "metric", N: "name"},
			expected: metric.Name(),
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
