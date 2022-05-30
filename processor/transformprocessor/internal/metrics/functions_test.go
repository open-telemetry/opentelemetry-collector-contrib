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

package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_newFunctionCall_NumberDataPoint(t *testing.T) {
	input := pmetric.NewNumberDataPoint()
	attrs := pcommon.NewMap()
	attrs.InsertString("test", "hello world")
	attrs.InsertInt("test2", 3)
	attrs.InsertBool("test3", true)
	attrs.CopyTo(input.Attributes())

	tests := []struct {
		name string
		inv  common.Invocation
		want func(pmetric.NumberDataPoint)
	}{
		{
			name: "set timestamp",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "start_time_unix_nano",
								},
							},
						},
					},
					{
						Int: intp(int64(100_000_000)),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
			},
		},
		{
			name: "keep_keys one",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "keep_keys two",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
					{
						String: strp("test2"),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "keep_keys none",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
			},
		},
		{
			name: "truncate attributes",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "h")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes with zero",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(0),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes nothing",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(100),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes exact",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(11),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate resource attributes",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "resource",
								},
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(11),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes zero",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(0),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes nothing",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(100),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit resource attributes",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "resource",
								},
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.NumberDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataPoint := pmetric.NewNumberDataPoint()
			input.CopyTo(dataPoint)

			evaluate, err := common.NewFunctionCall(tt.inv, DefaultFunctions(), ParsePath)
			assert.NoError(t, err)
			evaluate(metricTransformContext{
				dataPoint: dataPoint,
				il:        pcommon.NewInstrumentationScope(),
				resource:  pcommon.NewResource(),
			})

			expected := pmetric.NewNumberDataPoint()
			tt.want(expected)
			assert.Equal(t, expected, dataPoint)
		})
	}
}

func Test_newFunctionCall_HistogramDataPoint(t *testing.T) {
	input := pmetric.NewHistogramDataPoint()
	attrs := pcommon.NewMap()
	attrs.InsertString("test", "hello world")
	attrs.InsertInt("test2", 3)
	attrs.InsertBool("test3", true)
	attrs.CopyTo(input.Attributes())

	tests := []struct {
		name string
		inv  common.Invocation
		want func(pmetric.HistogramDataPoint)
	}{
		{
			name: "set timestamp",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "start_time_unix_nano",
								},
							},
						},
					},
					{
						Int: intp(int64(100_000_000)),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
			},
		},
		{
			name: "keep_keys one",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "keep_keys two",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
					{
						String: strp("test2"),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "keep_keys none",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
			},
		},
		{
			name: "truncate attributes",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "h")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes with zero",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(0),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes nothing",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(100),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes exact",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(11),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate resource attributes",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "resource",
								},
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(11),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes zero",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(0),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes nothing",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(100),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit resource attributes",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "resource",
								},
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.HistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataPoint := pmetric.NewHistogramDataPoint()
			input.CopyTo(dataPoint)

			evaluate, err := common.NewFunctionCall(tt.inv, DefaultFunctions(), ParsePath)
			assert.NoError(t, err)
			evaluate(metricTransformContext{
				dataPoint: dataPoint,
				il:        pcommon.NewInstrumentationScope(),
				resource:  pcommon.NewResource(),
			})

			expected := pmetric.NewHistogramDataPoint()
			tt.want(expected)
			assert.Equal(t, expected, dataPoint)
		})
	}
}

