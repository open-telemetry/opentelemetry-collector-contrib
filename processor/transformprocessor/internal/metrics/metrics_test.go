// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

var (
	traceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func Test_newPathGetSetter_NumberDataPoint(t *testing.T) {
	refNumberDataPoint, _, _ := createNumberDataPointTelemetry(pmetric.NumberDataPointValueTypeInt)

	newExemplars, newAttrs, newArrStr, newArrBool, newArrInt, newArrFloat, newArrBytes := createNewTelemetry()

	tests := []struct {
		name      string
		path      []common.Field
		orig      interface{}
		new       interface{}
		modified  func(interface{}, pcommon.InstrumentationScope, pcommon.Resource)
		valueType pmetric.NumberDataPointValueType
	}{
		{
			name: "start_time_unix_nano",
			path: []common.Field{
				{
					Name: "start_time_unix_nano",
				},
			},
			orig: int64(100_000_000),
			new:  int64(200_000_000),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: []common.Field{
				{
					Name: "time_unix_nano",
				},
			},
			orig: int64(500_000_000),
			new:  int64(200_000_000),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "value_double",
			path: []common.Field{
				{
					Name: "value_double",
				},
			},
			orig: 1.1,
			new:  2.2,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetDoubleVal(2.2)
			},
			valueType: pmetric.NumberDataPointValueTypeDouble,
		},
		{
			name: "value_int",
			path: []common.Field{
				{
					Name: "value_int",
				},
			},
			orig: int64(1),
			new:  int64(2),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetIntVal(2)
			},
		},
		{
			name: "flags",
			path: []common.Field{
				{
					Name: "flags",
				},
			},
			orig: pmetric.NewMetricDataPointFlags(),
			new:  pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetFlags(pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))
			},
		},
		{
			name: "exemplars",
			path: []common.Field{
				{
					Name: "exemplars",
				},
			},
			orig: refNumberDataPoint.(pmetric.NumberDataPoint).Exemplars(),
			new:  newExemplars,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				newExemplars.CopyTo(datapoint.(pmetric.NumberDataPoint).Exemplars())
			},
		},
		{
			name: "attributes",
			path: []common.Field{
				{
					Name: "attributes",
				},
			},
			orig: refNumberDataPoint.(pmetric.NumberDataPoint).Attributes(),
			new:  newAttrs,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().Clear()
				newAttrs.CopyTo(datapoint.(pmetric.NumberDataPoint).Attributes())
			},
		},
		{
			name: "attributes string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("str"),
				},
			},
			orig: "val",
			new:  "newVal",
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().UpsertString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bool"),
				},
			},
			orig: true,
			new:  false,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().UpsertBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("int"),
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().UpsertInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("double"),
				},
			},
			orig: float64(1.2),
			new:  float64(2.4),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().UpsertDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bytes"),
				},
			},
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().UpsertBytes("bytes", []byte{2, 3, 4})
			},
		},
		{
			name: "attributes array string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.(pmetric.NumberDataPoint).Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			new: []string{"new"},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().Upsert("arr_str", newArrStr)
			},
		},
		{
			name: "attributes array bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.(pmetric.NumberDataPoint).Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			new: []bool{false},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().Upsert("arr_bool", newArrBool)
			},
		},
		{
			name: "attributes array int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.(pmetric.NumberDataPoint).Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			new: []int64{20},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().Upsert("arr_int", newArrInt)
			},
		},
		{
			name: "attributes array float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.(pmetric.NumberDataPoint).Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			new: []float64{2.0},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().Upsert("arr_float", newArrFloat)
			},
		},
		{
			name: "attributes array bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.(pmetric.NumberDataPoint).Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			new: [][]byte{{9, 6, 4}},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			numberDataPoint, il, resource := createNumberDataPointTelemetry(tt.valueType)

			ctx := metricTransformContext{
				dataPoint: numberDataPoint,
				metric:    pmetric.NewMetric(),
				il:        il,
				resource:  resource,
			}

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.new)

			exNumberDataPoint, exIl, exResource := createNumberDataPointTelemetry(tt.valueType)
			tt.modified(exNumberDataPoint, exIl, exResource)

			assert.Equal(t, exNumberDataPoint, numberDataPoint)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exResource, resource)
		})
	}
}

