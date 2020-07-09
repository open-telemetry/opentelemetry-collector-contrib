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
	"go.opentelemetry.io/collector/consumer/consumerdata"
)

const (
	metric1         = "metric1"
	metric2         = "metric2"
	label1          = "label1"
	label2          = "label2"
	labelValue11    = "label1-value1"
	labelValue21    = "label2-value1"
	labelValue12    = "label1-value2"
	labelValue22    = "label2-value2"
	newMetric1      = "metric1/new"
	newMetric2      = "metric2/new"
	newLabel1       = "label1/new"
	newLabelValue11 = "label1-value1/new"
	nonexist        = "nonexist"
)

type testPoint struct {
	timestamp             int64
	value                 int
	count                 int64
	sum                   float64
	bounds                []float64
	buckets               []int64
	sumOfSquaredDeviation float64
}

type testTimeseries struct {
	startTimestamp int64
	labelValues    []string
	points         []testPoint
}

type testMetric struct {
	name       string
	dataType   metricspb.MetricDescriptor_Type
	labelKeys  []string
	timeseries []testTimeseries
}

type metricsTransformTest struct {
	name       string // test name
	transforms []Transform
	in         []*metricspb.Metric
	out        []*metricspb.Metric
}

var (
	// operations
	validUpateLabelOperation = Operation{
		Action:   UpdateLabel,
		Label:    label1,
		NewLabel: newLabel1,
	}

	validUpdateLabelValueOperation = Operation{
		Action: UpdateLabel,
		Label:  label1,
		ValueActions: []ValueAction{
			{
				Value:    labelValue11,
				NewValue: newLabelValue11,
			},
		},
	}

	validUpdateLabelAggrSumOperation = Operation{
		Action:          AggregateLabels,
		LabelSet:        []string{label1},
		AggregationType: Sum,
	}

	validUpdateLabelAggrAverageOperation = Operation{
		Action:          AggregateLabels,
		LabelSet:        []string{label1},
		AggregationType: Average,
	}

	validUpdateLabelAggrMaxOperation = Operation{
		Action:          AggregateLabels,
		LabelSet:        []string{label1},
		AggregationType: Max,
	}

	validUpdateLabelValuesAggrSumOperation = Operation{
		Action:           AggregateLabelValues,
		Label:            label2,
		AggregatedValues: []string{labelValue21, labelValue22},
		NewValue:         labelValue21,
		AggregationType:  Sum,
	}

	validUpdateLabelValuesAggrAverageOperation = Operation{
		Action:           AggregateLabelValues,
		Label:            label2,
		AggregatedValues: []string{labelValue21, labelValue22},
		NewValue:         labelValue21,
		AggregationType:  Average,
	}

	validUpdateLabelValuesAggrMaxOperation = Operation{
		Action:           AggregateLabelValues,
		Label:            label2,
		AggregatedValues: []string{labelValue21, labelValue22},
		NewValue:         labelValue21,
		AggregationType:  Max,
	}

	// test cases
	standardTests = []metricsTransformTest{
		// UPDATE
		{
			name: "metric_name_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					NewName:    "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("new/metric1").Build(),
			},
		},
		{
			name: "metric_name_update_multiple",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					NewName:    "new/metric1",
				},
				{
					MetricName: "metric2",
					Action:     Update,
					NewName:    "new/metric2",
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").Build(),
				TestcaseBuilder().SetName("metric2").Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("new/metric1").Build(),
				TestcaseBuilder().SetName("new/metric2").Build(),
			},
		},
		{
			name: "metric_name_update_nonexist",
			transforms: []Transform{
				{
					MetricName: "nonexist",
					Action:     Update,
					NewName:    "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").Build(),
			},
		},
		{
			name: "metric_label_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:   UpdateLabel,
							Label:    "label1",
							NewLabel: "new/label1",
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"new/label1", "label2"}).Build(),
			},
		},
		{
			name: "metric_label_value_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action: UpdateLabel,
							Label:  "label1",
							ValueActions: []ValueAction{
								{
									Value:    "label1-value1",
									NewValue: "new/label1-value1",
								},
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).
					InitTimeseries(2).SetLabelValues([][]string{{"label1-value1"}, {"label1-value2"}}).Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).
					InitTimeseries(2).SetLabelValues([][]string{{"new/label1-value1"}, {"label1-value2"}}).Build(),
			},
		},
		// {
		// 	name: "metric_label_aggregation_sum_int_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelAggrSumOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64)},
		// 	out: []testMetric{outLabelAggrBuilder(4, metricspb.MetricDescriptor_GAUGE_INT64)},
		// },
		// {
		// 	name: "metric_label_aggregation_average_int_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelAggrAverageOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64)},
		// 	out: []testMetric{outLabelAggrBuilder(2, metricspb.MetricDescriptor_GAUGE_INT64)},
		// },
		// {
		// 	name: "metric_label_aggregation_max_int_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelAggrMaxOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64)},
		// 	out: []testMetric{outLabelAggrBuilder(3, metricspb.MetricDescriptor_GAUGE_INT64)},
		// },
		// {
		// 	name: "metric_label_aggregation_sum_double_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelAggrSumOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// 	out: []testMetric{outLabelAggrBuilder(4, metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// },
		// {
		// 	name: "metric_label_aggregation_average_double_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelAggrAverageOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// 	out: []testMetric{outLabelAggrBuilder(2, metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// },
		// {
		// 	name: "metric_label_aggregation_max_double_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelAggrMaxOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// 	out: []testMetric{outLabelAggrBuilder(3, metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// },
		// {
		// 	name: "metric_label_values_aggregation_sum_int_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelValuesAggrSumOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64)},
		// 	out: []testMetric{outLabelValuesAggrBuilder(4, metricspb.MetricDescriptor_GAUGE_INT64)},
		// },
		// {
		// 	name: "metric_label_values_aggregation_average_int_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelValuesAggrAverageOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64)},
		// 	out: []testMetric{outLabelValuesAggrBuilder(2, metricspb.MetricDescriptor_GAUGE_INT64)},
		// },
		// {
		// 	name: "metric_label_values_aggregation_max_int_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelValuesAggrMaxOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64)},
		// 	out: []testMetric{outLabelValuesAggrBuilder(3, metricspb.MetricDescriptor_GAUGE_INT64)},
		// },
		// {
		// 	name: "metric_label_values_aggregation_sum_double_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelValuesAggrSumOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// 	out: []testMetric{outLabelValuesAggrBuilder(4, metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// },
		// {
		// 	name: "metric_label_values_aggregation_average_double_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelValuesAggrAverageOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// 	out: []testMetric{outLabelValuesAggrBuilder(2, metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// },
		// {
		// 	name: "metric_label_values_aggregation_max_double_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelValuesAggrMaxOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// 	out: []testMetric{outLabelValuesAggrBuilder(3, metricspb.MetricDescriptor_GAUGE_DOUBLE)},
		// },
		// {
		// 	name: "metric_label_values_aggregation_sum_distribution_update",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Update,
		// 			Operations: []Operation{validUpdateLabelAggrSumOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialDistValueMetricBuilder()},
		// 	out: []testMetric{outDistValueMetricBuilder()},
		// },
		// INSERT
		{
			name: "metric_name_insert",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").Build(),
				TestcaseBuilder().SetName("new/metric1").Build(),
			},
		},
		{
			name: "metric_name_insert_multiple",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "new/metric1",
				},
				{
					MetricName: "metric2",
					Action:     Insert,
					NewName:    "new/metric2",
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").Build(),
				TestcaseBuilder().SetName("metric2").Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").Build(),
				TestcaseBuilder().SetName("metric2").Build(),
				TestcaseBuilder().SetName("new/metric1").Build(),
				TestcaseBuilder().SetName("new/metric2").Build(),
			},
		},
		{
			name: "metric_label_update_with_metric_insert",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "new/metric1",
					Operations: []Operation{
						{
							Action:   UpdateLabel,
							Label:    "label1",
							NewLabel: "new/label1",
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).Build(),
				TestcaseBuilder().SetName("new/metric1").SetLabels([]string{"new/label1", "label2"}).Build(),
			},
		},
		{
			name: "metric_label_value_update_with_metric_insert",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "new/metric1",
					Operations: []Operation{
						{
							Action: UpdateLabel,
							Label:  "label1",
							ValueActions: []ValueAction{
								{
									Value:    "label1-value1",
									NewValue: "new/label1-value1",
								},
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).
					InitTimeseries(2).SetLabelValues([][]string{{"label1-value1"}, {"label1-value2"}}).Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).
					InitTimeseries(2).SetLabelValues([][]string{{"label1-value1"}, {"label1-value2"}}).Build(),

				TestcaseBuilder().SetName("new/metric1").SetLabels([]string{"label1"}).
					InitTimeseries(2).SetLabelValues([][]string{{"new/label1-value1"}, {"label1-value2"}}).Build(),
			},
		},
		// {
		// 	name: "metric_label_aggregation_sum_int_insert",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Insert,
		// 			Operations: []Operation{validUpdateLabelAggrSumOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64)},
		// 	out: []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64), outLabelAggrBuilder(4, metricspb.MetricDescriptor_GAUGE_INT64)},
		// },
		// {
		// 	name: "metric_label_values_aggregation_sum_int_insert",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Insert,
		// 			Operations: []Operation{validUpdateLabelValuesAggrSumOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64)},
		// 	out: []testMetric{initialAggrBuilder(metricspb.MetricDescriptor_GAUGE_INT64), outLabelValuesAggrBuilder(4, metricspb.MetricDescriptor_GAUGE_INT64)},
		// },
		// {
		// 	name: "metric_label_values_aggregation_sum_distribution_insert",
		// 	transforms: []Transform{
		// 		{
		// 			MetricName: metric1,
		// 			Action:     Insert,
		// 			Operations: []Operation{validUpdateLabelAggrSumOperation},
		// 		},
		// 	},
		// 	in:  []testMetric{initialDistValueMetricBuilder()},
		// 	out: []testMetric{initialDistValueMetricBuilder(), outDistValueMetricBuilder()},
		// },
	}
)

