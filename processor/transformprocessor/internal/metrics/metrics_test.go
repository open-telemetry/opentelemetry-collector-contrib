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
	refNumberDataPoint, _, _, _ := createNumberDataPointTelemetry(pmetric.NumberDataPointValueTypeInt)

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

	tests := []struct {
		name      string
		path      []common.Field
		orig      interface{}
		new       interface{}
		modified  func(interface{}, pmetric.Metric, pcommon.InstrumentationScope, pcommon.Resource)
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
			valueType: pmetric.NumberDataPointValueTypeInt,
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
			valueType: pmetric.NumberDataPointValueTypeInt,
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetIntVal(2)
			},
			valueType: pmetric.NumberDataPointValueTypeInt,
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetFlags(pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))
			},
			valueType: pmetric.NumberDataPointValueTypeInt,
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).SetFlags(pmetric.NewMetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))
			},
			valueType: pmetric.NumberDataPointValueTypeInt,
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				newExemplars.CopyTo(datapoint.(pmetric.NumberDataPoint).Exemplars())
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
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
			modified: func(datapoint interface{}, metric pmetric.Metric, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				datapoint.(pmetric.NumberDataPoint).Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			numberDataPoint, metric, il, resource := createNumberDataPointTelemetry(tt.valueType)

			ctx := metricTransformContext{
				dataPoint: numberDataPoint,
				metric:    metric,
				il:        il,
				resource:  resource,
			}

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.new)

			exNumberDataPoint, exMetric, exIl, exResource := createNumberDataPointTelemetry(tt.valueType)
			tt.modified(exNumberDataPoint, exMetric, exIl, exResource)

			assert.Equal(t, exNumberDataPoint, numberDataPoint)
			assert.Equal(t, exMetric, metric)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exResource, resource)
		})
	}
}

func createNumberDataPointTelemetry(valueType pmetric.NumberDataPointValueType) (interface{}, pmetric.Metric, pcommon.InstrumentationScope, pcommon.Resource) {
	numberDataPoint := pmetric.NewNumberDataPoint()
	numberDataPoint.SetFlags(pmetric.NewMetricDataPointFlags())
	numberDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	numberDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))

	if valueType == pmetric.NumberDataPointValueTypeInt {
		numberDataPoint.SetIntVal(1)
	} else {
		numberDataPoint.SetDoubleVal(1.1)
	}

	numberDataPoint.Attributes().UpsertString("str", "val")
	numberDataPoint.Attributes().UpsertBool("bool", true)
	numberDataPoint.Attributes().UpsertInt("int", 10)
	numberDataPoint.Attributes().UpsertDouble("double", 1.2)
	numberDataPoint.Attributes().UpsertBytes("bytes", []byte{1, 3, 2})

	arrStr := pcommon.NewValueSlice()
	arrStr.SliceVal().AppendEmpty().SetStringVal("one")
	arrStr.SliceVal().AppendEmpty().SetStringVal("two")
	numberDataPoint.Attributes().Upsert("arr_str", arrStr)

	arrBool := pcommon.NewValueSlice()
	arrBool.SliceVal().AppendEmpty().SetBoolVal(true)
	arrBool.SliceVal().AppendEmpty().SetBoolVal(false)
	numberDataPoint.Attributes().Upsert("arr_bool", arrBool)

	arrInt := pcommon.NewValueSlice()
	arrInt.SliceVal().AppendEmpty().SetIntVal(2)
	arrInt.SliceVal().AppendEmpty().SetIntVal(3)
	numberDataPoint.Attributes().Upsert("arr_int", arrInt)

	arrFloat := pcommon.NewValueSlice()
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(1.0)
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)
	numberDataPoint.Attributes().Upsert("arr_float", arrFloat)

	arrBytes := pcommon.NewValueSlice()
	arrBytes.SliceVal().AppendEmpty().SetBytesVal([]byte{1, 2, 3})
	arrBytes.SliceVal().AppendEmpty().SetBytesVal([]byte{2, 3, 4})
	numberDataPoint.Attributes().Upsert("arr_bytes", arrBytes)

	numberDataPoint.Exemplars().AppendEmpty().SetIntVal(0)

	il := pcommon.NewInstrumentationScope()
	il.SetName("library")
	il.SetVersion("version")

	resource := pcommon.NewResource()
	numberDataPoint.Attributes().CopyTo(resource.Attributes())

	metric := pmetric.NewMetric()
	metric.SetName("name")
	metric.SetDescription("description")
	metric.SetUnit("unit")
	metric.SetDataType(pmetric.MetricDataTypeSum)
	numberDataPoint.CopyTo(metric.Sum().DataPoints().AppendEmpty())

	return numberDataPoint, metric, il, resource
}

func strp(s string) *string {
	return &s
}

func intp(i int64) *int64 {
	return &i
}