func createNumberDataPointTelemetry(valueType pmetric.NumberDataPointValueType) (interface{}, pcommon.InstrumentationScope, pcommon.Resource) {
	numberDataPoint := pmetric.NewNumberDataPoint()
	numberDataPoint.SetFlags(pmetric.NewMetricDataPointFlags())
	numberDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	numberDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))

	if valueType == pmetric.NumberDataPointValueTypeDouble {
		numberDataPoint.SetDoubleVal(1.1)
	} else {
		numberDataPoint.SetIntVal(1)
	}

	createAttributeTelemetry(numberDataPoint.Attributes())

	numberDataPoint.Exemplars().AppendEmpty().SetIntVal(0)

	resource := pcommon.NewResource()
	numberDataPoint.Attributes().CopyTo(resource.Attributes())

	//metric := pmetric.NewMetric()
	//metric.SetName("name")
	//metric.SetDescription("description")
	//metric.SetUnit("unit")
	//metric.SetDataType(pmetric.MetricDataTypeSum)
	//numberDataPoint.CopyTo(metric.Sum().DataPoints().AppendEmpty())

	return numberDataPoint, createIlTelemetry(), resource
}

func Test_newPathGetSetter_HistogramDataPoint(t *testing.T) {
	refHistogramDataPoint, _, _ := createHistogramDataPointTelemetry()

	newExemplars, newAttrs, newArrStr, newArrBool, newArrInt, newArrFloat, newArrBytes := createNewTelemetry()

	tests := []struct {
		name     string
		path     []common.Field
		orig     interface{}
		new      interface{}
		modified func(interface{}, pcommon.InstrumentationScope, pcommon.Resource)
	}{
		{
			name: "start_time_unix_nano",
			path: []common.Field{
				{
					Name: "start_time_unix_nano",
				},
			},
			orig: int64(100_000_000),
			new:  int64(200_000_000),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: []common.Field{
				{
					Name: "time_unix_nano",
				},
			},
			orig: int64(500_000_000),
			new:  int64(200_000_000),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "flags",
			path: []common.Field{
				{
					Name: "flags",
				},
			},
			orig: pmetric.NewMetricDataPointFlags(),
			new:  pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).SetFlags(pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))
			},
		},
		{
			name: "count",
			path: []common.Field{
				{
					Name: "count",
				},
			},
			orig: uint64(2),
			new:  uint64(3),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).SetCount(3)
			},
		},
		{
			name: "sum",
			path: []common.Field{
				{
					Name: "sum",
				},
			},
			orig: 10.1,
			new:  10.2,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).SetSum(10.2)
			},
		},
		{
			name: "bucket_counts",
			path: []common.Field{
				{
					Name: "bucket_counts",
				},
			},
			orig: []uint64{1, 1},
			new:  []uint64{1, 2},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).SetBucketCounts([]uint64{1, 2})
			},
		},
		{
			name: "explicit_bounds",
			path: []common.Field{
				{
					Name: "explicit_bounds",
				},
			},
			orig: []float64{1, 2},
			new:  []float64{1, 2, 3},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).SetExplicitBounds([]float64{1, 2, 3})
			},
		},
		{
			name: "exemplars",
			path: []common.Field{
				{
					Name: "exemplars",
				},
			},
			orig: refHistogramDataPoint.(pmetric.HistogramDataPoint).Exemplars(),
			new:  newExemplars,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				newExemplars.CopyTo(datapoint.(pmetric.HistogramDataPoint).Exemplars())
			},
		},
		{
			name: "attributes",
			path: []common.Field{
				{
					Name: "attributes",
				},
			},
			orig: refHistogramDataPoint.(pmetric.HistogramDataPoint).Attributes(),
			new:  newAttrs,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().Clear()
				newAttrs.CopyTo(datapoint.(pmetric.HistogramDataPoint).Attributes())
			},
		},
		{
			name: "attributes string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("str"),
				},
			},
			orig: "val",
			new:  "newVal",
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().UpsertString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bool"),
				},
			},
			orig: true,
			new:  false,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().UpsertBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("int"),
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().UpsertInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("double"),
				},
			},
			orig: float64(1.2),
			new:  float64(2.4),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().UpsertDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bytes"),
				},
			},
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().UpsertBytes("bytes", []byte{2, 3, 4})
			},
		},
		{
			name: "attributes array string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.(pmetric.HistogramDataPoint).Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			new: []string{"new"},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().Upsert("arr_str", newArrStr)
			},
		},
		{
			name: "attributes array bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.(pmetric.HistogramDataPoint).Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			new: []bool{false},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().Upsert("arr_bool", newArrBool)
			},
		},
		{
			name: "attributes array int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.(pmetric.HistogramDataPoint).Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			new: []int64{20},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().Upsert("arr_int", newArrInt)
			},
		},
		{
			name: "attributes array float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.(pmetric.HistogramDataPoint).Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			new: []float64{2.0},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().Upsert("arr_float", newArrFloat)
			},
		},
		{
			name: "attributes array bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.(pmetric.HistogramDataPoint).Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			new: [][]byte{{9, 6, 4}},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.HistogramDataPoint).Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			numberDataPoint, il, resource := createHistogramDataPointTelemetry()

			ctx := metricTransformContext{
				dataPoint: numberDataPoint,
				metric:    pmetric.NewMetric(),
				il:        il,
				resource:  resource,
			}

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.new)

			exNumberDataPoint, exIl, exResource := createHistogramDataPointTelemetry()
			tt.modified(exNumberDataPoint, exIl, exResource)

			assert.Equal(t, exNumberDataPoint, numberDataPoint)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exResource, resource)
		})
	}
}

