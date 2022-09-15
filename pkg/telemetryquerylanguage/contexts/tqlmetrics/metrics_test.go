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

package tqlmetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
)

func Test_newPathGetSetter_NumberDataPoint(t *testing.T) {
	refNumberDataPoint := createNumberDataPointTelemetry(pmetric.NumberDataPointValueTypeInt)

	newExemplars, newAttrs := createNewTelemetry()

	tests := []struct {
		name      string
		path      []tql.Field
		orig      interface{}
		newVal    interface{}
		modified  func(pmetric.NumberDataPoint)
		valueType pmetric.NumberDataPointValueType
	}{
		{
			name: "start_time_unix_nano",
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name: "value_double",
				},
			},
			orig:   1.1,
			newVal: 2.2,
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetDoubleVal(2.2)
			},
			valueType: pmetric.NumberDataPointValueTypeDouble,
		},
		{
			name: "value_int",
			path: []tql.Field{
				{
					Name: "value_int",
				},
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetIntVal(2)
			},
		},
		{
			name: "flags",
			path: []tql.Field{
				{
					Name: "flags",
				},
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.SetFlags(pmetric.DefaultMetricDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "exemplars",
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name: "attributes",
				},
			},
			orig:   refNumberDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().Clear()
				newAttrs.CopyTo(datapoint.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("str"),
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bool"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("int"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("double"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bytes"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			newVal: []string{"new"},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStringVal("new")
			},
		},
		{
			name: "attributes array bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			newVal: []bool{false},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBoolVal(false)
			},
		},
		{
			name: "attributes array int",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			newVal: []int64{20},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetIntVal(20)
			},
		},
		{
			name: "attributes array float",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			newVal: []float64{2.0},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDoubleVal(2.0)
			},
		},
		{
			name: "attributes array bytes",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refNumberDataPoint.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(datapoint pmetric.NumberDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{9, 6, 4})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			numberDataPoint := createNumberDataPointTelemetry(tt.valueType)

			ctx := NewTransformContext(numberDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.newVal)

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
		numberDataPoint.SetDoubleVal(1.1)
	} else {
		numberDataPoint.SetIntVal(1)
	}

	createAttributeTelemetry(numberDataPoint.Attributes())

	numberDataPoint.Exemplars().AppendEmpty().SetIntVal(0)

	return numberDataPoint
}

func Test_newPathGetSetter_HistogramDataPoint(t *testing.T) {
	refHistogramDataPoint := createHistogramDataPointTelemetry()

	newExemplars, newAttrs := createNewTelemetry()

	tests := []struct {
		name     string
		path     []tql.Field
		orig     interface{}
		newVal   interface{}
		modified func(pmetric.HistogramDataPoint)
	}{
		{
			name: "start_time_unix_nano",
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name: "flags",
				},
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.SetFlags(pmetric.DefaultMetricDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "count",
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name: "attributes",
				},
			},
			orig:   refHistogramDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().Clear()
				newAttrs.CopyTo(datapoint.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("str"),
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bool"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("int"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("double"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bytes"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			newVal: []string{"new"},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStringVal("new")
			},
		},
		{
			name: "attributes array bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			newVal: []bool{false},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBoolVal(false)
			},
		},
		{
			name: "attributes array int",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			newVal: []int64{20},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetIntVal(20)
			},
		},
		{
			name: "attributes array float",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			newVal: []float64{2.0},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDoubleVal(2.0)
			},
		},
		{
			name: "attributes array bytes",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refHistogramDataPoint.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(datapoint pmetric.HistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{9, 6, 4})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			histogramDataPoint := createHistogramDataPointTelemetry()

			ctx := NewTransformContext(histogramDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.newVal)

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

	histogramDataPoint.Exemplars().AppendEmpty().SetIntVal(0)

	return histogramDataPoint
}