// constructTestInputMetricsDate builds the actual metrics from the test cases
func constructTestInputMetricsData(test metricsTransformTest) consumerdata.MetricsData {
	md := consumerdata.MetricsData{
		Metrics: make([]*metricspb.Metric, len(test.in)),
	}
	for idx, in := range test.in {
		// dataType := in.dataType
		// // construct label keys
		// labels := make([]*metricspb.LabelKey, len(in.labelKeys))
		// for lidx, l := range in.labelKeys {
		// 	labels[lidx] = &metricspb.LabelKey{
		// 		Key: l,
		// 	}
		// }
		// // construct timeseries with label values and points
		// timeseries := make([]*metricspb.TimeSeries, len(in.timeseries))
		// for tidx, ts := range in.timeseries {
		// 	labelValues := make([]*metricspb.LabelValue, len(ts.labelValues))
		// 	for vidx, value := range ts.labelValues {
		// 		labelValues[vidx] = &metricspb.LabelValue{
		// 			Value: value,
		// 		}
		// 	}
		// 	points := make([]*metricspb.Point, len(ts.points))
		// 	for pidx, p := range ts.points {
		// 		points[pidx] = &metricspb.Point{
		// 			Timestamp: &timestamp.Timestamp{
		// 				Seconds: p.timestamp,
		// 				Nanos:   0,
		// 			},
		// 		}
		// 		switch dataType {
		// 		case metricspb.MetricDescriptor_CUMULATIVE_INT64, metricspb.MetricDescriptor_GAUGE_INT64:
		// 			points[pidx].Value = &metricspb.Point_Int64Value{
		// 				Int64Value: int64(p.value),
		// 			}
		// 		case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metricspb.MetricDescriptor_GAUGE_DOUBLE:
		// 			points[pidx].Value = &metricspb.Point_DoubleValue{
		// 				DoubleValue: float64(p.value),
		// 			}
		// 		case metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION, metricspb.MetricDescriptor_GAUGE_DISTRIBUTION:
		// 			buckets := make([]*metricspb.DistributionValue_Bucket, len(p.buckets))
		// 			for buIdx, bucket := range p.buckets {
		// 				buckets[buIdx] = &metricspb.DistributionValue_Bucket{
		// 					Count: bucket,
		// 				}
		// 			}
		// 			points[pidx].Value = &metricspb.Point_DistributionValue{
		// 				DistributionValue: &metricspb.DistributionValue{
		// 					BucketOptions: &metricspb.DistributionValue_BucketOptions{
		// 						Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
		// 							Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
		// 								Bounds: p.bounds,
		// 							},
		// 						},
		// 					},
		// 					Count:                 p.count,
		// 					Sum:                   p.sum,
		// 					Buckets:               buckets,
		// 					SumOfSquaredDeviation: p.sumOfSquaredDeviation,
		// 				},
		// 			}
		// 		}
		// 	}
		// 	timeseries[tidx] = &metricspb.TimeSeries{
		// 		StartTimestamp: &timestamp.Timestamp{
		// 			Seconds: ts.startTimestamp,
		// 			Nanos:   0,
		// 		},
		// 		LabelValues: labelValues,
		// 		Points:      points,
		// 	}
		// }

		// compose the metric
		md.Metrics[idx] = in
	}
	return md
}

