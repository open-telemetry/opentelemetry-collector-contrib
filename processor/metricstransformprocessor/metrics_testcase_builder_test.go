// Copyright 2020 OpenTelemetry Authors
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

package metricstransformprocessor

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type builder struct {
	metric *metricspb.Metric
}

// metricBuilder is used to build metrics for testing
func metricBuilder() builder {
	return builder{
		metric: &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{},
			Timeseries:       make([]*metricspb.TimeSeries, 0),
		},
	}
}

// setName sets the name of the metric
func (b builder) setName(name string) builder {
	b.metric.MetricDescriptor.Name = name
	return b
}

// setLabels sets the labels for the metric
func (b builder) setLabels(labels []string) builder {
	labelKeys := make([]*metricspb.LabelKey, len(labels))
	for i, l := range labels {
		labelKeys[i] = &metricspb.LabelKey{
			Key: l,
		}
	}
	b.metric.MetricDescriptor.LabelKeys = labelKeys
	return b
}

// addTimeseries adds new timeseries with the labelValuesVal and startTimestamp
func (b builder) addTimeseries(startTimestampSeconds int64, labelValuesVal []string) builder {
	labelValues := make([]*metricspb.LabelValue, len(labelValuesVal))
	for i, v := range labelValuesVal {
		labelValues[i] = &metricspb.LabelValue{
			Value:    v,
			HasValue: true,
		}
	}

	var startTimestamp *timestamppb.Timestamp
	if startTimestampSeconds != 0 {
		startTimestamp = &timestamppb.Timestamp{Seconds: startTimestampSeconds}
	}

	timeseries := &metricspb.TimeSeries{
		StartTimestamp: startTimestamp,
		LabelValues:    labelValues,
		Points:         nil,
	}
	b.metric.Timeseries = append(b.metric.Timeseries, timeseries)
	return b
}

// setDataType sets the data type of this metric
func (b builder) setDataType(dataType metricspb.MetricDescriptor_Type) builder {
	b.metric.MetricDescriptor.Type = dataType
	return b
}

// setUnit sets the unit of this metric
func (b builder) setUnit(unit string) builder {
	b.metric.MetricDescriptor.Unit = unit
	return b
}

// addInt64Point adds a int64 point to the tidx-th timseries
func (b builder) addInt64Point(tidx int, val int64, timestampVal int64) builder {
	point := &metricspb.Point{
		Timestamp: &timestamppb.Timestamp{
			Seconds: timestampVal,
			Nanos:   0,
		},
		Value: &metricspb.Point_Int64Value{
			Int64Value: val,
		},
	}
	points := b.metric.Timeseries[tidx].Points
	b.metric.Timeseries[tidx].Points = append(points, point)
	return b
}

// addDoublePoint adds a double point to the tidx-th timseries
func (b builder) addDoublePoint(tidx int, val float64, timestampVal int64) builder {
	point := &metricspb.Point{
		Timestamp: &timestamppb.Timestamp{
			Seconds: timestampVal,
			Nanos:   0,
		},
		Value: &metricspb.Point_DoubleValue{
			DoubleValue: val,
		},
	}
	points := b.metric.Timeseries[tidx].Points
	b.metric.Timeseries[tidx].Points = append(points, point)
	return b
}

// addDistributionPoints adds a distribution point to the tidx-th timseries
func (b builder) addDistributionPoints(tidx int, count int64, sum float64, bounds []float64, bucketsVal []int64) builder {
	buckets := make([]*metricspb.DistributionValue_Bucket, len(bucketsVal))
	for buIdx, bucket := range bucketsVal {
		buckets[buIdx] = &metricspb.DistributionValue_Bucket{
			Count: bucket,
		}
	}
	point := &metricspb.Point{
		Timestamp: &timestamppb.Timestamp{
			Seconds: 1,
			Nanos:   0,
		},
		Value: &metricspb.Point_DistributionValue{
			DistributionValue: &metricspb.DistributionValue{
				BucketOptions: &metricspb.DistributionValue_BucketOptions{
					Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
						Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
							Bounds: bounds,
						},
					},
				},
				Count:   count,
				Sum:     sum,
				Buckets: buckets,
			},
		},
	}
	points := b.metric.Timeseries[tidx].Points
	b.metric.Timeseries[tidx].Points = append(points, point)
	return b
}

// Build builds from the builder to the final metric
func (b builder) build() *metricspb.Metric {
	return b.metric
}