func Test_newPathGetSetter_ExpoHistogramDataPoint(t *testing.T) {
	refExpoHistogramDataPoint := createExpoHistogramDataPointTelemetry()

	newExemplars, newAttrs := createNewTelemetry()

	newPositive := pmetric.NewBuckets()
	newPositive.SetOffset(10)
	newPositive.BucketCounts().FromRaw([]uint64{4, 5})

	newNegative := pmetric.NewBuckets()
	newNegative.SetOffset(10)
	newNegative.BucketCounts().FromRaw([]uint64{4, 5})

	tests := []struct {
		name     string
		path     []tql.Field
		orig     interface{}
		newVal   interface{}
		modified func(pmetric.ExponentialHistogramDataPoint)
	}{
		{
			name: "start_time_unix_nano",
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name: "flags",
				},
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.SetFlags(pmetric.DefaultMetricDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "count",
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name: "attributes",
				},
			},
			orig:   refExpoHistogramDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().Clear()
				newAttrs.CopyTo(datapoint.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("str"),
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bool"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("int"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("double"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bytes"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			newVal: []string{"new"},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStringVal("new")
			},
		},
		{
			name: "attributes array bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			newVal: []bool{false},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBoolVal(false)
			},
		},
		{
			name: "attributes array int",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			newVal: []int64{20},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetIntVal(20)
			},
		},
		{
			name: "attributes array float",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			newVal: []float64{2.0},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDoubleVal(2.0)
			},
		},
		{
			name: "attributes array bytes",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(datapoint pmetric.ExponentialHistogramDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{9, 6, 4})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			expoHistogramDataPoint := createExpoHistogramDataPointTelemetry()

			ctx := NewTransformContext(expoHistogramDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.newVal)

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

	expoHistogramDataPoint.Exemplars().AppendEmpty().SetIntVal(0)

	return expoHistogramDataPoint
}

func Test_newPathGetSetter_SummaryDataPoint(t *testing.T) {
	refExpoHistogramDataPoint := createSummaryDataPointTelemetry()

	_, newAttrs := createNewTelemetry()

	newQuartileValues := pmetric.NewValueAtQuantileSlice()
	newQuartileValues.AppendEmpty().SetValue(100)

	tests := []struct {
		name     string
		path     []tql.Field
		orig     interface{}
		newVal   interface{}
		modified func(pmetric.SummaryDataPoint)
	}{
		{
			name: "start_time_unix_nano",
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name: "flags",
				},
			},
			orig:   int64(0),
			newVal: int64(1),
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.SetFlags(pmetric.DefaultMetricDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "count",
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name: "quantile_values",
				},
			},
			orig:   refExpoHistogramDataPoint.QuantileValues(),
			newVal: newQuartileValues,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				newQuartileValues.CopyTo(datapoint.QuantileValues())
			},
		},
		{
			name: "attributes",
			path: []tql.Field{
				{
					Name: "attributes",
				},
			},
			orig:   refExpoHistogramDataPoint.Attributes(),
			newVal: newAttrs,
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().Clear()
				newAttrs.CopyTo(datapoint.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("str"),
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bool"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("int"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("double"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bytes"),
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			newVal: []string{"new"},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStringVal("new")
			},
		},
		{
			name: "attributes array bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			newVal: []bool{false},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBoolVal(false)
			},
		},
		{
			name: "attributes array int",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			newVal: []int64{20},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetIntVal(20)
			},
		},
		{
			name: "attributes array float",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			newVal: []float64{2.0},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDoubleVal(2.0)
			},
		},
		{
			name: "attributes array bytes",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refExpoHistogramDataPoint.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(datapoint pmetric.SummaryDataPoint) {
				datapoint.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{9, 6, 4})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			summaryDataPoint := createSummaryDataPointTelemetry()

			ctx := NewTransformContext(summaryDataPoint, pmetric.NewMetric(), pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.newVal)

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
	attributes.PutString("str", "val")
	attributes.PutBool("bool", true)
	attributes.PutInt("int", 10)
	attributes.PutDouble("double", 1.2)
	attributes.PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	arrStr := attributes.PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStringVal("one")
	arrStr.AppendEmpty().SetStringVal("two")

	arrBool := attributes.PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBoolVal(true)
	arrBool.AppendEmpty().SetBoolVal(false)

	arrInt := attributes.PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetIntVal(2)
	arrInt.AppendEmpty().SetIntVal(3)

	arrFloat := attributes.PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDoubleVal(1.0)
	arrFloat.AppendEmpty().SetDoubleVal(2.0)

	arrBytes := attributes.PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{2, 3, 4})
}

