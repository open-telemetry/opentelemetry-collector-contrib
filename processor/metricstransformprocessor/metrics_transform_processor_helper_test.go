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
	"math"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

type metricsTransformTest struct {
	name       string // test name
	transforms []mtpTransform
	in         []*metricspb.Metric
	out        []*metricspb.Metric
}

var (
	// test cases
	standardTests = []metricsTransformTest{
		// UPDATE
		{
			name: "metric_name_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					NewName:    "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("new/metric1").build(),
			},
		},
		{
			name: "metric_name_update_multiple",
			transforms: []mtpTransform{
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
				testcaseBuilder().setName("metric1").build(),
				testcaseBuilder().setName("metric2").build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("new/metric1").build(),
				testcaseBuilder().setName("new/metric2").build(),
			},
		},
		{
			name: "metric_name_update_nonexist",
			transforms: []mtpTransform{
				{
					MetricName: "nonexist",
					Action:     Update,
					NewName:    "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").build(),
			},
		},
		{
			name: "metric_label_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:   UpdateLabel,
								Label:    "label1",
								NewLabel: "new/label1",
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"new/label1", "label2"}).build(),
			},
		},
		{
			name: "metric_label_value_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action: UpdateLabel,
								Label:  "label1",
							},
							valueActionsMapping: map[string]string{
								"label1-value1": "new/label1-value1",
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).
					addTimeseries(1, []string{"label1-value1"}).addTimeseries(1, []string{"label1-value2"}).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).
					addTimeseries(1, []string{"new/label1-value1"}).addTimeseries(1, []string{"label1-value2"}).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_sum_int_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 4, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_mean_int_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Mean,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 2, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_max_int_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Max,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 3, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_min_int_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Min,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 1, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_sum_double_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addDoublePoint(0, 3, 2).addDoublePoint(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1"}).
					addDoublePoint(0, 4, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_mean_double_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Mean,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addDoublePoint(0, 3, 2).addDoublePoint(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1"}).
					addDoublePoint(0, 2, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_max_double_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Max,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addDoublePoint(0, 3, 2).addDoublePoint(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1"}).
					addDoublePoint(0, 3, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_min_double_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Min,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addDoublePoint(0, 3, 2).addDoublePoint(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1"}).
					addDoublePoint(0, 1, 2).
					build(),
			},
		},
		{
			name: "metric_label_values_aggregation_sum_int_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabelValues,
								NewValue:        "new/label2-value",
								AggregationType: Sum,
							},
							labelSetMap:         map[string]bool{"label2": true},
							aggregatedValuesSet: map[string]bool{"label2-value1": true, "label2-value2": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addTimeseries(1, []string{"label1-value1", "label2-value3"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).addInt64Point(2, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "new/label2-value"}).addTimeseries(1, []string{"label1-value1", "label2-value3"}).
					addInt64Point(0, 4, 2).addInt64Point(1, 1, 2).
					build(),
			},
		},
		// this test case also tests the correctness of the SumOfSquaredDeviation merging
		{
			name: "metric_label_values_aggregation_sum_distribution_update",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Update,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).addTimeseries(3, []string{"label1-value1", "label2-value2"}).
					addDistributionPoints(0, 1, 3, 6, []float64{1, 2}, []int64{0, 1, 2}, 2).  // pointGroup1: {1, 2, 3}, SumOfSquaredDeviation = 2
					addDistributionPoints(1, 1, 5, 10, []float64{1, 2}, []int64{0, 2, 3}, 4). // pointGroup2: {1, 2, 3, 3, 1}, SumOfSquaredDeviation = 4
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1"}).
					addDistributionPoints(0, 1, 8, 16, []float64{1, 2}, []int64{0, 3, 5}, 6). // pointGroupCombined: {1, 2, 3, 1, 2, 3, 3, 1}, SumOfSquaredDeviation = 6
					build(),
			},
		},
		// INSERT
		{
			name: "metric_name_insert",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").build(),
				testcaseBuilder().setName("new/metric1").build(),
			},
		},
		{
			name: "metric_name_insert_multiple",
			transforms: []mtpTransform{
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
				testcaseBuilder().setName("metric1").build(),
				testcaseBuilder().setName("metric2").build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").build(),
				testcaseBuilder().setName("metric2").build(),
				testcaseBuilder().setName("new/metric1").build(),
				testcaseBuilder().setName("new/metric2").build(),
			},
		},
		{
			name: "metric_label_update_with_metric_insert",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "new/metric1",
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:   UpdateLabel,
								Label:    "label1",
								NewLabel: "new/label1",
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).build(),
				testcaseBuilder().setName("new/metric1").setLabels([]string{"new/label1", "label2"}).build(),
			},
		},
		{
			name: "metric_label_value_update_with_metric_insert",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Insert,
					NewName:    "new/metric1",
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action: UpdateLabel,
								Label:  "label1",
							},
							valueActionsMapping: map[string]string{"label1-value1": "new/label1-value1"},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).
					addTimeseries(1, []string{"label1-value1"}).addTimeseries(1, []string{"label1-value2"}).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).
					addTimeseries(1, []string{"label1-value1"}).addTimeseries(1, []string{"label1-value2"}).
					build(),

				testcaseBuilder().setName("new/metric1").setLabels([]string{"label1"}).
					addTimeseries(1, []string{"new/label1-value1"}).addTimeseries(1, []string{"label1-value2"}).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_sum_int_insert",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Insert,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 4, 2).
					build(),
			},
		},
		{
			name: "metric_label_values_aggregation_sum_int_insert",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Insert,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabelValues,
								NewValue:        "new/label2-value",
								AggregationType: Sum,
							},
							labelSetMap:         map[string]bool{"label2": true},
							aggregatedValuesSet: map[string]bool{"label2-value1": true, "label2-value2": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "new/label2-value"}).
					addInt64Point(0, 4, 2).
					build(),
			},
		},
		{
			name: "metric_labels_aggregation_sum_distribution_insert",
			transforms: []mtpTransform{
				{
					MetricName: "metric1",
					Action:     Insert,
					Operations: []mtpOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).addTimeseries(3, []string{"label1-value1", "label2-value2"}).
					addDistributionPoints(0, 1, 3, 6, []float64{1, 2}, []int64{0, 1, 2}, 3).
					addDistributionPoints(1, 1, 5, 10, []float64{1, 2}, []int64{1, 1, 3}, 4).
					build(),
			},
			out: []*metricspb.Metric{
				testcaseBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).addTimeseries(3, []string{"label1-value1", "label2-value2"}).
					addDistributionPoints(0, 1, 3, 6, []float64{1, 2}, []int64{0, 1, 2}, 3).
					addDistributionPoints(1, 1, 5, 10, []float64{1, 2}, []int64{1, 1, 3}, 4).
					build(),
				testcaseBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1"}).
					addDistributionPoints(0, 1, 8, 16, []float64{1, 2}, []int64{1, 2, 5}, 7).
					build(),
			},
		},
	}
)

// calculateSumOfSquaredDeviation returns the sum and the sumOfSquaredDeviation for this slice
func calculateSumOfSquaredDeviation(slice []float64) (sum float64, sumOfSquaredDeviation float64) {
	sum = 0
	for _, e := range slice {
		sum += e
	}
	ave := sum / float64(len(slice))
	sumOfSquaredDeviation = 0
	for _, e := range slice {
		sumOfSquaredDeviation += math.Pow((e - ave), 2)
	}
	return
}
