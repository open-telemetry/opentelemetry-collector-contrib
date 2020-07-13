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

type metricsTransformTest struct {
	name       string // test name
	transforms []Transform
	in         []*metricspb.Metric
	out        []*metricspb.Metric
}

var (
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
							ValueActionsMapping: map[string]string{
								"label1-value1": "new/label1-value1",
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).
					AddTimeseries(1, []string{"label1-value1"}).AddTimeseries(1, []string{"label1-value2"}).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).
					AddTimeseries(1, []string{"new/label1-value1"}).AddTimeseries(1, []string{"label1-value2"}).
					Build(),
			},
		},
		{
			name: "metric_label_aggregation_sum_int_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Sum,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddInt64Point(0, 3, 2).AddInt64Point(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(1, []string{"label1-value1"}).
					AddInt64Point(0, 4, 2).
					Build(),
			},
		},
		{
			name: "metric_label_aggregation_average_int_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Average,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddInt64Point(0, 3, 2).AddInt64Point(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(1, []string{"label1-value1"}).
					AddInt64Point(0, 2, 2).
					Build(),
			},
		},
		{
			name: "metric_label_aggregation_max_int_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Max,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddInt64Point(0, 3, 2).AddInt64Point(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(1, []string{"label1-value1"}).
					AddInt64Point(0, 3, 2).
					Build(),
			},
		},
		{
			name: "metric_label_aggregation_min_int_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Min,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddInt64Point(0, 3, 2).AddInt64Point(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(1, []string{"label1-value1"}).
					AddInt64Point(0, 1, 2).
					Build(),
			},
		},
		{
			name: "metric_label_aggregation_sum_double_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Sum,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddDoublePoint(0, 3, 2).AddDoublePoint(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					AddTimeseries(1, []string{"label1-value1"}).
					AddDoublePoint(0, 4, 2).
					Build(),
			},
		},
		{
			name: "metric_label_aggregation_average_double_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Average,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddDoublePoint(0, 3, 2).AddDoublePoint(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					AddTimeseries(1, []string{"label1-value1"}).
					AddDoublePoint(0, 2, 2).
					Build(),
			},
		},
		{
			name: "metric_label_aggregation_max_double_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Max,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddDoublePoint(0, 3, 2).AddDoublePoint(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					AddTimeseries(1, []string{"label1-value1"}).
					AddDoublePoint(0, 3, 2).
					Build(),
			},
		},
		{
			name: "metric_label_aggregation_min_double_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Min,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddDoublePoint(0, 3, 2).AddDoublePoint(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					AddTimeseries(1, []string{"label1-value1"}).
					AddDoublePoint(0, 1, 2).
					Build(),
			},
		},
		{
			name: "metric_label_values_aggregation_sum_int_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:              AggregateLabelValues,
							LabelSetMap:         map[string]bool{"label2": true},
							AggregatedValuesSet: map[string]bool{"label2-value1": true, "label2-value2": true},
							NewValue:            "new/label2-value",
							AggregationType:     Sum,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddInt64Point(0, 3, 2).AddInt64Point(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(1, []string{"label1-value1", "new/label2-value"}).
					AddInt64Point(0, 4, 2).
					Build(),
			},
		},
		// this test case also tests the correctness of the SumOfSquaredDeviation merging
		{
			name: "metric_label_values_aggregation_sum_distribution_update",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Sum,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					AddTimeseries(1, []string{"label1-value1", "label2-value1"}).AddTimeseries(3, []string{"label1-value1", "label2-value2"}).
					AddDistributionPoints(0, 1, 3, 6, []float64{1, 2}, []int64{0, 1, 2}, 2).  // pointGroup1: {1, 2, 3}, SumOfSquaredDeviation = 2
					AddDistributionPoints(1, 1, 5, 10, []float64{1, 2}, []int64{0, 2, 3}, 4). // pointGroup2: {1, 2, 3, 3, 1}, SumOfSquaredDeviation = 4
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					AddTimeseries(1, []string{"label1-value1"}).
					AddDistributionPoints(0, 1, 8, 16, []float64{1, 2}, []int64{0, 3, 5}, 6). // pointGroupCombined: {1, 2, 3, 1, 2, 3, 3, 1}, SumOfSquaredDeviation = 6
					Build(),
			},
		},
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
							Action:              UpdateLabel,
							Label:               "label1",
							ValueActionsMapping: map[string]string{"label1-value1": "new/label1-value1"},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).
					AddTimeseries(1, []string{"label1-value1"}).AddTimeseries(1, []string{"label1-value2"}).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).
					AddTimeseries(1, []string{"label1-value1"}).AddTimeseries(1, []string{"label1-value2"}).
					Build(),

				TestcaseBuilder().SetName("new/metric1").SetLabels([]string{"label1"}).
					AddTimeseries(1, []string{"new/label1-value1"}).AddTimeseries(1, []string{"label1-value2"}).
					Build(),
			},
		},
		{
			name: "metric_label_aggregation_sum_int_insert",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Sum,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddInt64Point(0, 3, 2).AddInt64Point(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddInt64Point(0, 3, 2).AddInt64Point(1, 1, 2).
					Build(),
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(1, []string{"label1-value1"}).
					AddInt64Point(0, 4, 2).
					Build(),
			},
		},
		{
			name: "metric_label_values_aggregation_sum_int_insert",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					Operations: []Operation{
						{
							Action:              AggregateLabelValues,
							LabelSetMap:         map[string]bool{"label2": true},
							AggregatedValuesSet: map[string]bool{"label2-value1": true, "label2-value2": true},
							NewValue:            "new/label2-value",
							AggregationType:     Sum,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddInt64Point(0, 3, 2).AddInt64Point(1, 1, 2).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(2, []string{"label1-value1", "label2-value1"}).AddTimeseries(1, []string{"label1-value1", "label2-value2"}).
					AddInt64Point(0, 3, 2).AddInt64Point(1, 1, 2).
					Build(),
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					AddTimeseries(1, []string{"label1-value1", "new/label2-value"}).
					AddInt64Point(0, 4, 2).
					Build(),
			},
		},
		{
			name: "metric_labels_aggregation_sum_distribution_insert",
			transforms: []Transform{
				{
					MetricName: "metric1",
					Action:     Insert,
					Operations: []Operation{
						{
							Action:          AggregateLabels,
							LabelSetMap:     map[string]bool{"label1": true},
							AggregationType: Sum,
						},
					},
				},
			},
			in: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					AddTimeseries(1, []string{"label1-value1", "label2-value1"}).AddTimeseries(3, []string{"label1-value1", "label2-value2"}).
					AddDistributionPoints(0, 1, 3, 6, []float64{1, 2}, []int64{0, 1, 2}, 3).
					AddDistributionPoints(1, 1, 5, 10, []float64{1, 2}, []int64{1, 1, 3}, 4).
					Build(),
			},
			out: []*metricspb.Metric{
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1", "label2"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					AddTimeseries(1, []string{"label1-value1", "label2-value1"}).AddTimeseries(3, []string{"label1-value1", "label2-value2"}).
					AddDistributionPoints(0, 1, 3, 6, []float64{1, 2}, []int64{0, 1, 2}, 3).
					AddDistributionPoints(1, 1, 5, 10, []float64{1, 2}, []int64{1, 1, 3}, 4).
					Build(),
				TestcaseBuilder().SetName("metric1").SetLabels([]string{"label1"}).SetDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					AddTimeseries(1, []string{"label1-value1"}).
					AddDistributionPoints(0, 1, 8, 16, []float64{1, 2}, []int64{1, 2, 5}, 7).
					Build(),
			},
		},
	}
)

// constructTestInputMetricsDate builds the actual metrics from the test cases
func constructTestInputMetricsData(test metricsTransformTest) consumerdata.MetricsData {
	md := consumerdata.MetricsData{
		Metrics: make([]*metricspb.Metric, len(test.in)),
	}
	for idx, in := range test.in {
		// compose the metric
		md.Metrics[idx] = in
	}
	return md
}