func createHistogramDataPointTelemetry() (interface{}, pcommon.InstrumentationScope, pcommon.Resource) {
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	histogramDataPoint.SetFlags(pmetric.NewMetricDataPointFlags())
	histogramDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	histogramDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	histogramDataPoint.SetCount(2)
	histogramDataPoint.SetSum(10.1)
	histogramDataPoint.SetBucketCounts([]uint64{1, 1})
	histogramDataPoint.SetExplicitBounds([]float64{1, 2})

	createAttributeTelemetry(histogramDataPoint.Attributes())

	histogramDataPoint.Exemplars().AppendEmpty().SetIntVal(0)

	resource := pcommon.NewResource()
	histogramDataPoint.Attributes().CopyTo(resource.Attributes())

	return histogramDataPoint, createIlTelemetry(), resource
}

func Test_newPathGetSetter_ExpoHistogramDataPoint(t *testing.T) {
	refExpoHistogramDataPoint, _, _ := createExpoHistogramDataPointTelemetry()

	newExemplars, newAttrs, newArrStr, newArrBool, newArrInt, newArrFloat, newArrBytes := createNewTelemetry()

	newPositive := pmetric.NewBuckets()
	newPositive.SetOffset(10)
	newPositive.SetBucketCounts([]uint64{4, 5})

	newNegative := pmetric.NewBuckets()
	newNegative.SetOffset(10)
	newNegative.SetBucketCounts([]uint64{4, 5})

	tests := []struct {
		name     string
		path     []common.Field
		orig     interface{}
		new      interface{}
		modified func(interface{}, pcommon.InstrumentationScope, pcommon.Resource)
	}{
		{
			name: "start_time_unix_nano",
			path: []common.Field{
				{
					Name: "start_time_unix_nano",
				},
			},
			orig: int64(100_000_000),
			new:  int64(200_000_000),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: []common.Field{
				{
					Name: "time_unix_nano",
				},
			},
			orig: int64(500_000_000),
			new:  int64(200_000_000),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "flags",
			path: []common.Field{
				{
					Name: "flags",
				},
			},
			orig: pmetric.NewMetricDataPointFlags(),
			new:  pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).SetFlags(pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))
			},
		},
		{
			name: "count",
			path: []common.Field{
				{
					Name: "count",
				},
			},
			orig: uint64(2),
			new:  uint64(3),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).SetCount(3)
			},
		},
		{
			name: "sum",
			path: []common.Field{
				{
					Name: "sum",
				},
			},
			orig: 10.1,
			new:  10.2,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).SetSum(10.2)
			},
		},
		{
			name: "scale",
			path: []common.Field{
				{
					Name: "scale",
				},
			},
			orig: int32(1),
			new:  int32(2),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).SetScale(2)
			},
		},
		{
			name: "zero_count",
			path: []common.Field{
				{
					Name: "zero_count",
				},
			},
			orig: uint64(1),
			new:  uint64(2),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).SetZeroCount(2)
			},
		},
		{
			name: "positive",
			path: []common.Field{
				{
					Name: "positive",
				},
			},
			orig: refExpoHistogramDataPoint.(pmetric.ExponentialHistogramDataPoint).Positive(),
			new:  newPositive,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				newPositive.CopyTo(datapoint.(pmetric.ExponentialHistogramDataPoint).Positive())
			},
		},
		{
			name: "positive offset",
			path: []common.Field{
				{
					Name: "positive",
				},
				{
					Name: "offset",
				},
			},
			orig: int32(1),
			new:  int32(2),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Positive().SetOffset(2)
			},
		},
		{
			name: "positive bucket_counts",
			path: []common.Field{
				{
					Name: "positive",
				},
				{
					Name: "bucket_counts",
				},
			},
			orig: []uint64{1, 1},
			new:  []uint64{0, 1, 2},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Positive().SetBucketCounts([]uint64{0, 1, 2})
			},
		},
		{
			name: "negative",
			path: []common.Field{
				{
					Name: "negative",
				},
			},
			orig: refExpoHistogramDataPoint.(pmetric.ExponentialHistogramDataPoint).Negative(),
			new:  newPositive,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				newPositive.CopyTo(datapoint.(pmetric.ExponentialHistogramDataPoint).Negative())
			},
		},
		{
			name: "negative offset",
			path: []common.Field{
				{
					Name: "negative",
				},
				{
					Name: "offset",
				},
			},
			orig: int32(1),
			new:  int32(2),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Negative().SetOffset(2)
			},
		},
		{
			name: "negative bucket_counts",
			path: []common.Field{
				{
					Name: "negative",
				},
				{
					Name: "bucket_counts",
				},
			},
			orig: []uint64{1, 1},
			new:  []uint64{0, 1, 2},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Negative().SetBucketCounts([]uint64{0, 1, 2})
			},
		},
		{
			name: "exemplars",
			path: []common.Field{
				{
					Name: "exemplars",
				},
			},
			orig: refExpoHistogramDataPoint.(pmetric.ExponentialHistogramDataPoint).Exemplars(),
			new:  newExemplars,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				newExemplars.CopyTo(datapoint.(pmetric.ExponentialHistogramDataPoint).Exemplars())
			},
		},
		{
			name: "attributes",
			path: []common.Field{
				{
					Name: "attributes",
				},
			},
			orig: refExpoHistogramDataPoint.(pmetric.ExponentialHistogramDataPoint).Attributes(),
			new:  newAttrs,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Clear()
				newAttrs.CopyTo(datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes())
			},
		},
		{
			name: "attributes string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("str"),
				},
			},
			orig: "val",
			new:  "newVal",
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().UpsertString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bool"),
				},
			},
			orig: true,
			new:  false,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().UpsertBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("int"),
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().UpsertInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("double"),
				},
			},
			orig: 1.2,
			new:  2.4,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().UpsertDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bytes"),
				},
			},
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().UpsertBytes("bytes", []byte{2, 3, 4})
			},
		},
		{
			name: "attributes array string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			new: []string{"new"},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Upsert("arr_str", newArrStr)
			},
		},
		{
			name: "attributes array bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			new: []bool{false},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Upsert("arr_bool", newArrBool)
			},
		},
		{
			name: "attributes array int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			new: []int64{20},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Upsert("arr_int", newArrInt)
			},
		},
		{
			name: "attributes array float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			new: []float64{2.0},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Upsert("arr_float", newArrFloat)
			},
		},
		{
			name: "attributes array bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			new: [][]byte{{9, 6, 4}},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.ExponentialHistogramDataPoint).Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			numberDataPoint, il, resource := createExpoHistogramDataPointTelemetry()

			ctx := metricTransformContext{
				dataPoint: numberDataPoint,
				metric:    pmetric.NewMetric(),
				il:        il,
				resource:  resource,
			}

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.new)

			exNumberDataPoint, exIl, exResource := createExpoHistogramDataPointTelemetry()
			tt.modified(exNumberDataPoint, exIl, exResource)

			assert.Equal(t, exNumberDataPoint, numberDataPoint)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exResource, resource)
		})
	}
}