func Test_newPathGetSetter_Metric(t *testing.T) {
	refMetric := createMetricTelemetry()

	newMetric := pmetric.NewMetric()
	newMetric.SetName("new name")

	tests := []struct {
		name     string
		path     []tql.Field
		orig     interface{}
		newVal   interface{}
		modified func(metric pmetric.Metric)
	}{
		{
			name: "metric",
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name: "metric",
				},
				{
					Name: "type",
				},
			},
			orig:   int64(pmetric.MetricDataTypeSum),
			newVal: int64(pmetric.MetricDataTypeSum),
			modified: func(metric pmetric.Metric) {
			},
		},
		{
			name: "metric aggregation_temporality",
			path: []tql.Field{
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
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
			},
		},
		{
			name: "metric is_monotonic",
			path: []tql.Field{
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

			got := accessor.Get(ctx)
			assert.Equal(t, tt.orig, got)

			accessor.Set(ctx, tt.newVal)

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
	metric.SetEmptySum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	metric.Sum().SetIsMonotonic(true)
	return metric
}

func createNewTelemetry() (pmetric.ExemplarSlice, pcommon.Map) {
	newExemplars := pmetric.NewExemplarSlice()
	newExemplars.AppendEmpty().SetIntVal(4)

	newAttrs := pcommon.NewMap()
	newAttrs.PutString("hello", "world")

	return newExemplars, newAttrs
}

func Test_ParseEnum(t *testing.T) {
	tests := []struct {
		name string
		want tql.Enum
	}{
		{
			name: "AGGREGATION_TEMPORALITY_UNSPECIFIED",
			want: tql.Enum(pmetric.MetricAggregationTemporalityUnspecified),
		},
		{
			name: "AGGREGATION_TEMPORALITY_DELTA",
			want: tql.Enum(pmetric.MetricAggregationTemporalityDelta),
		},
		{
			name: "AGGREGATION_TEMPORALITY_CUMULATIVE",
			want: tql.Enum(pmetric.MetricAggregationTemporalityCumulative),
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
			want: tql.Enum(pmetric.MetricDataTypeNone),
		},
		{
			name: "METRIC_DATA_TYPE_GAUGE",
			want: tql.Enum(pmetric.MetricDataTypeGauge),
		},
		{
			name: "METRIC_DATA_TYPE_SUM",
			want: tql.Enum(pmetric.MetricDataTypeSum),
		},
		{
			name: "METRIC_DATA_TYPE_HISTOGRAM",
			want: tql.Enum(pmetric.MetricDataTypeHistogram),
		},
		{
			name: "METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM",
			want: tql.Enum(pmetric.MetricDataTypeExponentialHistogram),
		},
		{
			name: "METRIC_DATA_TYPE_SUMMARY",
			want: tql.Enum(pmetric.MetricDataTypeSummary),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ParseEnum((*tql.EnumSymbol)(tqltest.Strp(tt.name)))
			assert.NoError(t, err)
			assert.Equal(t, *actual, tt.want)
		})
	}
}

func Test_ParseEnum_False(t *testing.T) {
	tests := []struct {
		name       string
		enumSymbol *tql.EnumSymbol
	}{
		{
			name:       "unknown enum symbol",
			enumSymbol: (*tql.EnumSymbol)(tqltest.Strp("not an enum")),
		},
		{
			name:       "nil enum symbol",
			enumSymbol: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ParseEnum(tt.enumSymbol)
			assert.Error(t, err)
			assert.Nil(t, actual)
		})
	}
}