func initialAggrBuilder(dataType metricspb.MetricDescriptor_Type) testMetric {
	return testMetric{
		name:      metric1,
		labelKeys: []string{label1, label2},
		dataType:  dataType,
		timeseries: []testTimeseries{
			{
				startTimestamp: 2,
				labelValues:    []string{labelValue11, labelValue21},
				points: []testPoint{
					{
						timestamp: 2,
						value:     3,
					},
				},
			},
			{
				startTimestamp: 1,
				labelValues:    []string{labelValue11, labelValue22},
				points: []testPoint{
					{
						timestamp: 2,
						value:     1,
					},
				},
			},
		},
	}
}

func outLabelAggrBuilder(value int, dataType metricspb.MetricDescriptor_Type) testMetric {
	return testMetric{
		name:      metric1,
		labelKeys: []string{label1},
		dataType:  dataType,
		timeseries: []testTimeseries{
			{
				startTimestamp: 1,
				labelValues:    []string{labelValue11},
				points: []testPoint{
					{
						timestamp: 2,
						value:     value,
					},
				},
			},
		},
	}
}

func outLabelValuesAggrBuilder(value int, dataType metricspb.MetricDescriptor_Type) testMetric {
	outMetric := testMetric{
		name:      metric1,
		labelKeys: []string{label1, label2},
		dataType:  dataType,
		timeseries: []testTimeseries{
			{
				startTimestamp: 1,
				labelValues:    []string{labelValue11, labelValue21},
				points: []testPoint{
					{
						timestamp: 2,
						value:     value,
					},
				},
			},
		},
	}
	return outMetric
}

