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

package splunkhecreceiver

import (
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func Test_splunkV2ToMetricsData(t *testing.T) {
	// Timestamps for Splunk have a resolution to the millisecond, where the time is reported in seconds with a floating value to the millisecond.
	now := time.Now()
	msecInt64 := now.UnixNano() / 1e6
	sec := float64(msecInt64) / 1e3

	buildDefaultSplunkDataPt := func() *splunk.Metric {
		return &splunk.Metric{
			Time:       sec,
			Host:       "localhost",
			Source:     "source",
			SourceType: "sourcetype",
			Index:      "index",
			Event:      "metrics",
			Fields: map[string]interface{}{
				"metric_name:single": int64Ptr(13),
				"k0":                 "v0",
				"k1":                 "v1",
				"k2":                 "v2",
			},
		}
	}

	buildDefaultMetricsData := func() *consumerdata.MetricsData {
		return &consumerdata.MetricsData{
			Metrics: []*metricspb.Metric{{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "single",
					Type: metricspb.MetricDescriptor_UNSPECIFIED,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "k0"}, {Key: "k1"}, {Key: "k2"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						StartTimestamp: &timestamp.Timestamp{
							Seconds: int64(sec),
							Nanos:   int32(int64(sec*1e3)%1e3) * 1e6,
						},
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
								Seconds: int64(sec),
								Nanos:   int32(int64(sec*1e3)%1e3) * 1e6,
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
		splunkDataPoints      *splunk.Metric
		wantMetricsData       *consumerdata.MetricsData
		wantDroppedTimeseries int
	}{
		{
			name:             "int_gauge",
			splunkDataPoints: buildDefaultSplunkDataPt(),
			wantMetricsData:  buildDefaultMetricsData(),
		},
		{
			name: "multiple",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:yetanother"] = int64Ptr(14)
				pt.Fields["metric_name:yetanotherandanother"] = int64Ptr(15)
				return pt
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := &consumerdata.MetricsData{
					Metrics: []*metricspb.Metric{{
						MetricDescriptor: &metricspb.MetricDescriptor{
							Name: "single",
							Type: metricspb.MetricDescriptor_UNSPECIFIED,
							LabelKeys: []*metricspb.LabelKey{
								{Key: "k0"}, {Key: "k1"}, {Key: "k2"},
							},
						},
						Timeseries: []*metricspb.TimeSeries{
							{
								StartTimestamp: &timestamp.Timestamp{
									Seconds: int64(sec),
									Nanos:   int32(int64(sec*1e3)%1e3) * 1e6,
								},
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
										Seconds: int64(sec),
										Nanos:   int32(int64(sec*1e3)%1e3) * 1e6,
									},
									Value: &metricspb.Point_Int64Value{Int64Value: 13},
								}},
							},
						},
					},
						{
							MetricDescriptor: &metricspb.MetricDescriptor{
								Name: "yetanother",
								Type: metricspb.MetricDescriptor_UNSPECIFIED,
								LabelKeys: []*metricspb.LabelKey{
									{Key: "k0"}, {Key: "k1"}, {Key: "k2"},
								},
							},
							Timeseries: []*metricspb.TimeSeries{
								{
									StartTimestamp: &timestamp.Timestamp{
										Seconds: int64(sec),
										Nanos:   int32(int64(sec*1e3)%1e3) * 1e6,
									},
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
											Seconds: int64(sec),
											Nanos:   int32(int64(sec*1e3)%1e3) * 1e6,
										},
										Value: &metricspb.Point_Int64Value{Int64Value: 14},
									}},
								},
							},
						},
						{
							MetricDescriptor: &metricspb.MetricDescriptor{
								Name: "yetanotherandanother",
								Type: metricspb.MetricDescriptor_UNSPECIFIED,
								LabelKeys: []*metricspb.LabelKey{
									{Key: "k0"}, {Key: "k1"}, {Key: "k2"},
								},
							},
							Timeseries: []*metricspb.TimeSeries{
								{
									StartTimestamp: &timestamp.Timestamp{
										Seconds: int64(sec),
										Nanos:   int32(int64(sec*1e3)%1e3) * 1e6,
									},
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
											Seconds: int64(sec),
											Nanos:   int32(int64(sec*1e3)%1e3) * 1e6,
										},
										Value: &metricspb.Point_Int64Value{Int64Value: 15},
									}},
								},
							},
						}},
				}
				return md
			}(),
		},
		{
			name: "double_gauge",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = float64Ptr(13.13)
				return pt
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].Timeseries[0].Points[0].Value = &metricspb.Point_DoubleValue{DoubleValue: 13.13}
				return md
			}(),
		},
		{
			name: "int_counter_pointer",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				return pt
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				return md
			}(),
		},
		{
			name: "int_counter",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = int64(13)
				return pt
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				return md
			}(),
		},
		{
			name: "double_counter",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = float64Ptr(13.13)
				return pt
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].Timeseries[0].Points[0].Value = &metricspb.Point_DoubleValue{DoubleValue: 13.13}
				return md
			}(),
		},
		{
			name: "double_counter_as_string",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = strPtr("13.13")
				return pt
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].Timeseries[0].Points[0].Value = &metricspb.Point_DoubleValue{DoubleValue: 13.13}
				return md
			}(),
		},
		{
			name: "nil_timestamp",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				pt.Time = 0
				return pt
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].Timeseries[0].StartTimestamp = nil
				md.Metrics[0].Timeseries[0].Points[0].Timestamp = nil
				return md
			}(),
		},
		{
			name: "empty_dimension_value",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["k0"] = ""
				return pt
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics[0].Timeseries[0].LabelValues[0].Value = ""
				md.Metrics[0].Timeseries[0].LabelValues[0].HasValue = true
				return md
			}(),
		},
		{
			name: "invalid_point",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = "foo"
				return pt
			}(),
			wantMetricsData: func() *consumerdata.MetricsData {
				md := buildDefaultMetricsData()
				md.Metrics = md.Metrics[:len(md.Metrics)-1]
				return md
			}(),
		},
		{
			name: "nil_dimension_ignored",
			splunkDataPoints: func() *splunk.Metric {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["k4"] = nil
				pt.Fields["k5"] = nil
				pt.Fields["k6"] = nil
				return pt
			}(),
			wantMetricsData: buildDefaultMetricsData(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, numDroppedTimeseries := SplunkHecToMetricsData(zap.NewNop(), tt.splunkDataPoints)
			assert.Equal(t, tt.wantDroppedTimeseries, numDroppedTimeseries)
			assert.Equal(t, tt.wantMetricsData, md)
		})
	}
}

func Test_buildPoint_errors(t *testing.T) {
	type args struct {
		splunkDataPoint *splunk.Metric
	}
	tests := []struct {
		name    string
		args    args
		want    *metricspb.Point
		wantErr error
	}{
		{
			name: "bad_value",
			args: args{
				splunkDataPoint: &splunk.Metric{
					Fields: map[string]interface{}{
						"metric_name:foo": strPtr("bar"),
					},
				},
			},
			wantErr: errSplunkStringDatumNotNumber,
		},
		{
			name: "cannot_convert",
			args: args{
				splunkDataPoint: &splunk.Metric{
					Fields: map[string]interface{}{
						"metric_name:foo": nil,
					},
				},
			},
			wantErr: errSplunkNoDatumValue,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPoint(zap.NewNop(), tt.args.splunkDataPoint, "foo", 0)
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
