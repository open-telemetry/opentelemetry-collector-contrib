// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signalfxreceiver

import (
	"strconv"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

func Test_signalFxV2ToMetricsData(t *testing.T) {
	now := time.Now()
	msec := now.Unix() * 1e3

	buildDefaulstSFxDataPt := func() *sfxpb.DataPoint {
		return &sfxpb.DataPoint{
			Metric:    strPtr("single"),
			Timestamp: &msec,
			Value: &sfxpb.Datum{
				IntValue: int64Ptr(13),
			},
			MetricType: sfxTypePtr(sfxpb.MetricType_GAUGE),
			Dimensions: buildNDimensions(3),
		}
	}

	buildDefaultMetricsData := func() *consumerdata.MetricsData {
		return &consumerdata.MetricsData{
			Metrics: []*metricspb.Metric{{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "single",
					Type: metricspb.MetricDescriptor_GAUGE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "k0"}, {Key: "k1"}, {Key: "k2"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{
								Value:    "v0",
								HasValue: true,
							},
							{
								Value:    "v1",
								HasValue: true,
							},
							{
								Value:    "v2",
								HasValue: true,
							},
						},
						Points: []*metricspb.Point{{
							Timestamp: &timestamp.Timestamp{
								Seconds: msec / 1e3,
								Nanos:   int32(msec%1e3) * 1e3,
							},
							Value: &metricspb.Point_Int64Value{Int64Value: 13},
						}},
					},
				},
			}},
		}
	}

	tests := []struct {
		name                  string
		sfxDataPoints         []*sfxpb.DataPoint
		wantMetricsData       *consumerdata.MetricsData
		wantDroppedTimeseries int
	}{
		{
			name:            "int_gauge",
			sfxDataPoints:   []*sfxpb.DataPoint{buildDefaulstSFxDataPt()},
			wantMetricsData: buildDefaultMetricsData(),
		},
		{
			name: "double_gauge",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_GAUGE)
				pt.Value = &sfxpb.Datum{
					DoubleValue: float64Ptr(13.13),
				}
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].MetricDescriptor.Type = metricspb.MetricDescriptor_GAUGE_DOUBLE
				md.Metrics[0].Timeseries[0].Points[0].Value = &metricspb.Point_DoubleValue{DoubleValue: 13.13}
				return md
			}(),
		},
		{
			name: "int_counter",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_COUNTER)
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].MetricDescriptor.Type = metricspb.MetricDescriptor_CUMULATIVE_INT64
				return md
			}(),
		},
		{
			name: "double_counter",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_COUNTER)
				pt.Value = &sfxpb.Datum{
					DoubleValue: float64Ptr(13.13),
				}
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].MetricDescriptor.Type = metricspb.MetricDescriptor_CUMULATIVE_DOUBLE
				md.Metrics[0].Timeseries[0].Points[0].Value = &metricspb.Point_DoubleValue{DoubleValue: 13.13}
				return md
			}(),
		},
		{
			name: "nil_timestamp",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.Timestamp = nil
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].Timeseries[0].Points[0].Timestamp = nil
				return md
			}(),
		},
		{
			name: "nil_dimension_value",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.Dimensions[0].Value = nil
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].Timeseries[0].LabelValues[0].Value = ""
				md.Metrics[0].Timeseries[0].LabelValues[0].HasValue = false
				return md
			}(),
		},
		{
			name: "nil_dimension_ignored",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				targetLen := 2*len(pt.Dimensions) + 1
				dimensions := make([]*sfxpb.Dimension, targetLen)
				copy(dimensions[1:], pt.Dimensions)
				assert.Equal(t, targetLen, len(dimensions))
				assert.Nil(t, dimensions[0])
				pt.Dimensions = dimensions
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetricsData: buildDefaultMetricsData(),
		},
		{
			name:            "nil_datapoint_ignored",
			sfxDataPoints:   []*sfxpb.DataPoint{nil, buildDefaulstSFxDataPt(), nil},
			wantMetricsData: buildDefaultMetricsData(),
		},
		{
			name: "drop_inconsistent_datapoints",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				// nil Datum
				pt0 := buildDefaulstSFxDataPt()
				pt0.Value = nil

				// nil expected Datum value
				pt1 := buildDefaulstSFxDataPt()
				pt1.Value.IntValue = nil

				// Non-supported type
				pt2 := buildDefaulstSFxDataPt()
				pt2.MetricType = sfxTypePtr(sfxpb.MetricType_ENUM)

				// Unknown type
				pt3 := buildDefaulstSFxDataPt()
				pt3.MetricType = sfxTypePtr(sfxpb.MetricType_CUMULATIVE_COUNTER + 1)

				return []*sfxpb.DataPoint{
					pt0, buildDefaulstSFxDataPt(), pt1, pt2, pt3}
			}(),
			wantMetricsData:       buildDefaultMetricsData(),
			wantDroppedTimeseries: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, numDroppedTimeseries := SignalFxV2ToMetricsData(zap.NewNop(), tt.sfxDataPoints)
			assert.Equal(t, tt.wantMetricsData, md)
			assert.Equal(t, tt.wantDroppedTimeseries, numDroppedTimeseries)
		})
	}
}