func Test_newFunctionCall_ExponentialHistogramDataPoint(t *testing.T) {
	input := pmetric.NewExponentialHistogramDataPoint()
	attrs := pcommon.NewMap()
	attrs.InsertString("test", "hello world")
	attrs.InsertInt("test2", 3)
	attrs.InsertBool("test3", true)
	attrs.CopyTo(input.Attributes())

	tests := []struct {
		name string
		inv  common.Invocation
		want func(pmetric.ExponentialHistogramDataPoint)
	}{
		{
			name: "set timestamp",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "start_time_unix_nano",
								},
							},
						},
					},
					{
						Int: intp(int64(100_000_000)),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
			},
		},
		{
			name: "keep_keys one",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "keep_keys two",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
					{
						String: strp("test2"),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "keep_keys none",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
			},
		},
		{
			name: "truncate attributes",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "h")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes with zero",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(0),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes nothing",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(100),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes exact",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(11),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate resource attributes",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "resource",
								},
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(11),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes zero",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(0),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes nothing",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(100),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit resource attributes",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "resource",
								},
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.ExponentialHistogramDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataPoint := pmetric.NewExponentialHistogramDataPoint()
			input.CopyTo(dataPoint)

			evaluate, err := common.NewFunctionCall(tt.inv, DefaultFunctions(), ParsePath)
			assert.NoError(t, err)
			evaluate(metricTransformContext{
				dataPoint: dataPoint,
				il:        pcommon.NewInstrumentationScope(),
				resource:  pcommon.NewResource(),
			})

			expected := pmetric.NewExponentialHistogramDataPoint()
			tt.want(expected)
			assert.Equal(t, expected, dataPoint)
		})
	}
}

func Test_newFunctionCall_SummaryDataPoint(t *testing.T) {
	input := pmetric.NewSummaryDataPoint()
	attrs := pcommon.NewMap()
	attrs.InsertString("test", "hello world")
	attrs.InsertInt("test2", 3)
	attrs.InsertBool("test3", true)
	attrs.CopyTo(input.Attributes())

	tests := []struct {
		name string
		inv  common.Invocation
		want func(pmetric.SummaryDataPoint)
	}{
		{
			name: "set timestamp",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "start_time_unix_nano",
								},
							},
						},
					},
					{
						Int: intp(int64(100_000_000)),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
			},
		},
		{
			name: "keep_keys one",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "keep_keys two",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
					{
						String: strp("test2"),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "keep_keys none",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
			},
		},
		{
			name: "truncate attributes",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "h")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes with zero",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(0),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes nothing",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(100),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate attributes exact",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(11),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "truncate resource attributes",
			inv: common.Invocation{
				Function: "truncate_all",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "resource",
								},
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(11),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes zero",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(0),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit attributes nothing",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(100),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
		{
			name: "limit resource attributes",
			inv: common.Invocation{
				Function: "limit",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "resource",
								},
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(dataPoint pmetric.SummaryDataPoint) {
				input.CopyTo(dataPoint)
				dataPoint.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "hello world")
				attrs.InsertInt("test2", 3)
				attrs.InsertBool("test3", true)
				attrs.CopyTo(dataPoint.Attributes())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataPoint := pmetric.NewSummaryDataPoint()
			input.CopyTo(dataPoint)

			evaluate, err := common.NewFunctionCall(tt.inv, DefaultFunctions(), ParsePath)
			assert.NoError(t, err)
			evaluate(metricTransformContext{
				dataPoint: dataPoint,
				il:        pcommon.NewInstrumentationScope(),
				resource:  pcommon.NewResource(),
			})

			expected := pmetric.NewSummaryDataPoint()
			tt.want(expected)
			assert.Equal(t, expected, dataPoint)
		})
	}
}

func Test_newFunctionCall_Metric(t *testing.T) {
	input := pmetric.NewMetric()
	input.SetName("Starting Name")

	tests := []struct {
		name string
		inv  common.Invocation
		want func(pmetric.Metric)
	}{
		{
			name: "set name",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "metric",
								},
								{
									Name: "name",
								},
							},
						},
					},
					{
						String: strp("ending name"),
					},
				},
			},
			want: func(metric pmetric.Metric) {
				input.CopyTo(metric)
				metric.SetName("ending name")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			input.CopyTo(metric)

			evaluate, err := common.NewFunctionCall(tt.inv, DefaultFunctions(), ParsePath)
			assert.NoError(t, err)
			evaluate(metricTransformContext{
				metric:   metric,
				il:       pcommon.NewInstrumentationScope(),
				resource: pcommon.NewResource(),
			})

			expected := pmetric.NewMetric()
			tt.want(expected)
			assert.Equal(t, expected, metric)
		})
	}
}
