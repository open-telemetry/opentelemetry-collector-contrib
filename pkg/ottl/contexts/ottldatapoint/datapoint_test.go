// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottldatapoint

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter_Cache(t *testing.T) {
	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	tests := []struct {
		name      string
		path      []ottl.field
		orig      interface{}
		newVal    interface{}
		modified  func(cache pcommon.Map)
		valueType pmetric.NumberDataPointValueType
	}{

		{
			name: "cache",
			path: []ottl.field{
				{
					Name: "cache",
				},
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
		},
		{
			name: "cache access",
			path: []ottl.field{
				{
					Name: "cache",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("temp"),
						},
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			numberDataPoint := createNumberDataPointTelemetry(tt.valueType)

			ctx := NewTransformContext(numberDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got, err := accessor.Get(context.Background(), ctx)
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.Nil(t, err)

			exCache := pcommon.NewMap()
			tt.modified(exCache)

			assert.Equal(t, exCache, ctx.getCache())
		})
	}
}

func Test_newPathGetSetter_NumberDataPoint(t *testing.T) {
	refNumberDataPoint := createNumberDataPointTelemetry(pmetric.NumberDataPointValueTypeInt)

	newExemplars, newAttrs := createNewTelemetry()

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newMap := make(map[string]interface{})
	newMap2 := make(map[string]interface{})
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name      string
		path      []ottl.field
		orig      interface{}
		newVal    interface{}
		modified  func(pmetric.NumberDataPoint)
		valueType pmetric.NumberDataPointValueType
	}{
		{
			name: "start_time_unix_nano",
			path: []ottl.field{
				{
					Name: "start_time_unix_nano",
				},
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: []ottl.field{
				{
					Name: "time_unix_nano",
				},
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "value_double",
			path: []ottl.field{
				{
					Name: "value_double",
				},
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
			path: []ottl.field{
				{
					Name: "value_int",
				},
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetIntValue(2)
			},
		},
		{
			name: "flags",
			path: []ottl.field{
				{
					Name: "flags",
				},
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "exemplars",
			path: []ottl.field{
				{
					Name: "exemplars",
				},
			},
			orig:   refNumberDataPoint.Exemplars(),
			newVal: newExemplars,
			modified: func(datapoint pmetric.NumberDataPoint) {
				newExemplars.CopyTo(datapoint.Exemplars())
			},
		},
		{
			name: "attributes",
			path: []ottl.field{
				{
					Name: "attributes",
				},
			},
			orig:   refNumberDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.NumberDataPoint) {
				newAttrs.CopyTo(datapoint.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("str"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("bool"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("int"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("double"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("bytes"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_str"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_bool"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_int"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_float"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_bytes"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("pMap"),
						},
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
			name: "attributes map[string]interface{}",
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("map"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("slice"),
						},
						{
							Int: ottltest.Intp(0),
						},
						{
							String: ottltest.Strp("map"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("new"),
						},
						{
							Int: ottltest.Intp(2),
						},
						{
							Int: ottltest.Intp(0),
						},
					},
				},
			},
			orig: func() interface{} {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			numberDataPoint := createNumberDataPointTelemetry(tt.valueType)

			ctx := NewTransformContext(numberDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got, err := accessor.Get(context.Background(), ctx)
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.Nil(t, err)

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

	newMap := make(map[string]interface{})
	newMap2 := make(map[string]interface{})
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name     string
		path     []ottl.field
		orig     interface{}
		newVal   interface{}
		modified func(pmetric.HistogramDataPoint)
	}{
		{
			name: "start_time_unix_nano",
			path: []ottl.field{
				{
					Name: "start_time_unix_nano",
				},
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: []ottl.field{
				{
					Name: "time_unix_nano",
				},
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "flags",
			path: []ottl.field{
				{
					Name: "flags",
				},
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "count",
			path: []ottl.field{
				{
					Name: "count",
				},
			},
			orig:   int64(2),
			newVal: int64(3),
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetCount(3)
			},
		},
		{
			name: "sum",
			path: []ottl.field{
				{
					Name: "sum",
				},
			},
			orig:   10.1,
			newVal: 10.2,
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetSum(10.2)
			},
		},
		{
			name: "bucket_counts",
			path: []ottl.field{
				{
					Name: "bucket_counts",
				},
			},
			orig:   []uint64{1, 1},
			newVal: []uint64{1, 2},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.BucketCounts().FromRaw([]uint64{1, 2})
			},
		},
		{
			name: "explicit_bounds",
			path: []ottl.field{
				{
					Name: "explicit_bounds",
				},
			},
			orig:   []float64{1, 2},
			newVal: []float64{1, 2, 3},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.ExplicitBounds().FromRaw([]float64{1, 2, 3})
			},
		},
		{
			name: "exemplars",
			path: []ottl.field{
				{
					Name: "exemplars",
				},
			},
			orig:   refHistogramDataPoint.Exemplars(),
			newVal: newExemplars,
			modified: func(datapoint pmetric.HistogramDataPoint) {
				newExemplars.CopyTo(datapoint.Exemplars())
			},
		},
		{
			name: "attributes",
			path: []ottl.field{
				{
					Name: "attributes",
				},
			},
			orig:   refHistogramDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.HistogramDataPoint) {
				newAttrs.CopyTo(datapoint.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("str"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("bool"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("int"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("double"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("bytes"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_str"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_bool"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_int"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_float"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_bytes"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("pMap"),
						},
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
			name: "attributes map[string]interface{}",
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("map"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("slice"),
						},
						{
							Int: ottltest.Intp(0),
						},
						{
							String: ottltest.Strp("map"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("new"),
						},
						{
							Int: ottltest.Intp(2),
						},
						{
							Int: ottltest.Intp(0),
						},
					},
				},
			},
			orig: func() interface{} {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			histogramDataPoint := createHistogramDataPointTelemetry()

			ctx := NewTransformContext(histogramDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got, err := accessor.Get(context.Background(), ctx)
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.Nil(t, err)

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

	newMap := make(map[string]interface{})
	newMap2 := make(map[string]interface{})
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name     string
		path     []ottl.field
		orig     interface{}
		newVal   interface{}
		modified func(pmetric.ExponentialHistogramDataPoint)
	}{
		{
			name: "start_time_unix_nano",
			path: []ottl.field{
				{
					Name: "start_time_unix_nano",
				},
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: []ottl.field{
				{
					Name: "time_unix_nano",
				},
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "flags",
			path: []ottl.field{
				{
					Name: "flags",
				},
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "count",
			path: []ottl.field{
				{
					Name: "count",
				},
			},
			orig:   int64(2),
			newVal: int64(3),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetCount(3)
			},
		},
		{
			name: "sum",
			path: []ottl.field{
				{
					Name: "sum",
				},
			},
			orig:   10.1,
			newVal: 10.2,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetSum(10.2)
			},
		},
		{
			name: "scale",
			path: []ottl.field{
				{
					Name: "scale",
				},
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetScale(2)
			},
		},
		{
			name: "zero_count",
			path: []ottl.field{
				{
					Name: "zero_count",
				},
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetZeroCount(2)
			},
		},
		{
			name: "positive",
			path: []ottl.field{
				{
					Name: "positive",
				},
			},
			orig:   refExpoHistogramDataPoint.Positive(),
			newVal: newPositive,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				newPositive.CopyTo(datapoint.Positive())
			},
		},
		{
			name: "positive offset",
			path: []ottl.field{
				{
					Name: "positive",
				},
				{
					Name: "offset",
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
			path: []ottl.field{
				{
					Name: "positive",
				},
				{
					Name: "bucket_counts",
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
			path: []ottl.field{
				{
					Name: "negative",
				},
			},
			orig:   refExpoHistogramDataPoint.Negative(),
			newVal: newPositive,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				newPositive.CopyTo(datapoint.Negative())
			},
		},
		{
			name: "negative offset",
			path: []ottl.field{
				{
					Name: "negative",
				},
				{
					Name: "offset",
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
			path: []ottl.field{
				{
					Name: "negative",
				},
				{
					Name: "bucket_counts",
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
			path: []ottl.field{
				{
					Name: "exemplars",
				},
			},
			orig:   refExpoHistogramDataPoint.Exemplars(),
			newVal: newExemplars,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				newExemplars.CopyTo(datapoint.Exemplars())
			},
		},
		{
			name: "attributes",
			path: []ottl.field{
				{
					Name: "attributes",
				},
			},
			orig:   refExpoHistogramDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				newAttrs.CopyTo(datapoint.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("str"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("bool"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("int"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("double"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("bytes"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_str"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_bool"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_int"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_float"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_bytes"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("pMap"),
						},
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
			name: "attributes map[string]interface{}",
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("map"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("slice"),
						},
						{
							Int: ottltest.Intp(0),
						},
						{
							String: ottltest.Strp("map"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("new"),
						},
						{
							Int: ottltest.Intp(2),
						},
						{
							Int: ottltest.Intp(0),
						},
					},
				},
			},
			orig: func() interface{} {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			expoHistogramDataPoint := createExpoHistogramDataPointTelemetry()

			ctx := NewTransformContext(expoHistogramDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got, err := accessor.Get(context.Background(), ctx)
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.Nil(t, err)

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

	newMap := make(map[string]interface{})
	newMap2 := make(map[string]interface{})
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name     string
		path     []ottl.field
		orig     interface{}
		newVal   interface{}
		modified func(pmetric.SummaryDataPoint)
	}{
		{
			name: "start_time_unix_nano",
			path: []ottl.field{
				{
					Name: "start_time_unix_nano",
				},
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: []ottl.field{
				{
					Name: "time_unix_nano",
				},
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "flags",
			path: []ottl.field{
				{
					Name: "flags",
				},
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "count",
			path: []ottl.field{
				{
					Name: "count",
				},
			},
			orig:   int64(2),
			newVal: int64(3),
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetCount(3)
			},
		},
		{
			name: "sum",
			path: []ottl.field{
				{
					Name: "sum",
				},
			},
			orig:   10.1,
			newVal: 10.2,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetSum(10.2)
			},
		},
		{
			name: "quantile_values",
			path: []ottl.field{
				{
					Name: "quantile_values",
				},
			},
			orig:   refSummaryDataPoint.QuantileValues(),
			newVal: newQuartileValues,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				newQuartileValues.CopyTo(datapoint.QuantileValues())
			},
		},
		{
			name: "attributes",
			path: []ottl.field{
				{
					Name: "attributes",
				},
			},
			orig:   refSummaryDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				newAttrs.CopyTo(datapoint.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("str"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("bool"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("int"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("double"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("bytes"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_str"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_bool"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_int"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_float"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("arr_bytes"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("pMap"),
						},
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
			name: "attributes map[string]interface{}",
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("map"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("slice"),
						},
						{
							Int: ottltest.Intp(0),
						},
						{
							String: ottltest.Strp("map"),
						},
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
			path: []ottl.field{
				{
					Name: "attributes",
					Keys: []ottl.key{
						{
							String: ottltest.Strp("new"),
						},
						{
							Int: ottltest.Intp(2),
						},
						{
							Int: ottltest.Intp(0),
						},
					},
				},
			},
			orig: func() interface{} {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			summaryDataPoint := createSummaryDataPointTelemetry()

			ctx := NewTransformContext(summaryDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got, err := accessor.Get(context.Background(), ctx)
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.Nil(t, err)

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
	refMetric := createMetricTelemetry()

	newMetric := pmetric.NewMetric()
	newMetric.SetName("new name")

	tests := []struct {
		name     string
		path     []ottl.field
		orig     interface{}
		newVal   interface{}
		modified func(metric pmetric.Metric)
	}{
		{
			name: "metric",
			path: []ottl.field{
				{
					Name: "metric",
				},
			},
			orig:   refMetric,
			newVal: newMetric,
			modified: func(metric pmetric.Metric) {
				newMetric.CopyTo(metric)
			},
		},
		{
			name: "metric name",
			path: []ottl.field{
				{
					Name: "metric",
				},
				{
					Name: "name",
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
			path: []ottl.field{
				{
					Name: "metric",
				},
				{
					Name: "description",
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
			path: []ottl.field{
				{
					Name: "metric",
				},
				{
					Name: "unit",
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
			path: []ottl.field{
				{
					Name: "metric",
				},
				{
					Name: "type",
				},
			},
			orig:   int64(pmetric.MetricTypeSum),
			newVal: int64(pmetric.MetricTypeSum),
			modified: func(metric pmetric.Metric) {
			},
		},
		{
			name: "metric aggregation_temporality",
			path: []ottl.field{
				{
					Name: "metric",
				},
				{
					Name: "aggregation_temporality",
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
			path: []ottl.field{
				{
					Name: "metric",
				},
				{
					Name: "is_monotonic",
				},
			},
			orig:   true,
			newVal: false,
			modified: func(metric pmetric.Metric) {
				metric.Sum().SetIsMonotonic(false)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			metric := createMetricTelemetry()

			ctx := NewTransformContext(pmetric.NewNumberDataPoint(), metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got, err := accessor.Get(context.Background(), ctx)
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.Nil(t, err)

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
			actual, err := parseEnum((*ottl.enumSymbol)(ottltest.Strp(tt.name)))
			assert.NoError(t, err)
			assert.Equal(t, *actual, tt.want)
		})
	}
}

func Test_ParseEnum_False(t *testing.T) {
	tests := []struct {
		name       string
		enumSymbol *ottl.enumSymbol
	}{
		{
			name:       "unknown enum symbol",
			enumSymbol: (*ottl.enumSymbol)(ottltest.Strp("not an enum")),
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