func createExpoHistogramDataPointTelemetry() (interface{}, pcommon.InstrumentationScope, pcommon.Resource) {
	expoHistogramDataPoint := pmetric.NewExponentialHistogramDataPoint()
	expoHistogramDataPoint.SetFlags(pmetric.NewMetricDataPointFlags())
	expoHistogramDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	expoHistogramDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	expoHistogramDataPoint.SetCount(2)
	expoHistogramDataPoint.SetSum(10.1)
	expoHistogramDataPoint.SetScale(1)
	expoHistogramDataPoint.SetZeroCount(1)

	expoHistogramDataPoint.Positive().SetBucketCounts([]uint64{1, 1})
	expoHistogramDataPoint.Positive().SetOffset(1)

	expoHistogramDataPoint.Negative().SetBucketCounts([]uint64{1, 1})
	expoHistogramDataPoint.Negative().SetOffset(1)

	createAttributeTelemetry(expoHistogramDataPoint.Attributes())

	expoHistogramDataPoint.Exemplars().AppendEmpty().SetIntVal(0)

	resource := pcommon.NewResource()
	expoHistogramDataPoint.Attributes().CopyTo(resource.Attributes())

	return expoHistogramDataPoint, createIlTelemetry(), resource
}

func Test_newPathGetSetter_SummaryDataPoint(t *testing.T) {
	refExpoHistogramDataPoint, _, _ := createSummaryDataPointTelemetry()

	_, newAttrs, newArrStr, newArrBool, newArrInt, newArrFloat, newArrBytes := createNewTelemetry()

	newQuartileValues := pmetric.NewValueAtQuantileSlice()
	newQuartileValues.AppendEmpty().SetValue(100)

	tests := []struct {
		name     string
		path     []common.Field
		orig     interface{}
		new      interface{}
		modified func(interface{}, pcommon.InstrumentationScope, pcommon.Resource)
	}{
		{
			name: "start_time_unix_nano",
			path: []common.Field{
				{
					Name: "start_time_unix_nano",
				},
			},
			orig: int64(100_000_000),
			new:  int64(200_000_000),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "time_unix_nano",
			path: []common.Field{
				{
					Name: "time_unix_nano",
				},
			},
			orig: int64(500_000_000),
			new:  int64(200_000_000),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "flags",
			path: []common.Field{
				{
					Name: "flags",
				},
			},
			orig: pmetric.NewMetricDataPointFlags(),
			new:  pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).SetFlags(pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))
			},
		},
		{
			name: "count",
			path: []common.Field{
				{
					Name: "count",
				},
			},
			orig: uint64(2),
			new:  uint64(3),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).SetCount(3)
			},
		},
		{
			name: "sum",
			path: []common.Field{
				{
					Name: "sum",
				},
			},
			orig: 10.1,
			new:  10.2,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).SetSum(10.2)
			},
		},
		{
			name: "quantile_values",
			path: []common.Field{
				{
					Name: "quantile_values",
				},
			},
			orig: refExpoHistogramDataPoint.(pmetric.SummaryDataPoint).QuantileValues(),
			new:  newQuartileValues,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				newQuartileValues.CopyTo(datapoint.(pmetric.SummaryDataPoint).QuantileValues())
			},
		},
		{
			name: "attributes",
			path: []common.Field{
				{
					Name: "attributes",
				},
			},
			orig: refExpoHistogramDataPoint.(pmetric.SummaryDataPoint).Attributes(),
			new:  newAttrs,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().Clear()
				newAttrs.CopyTo(datapoint.(pmetric.SummaryDataPoint).Attributes())
			},
		},
		{
			name: "attributes string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("str"),
				},
			},
			orig: "val",
			new:  "newVal",
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().UpsertString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bool"),
				},
			},
			orig: true,
			new:  false,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().UpsertBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("int"),
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().UpsertInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("double"),
				},
			},
			orig: 1.2,
			new:  2.4,
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().UpsertDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("bytes"),
				},
			},
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().UpsertBytes("bytes", []byte{2, 3, 4})
			},
		},
		{
			name: "attributes array string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.SummaryDataPoint).Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			new: []string{"new"},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().Upsert("arr_str", newArrStr)
			},
		},
		{
			name: "attributes array bool",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.SummaryDataPoint).Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			new: []bool{false},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().Upsert("arr_bool", newArrBool)
			},
		},
		{
			name: "attributes array int",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.SummaryDataPoint).Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			new: []int64{20},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().Upsert("arr_int", newArrInt)
			},
		},
		{
			name: "attributes array float",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.SummaryDataPoint).Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			new: []float64{2.0},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().Upsert("arr_float", newArrFloat)
			},
		},
		{
			name: "attributes array bytes",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.(pmetric.SummaryDataPoint).Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			new: [][]byte{{9, 6, 4}},
			modified: func(datapoint interface{}, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.SummaryDataPoint).Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			numberDataPoint, il, resource := createSummaryDataPointTelemetry()

			ctx := metricTransformContext{
				dataPoint: numberDataPoint,
				metric:    pmetric.NewMetric(),
				il:        il,
				resource:  resource,
			}

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.new)

			exNumberDataPoint, exIl, exResource := createSummaryDataPointTelemetry()
			tt.modified(exNumberDataPoint, exIl, exResource)

			assert.Equal(t, exNumberDataPoint, numberDataPoint)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exResource, resource)
		})
	}
}