func Test_buildPoint_errors(t *testing.T) {
	type args struct {
		sfxDataPoint       *sfxpb.DataPoint
		expectedMetricType metricspb.MetricDescriptor_Type
	}
	tests := []struct {
		name    string
		args    args
		want    *metricspb.Point
		wantErr error
	}{
		{
			name: "nil_datum_value",
			args: args{
				sfxDataPoint: &sfxpb.DataPoint{},
			},
			wantErr: errSFxNilDatum,
		},
		{
			name: "expect_double_value",
			args: args{
				sfxDataPoint: &sfxpb.DataPoint{
					Value: &sfxpb.Datum{
						IntValue: int64Ptr(13),
					},
				},
				expectedMetricType: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
			},
			wantErr: errSFxUnexpectedInt64DatumType,
		},
		{
			name: "expect_int_value",
			args: args{
				sfxDataPoint: &sfxpb.DataPoint{
					Value: &sfxpb.Datum{
						DoubleValue: float64Ptr(13.13),
					},
				},
				expectedMetricType: metricspb.MetricDescriptor_GAUGE_INT64,
			},
			wantErr: errSFxUnexpectedFloat64DatumType,
		},
		{
			name: "no_value",
			args: args{
				sfxDataPoint: &sfxpb.DataPoint{
					Value: &sfxpb.Datum{},
				},
			},
			wantErr: errSFxNoDatumValue,
		},
		{
			name: "unexpect_str_value",
			args: args{
				sfxDataPoint: &sfxpb.DataPoint{
					Value: &sfxpb.Datum{
						StrValue: strPtr("13.13"),
					},
				},
			},
			wantErr: errSFxUnexpectedStringDatumType,
		},
		{
			name: "dbl_as_str",
			args: args{
				sfxDataPoint: &sfxpb.DataPoint{
					Value: &sfxpb.Datum{StrValue: strPtr("13.13")},
				},
				expectedMetricType: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			},
			want: &metricspb.Point{
				Value: &metricspb.Point_DoubleValue{DoubleValue: 13.13},
			},
		},
		{
			name: "str_not_dbl",
			args: args{
				sfxDataPoint: &sfxpb.DataPoint{
					Value: &sfxpb.Datum{StrValue: strPtr("not_a_number")},
				},
				expectedMetricType: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			},
			wantErr: errSFxStringDatumNotNumber,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPoint(tt.args.sfxDataPoint, tt.args.expectedMetricType)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func strPtr(s string) *string {
	l := s
	return &l
}

func int64Ptr(i int64) *int64 {
	l := i
	return &l
}

func float64Ptr(f float64) *float64 {
	l := f
	return &l
}

func sfxTypePtr(t sfxpb.MetricType) *sfxpb.MetricType {
	l := t
	return &l
}

func buildNDimensions(n uint) []*sfxpb.Dimension {
	d := make([]*sfxpb.Dimension, 0, n)
	for i := uint(0); i < n; i++ {
		idx := int(i)
		suffix := strconv.Itoa(idx)
		d = append(d, &sfxpb.Dimension{
			Key:   strPtr("k" + suffix),
			Value: strPtr("v" + suffix),
		})
	}
	return d
}