func initialDistValueMetricBuilder() testMetric {
	return testMetric{
		name:      metric1,
		labelKeys: []string{label1, label2},
		dataType:  metricspb.MetricDescriptor_GAUGE_DISTRIBUTION,
		timeseries: []testTimeseries{
			{
				startTimestamp: 1,
				labelValues:    []string{labelValue11, labelValue21},
				points: []testPoint{
					{
						timestamp:             1,
						count:                 3,
						sum:                   6,
						bounds:                []float64{1, 2},
						buckets:               []int64{0, 1, 2},
						sumOfSquaredDeviation: 3,
					},
				},
			},
			{
				startTimestamp: 3,
				labelValues:    []string{labelValue11, labelValue22},
				points: []testPoint{
					{
						timestamp:             1,
						count:                 5,
						sum:                   10,
						bounds:                []float64{1, 2},
						buckets:               []int64{1, 1, 3},
						sumOfSquaredDeviation: 4,
					},
				},
			},
		},
	}
}

func outDistValueMetricBuilder() testMetric {
	return testMetric{
		name:      metric1,
		labelKeys: []string{label1},
		dataType:  metricspb.MetricDescriptor_GAUGE_DISTRIBUTION,
		timeseries: []testTimeseries{
			{
				startTimestamp: 1,
				labelValues:    []string{labelValue11},
				points: []testPoint{
					{
						timestamp:             1,
						count:                 8,
						sum:                   16,
						bounds:                []float64{1, 2},
						buckets:               []int64{1, 2, 5},
						sumOfSquaredDeviation: 7,
					},
				},
			},
		},
	}
}