func createSummaryDataPointTelemetry() (interface{}, pcommon.InstrumentationScope, pcommon.Resource) {
	summaryDataPoint := pmetric.NewSummaryDataPoint()
	summaryDataPoint.SetFlags(pmetric.NewMetricDataPointFlags())
	summaryDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	summaryDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	summaryDataPoint.SetCount(2)
	summaryDataPoint.SetSum(10.1)

	summaryDataPoint.QuantileValues().AppendEmpty().SetValue(1)

	createAttributeTelemetry(summaryDataPoint.Attributes())

	resource := pcommon.NewResource()
	summaryDataPoint.Attributes().CopyTo(resource.Attributes())

	return summaryDataPoint, createIlTelemetry(), resource
}

func createAttributeTelemetry(attributes pcommon.Map) {
	attributes.UpsertString("str", "val")
	attributes.UpsertBool("bool", true)
	attributes.UpsertInt("int", 10)
	attributes.UpsertDouble("double", 1.2)
	attributes.UpsertBytes("bytes", []byte{1, 3, 2})

	arrStr := pcommon.NewValueSlice()
	arrStr.SliceVal().AppendEmpty().SetStringVal("one")
	arrStr.SliceVal().AppendEmpty().SetStringVal("two")
	attributes.Upsert("arr_str", arrStr)

	arrBool := pcommon.NewValueSlice()
	arrBool.SliceVal().AppendEmpty().SetBoolVal(true)
	arrBool.SliceVal().AppendEmpty().SetBoolVal(false)
	attributes.Upsert("arr_bool", arrBool)

	arrInt := pcommon.NewValueSlice()
	arrInt.SliceVal().AppendEmpty().SetIntVal(2)
	arrInt.SliceVal().AppendEmpty().SetIntVal(3)
	attributes.Upsert("arr_int", arrInt)

	arrFloat := pcommon.NewValueSlice()
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(1.0)
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)
	attributes.Upsert("arr_float", arrFloat)

	arrBytes := pcommon.NewValueSlice()
	arrBytes.SliceVal().AppendEmpty().SetBytesVal([]byte{1, 2, 3})
	arrBytes.SliceVal().AppendEmpty().SetBytesVal([]byte{2, 3, 4})
	attributes.Upsert("arr_bytes", arrBytes)
}

func createIlTelemetry() pcommon.InstrumentationScope {
	il := pcommon.NewInstrumentationScope()
	il.SetName("library")
	il.SetVersion("version")
	return il
}

func createNewTelemetry() (pmetric.ExemplarSlice, pcommon.Map, pcommon.Value, pcommon.Value, pcommon.Value, pcommon.Value, pcommon.Value) {
	newExemplars := pmetric.NewExemplarSlice()
	newExemplars.AppendEmpty().SetIntVal(4)

	newAttrs := pcommon.NewMap()
	newAttrs.UpsertString("hello", "world")

	newArrStr := pcommon.NewValueSlice()
	newArrStr.SliceVal().AppendEmpty().SetStringVal("new")

	newArrBool := pcommon.NewValueSlice()
	newArrBool.SliceVal().AppendEmpty().SetBoolVal(false)

	newArrInt := pcommon.NewValueSlice()
	newArrInt.SliceVal().AppendEmpty().SetIntVal(20)

	newArrFloat := pcommon.NewValueSlice()
	newArrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)

	newArrBytes := pcommon.NewValueSlice()
	newArrBytes.SliceVal().AppendEmpty().SetBytesVal([]byte{9, 6, 4})

	return newExemplars, newAttrs, newArrStr, newArrBool, newArrInt, newArrFloat, newArrBytes
}

func strp(s string) *string {
	return &s
}
