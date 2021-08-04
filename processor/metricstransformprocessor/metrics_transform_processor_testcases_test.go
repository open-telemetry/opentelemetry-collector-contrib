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
	"regexp"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

type metricsTransformTest struct {
	name       string // test name
	transforms []internalTransform
	in         []*metricspb.Metric
	out        []*metricspb.Metric
}

var (
	// test cases
	standardTests = []metricsTransformTest{
		// UPDATE
		{
			name: "metric_name_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("new/metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
		},
		{
			name: "metric_name_update_chained",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					NewName:             "new/metric1",
				},
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric2"},
					Action:              Update,
					NewName:             "new/metric2",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("new/metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("new/metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
		},
		{
			name: "metric_names_update_chained",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^(metric)(?P<namedsubmatch>[12])$")},
					Action:              Update,
					NewName:             "new/$1/$namedsubmatch",
				},
				{
					MetricIncludeFilter: internalFilterStrict{include: "new/metric/1"},
					Action:              Update,
					NewName:             "new/new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("new/new/metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("new/metric/2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
		},
		{
			name: "metric_name_update_nonexist",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "nonexist"},
					Action:              Update,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_DOUBLE).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).build(),
			},
		},
		{
			name: "metric_label_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
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
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).
					setLabels([]string{"label1"}).
					addTimeseries(1, []string{"value1"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).
					setLabels([]string{"new/label1"}).
					addTimeseries(1, []string{"value1"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_label_value_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
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
				metricBuilder().setName("metric1").setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 3, 2).
					addTimeseries(1, []string{"label1-value2"}).
					addInt64Point(1, 3, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).
					addTimeseries(1, []string{"new/label1-value1"}).
					addInt64Point(0, 3, 2).
					addTimeseries(1, []string{"label1-value2"}).
					addInt64Point(1, 3, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_sum_int_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1", "label2-value1"}).
					addInt64Point(0, 3, 2).
					addTimeseries(2, []string{"label1-value1", "label2-value2"}).
					addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"label1-value1"}).
					addInt64Point(0, 4, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_mean_int_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Mean,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 2, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_max_int_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Max,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addTimeseries(1, []string{"label1-value1", "label2-value3"}).
					addInt64Point(0, 1, 2).addInt64Point(1, 3, 2).addInt64Point(2, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 3, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_min_int_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Min,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addTimeseries(1, []string{"label1-value1", "label2-value3"}).
					addInt64Point(0, 3, 2).
					addInt64Point(1, 1, 2).
					addInt64Point(2, 3, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 1, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_sum_double_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addDoublePoint(0, 3, 2).
					addDoublePoint(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1"}).
					addDoublePoint(0, 4, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_mean_double_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Mean,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addDoublePoint(0, 3, 2).
					addDoublePoint(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1"}).
					addDoublePoint(0, 2, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_max_double_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Max,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addDoublePoint(0, 3, 2).
					addDoublePoint(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1"}).
					addDoublePoint(0, 3, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_min_double_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Min,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(2, []string{"label1-value2", "label2-value2"}).addDoublePoint(0, 3, 2).
					addTimeseries(0, []string{"label1-value0", "label2-value0"}).addDoublePoint(1, 4, 2).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).addDoublePoint(2, 1, 2).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).addDoublePoint(3, 3, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, []string{"label1-value1"}).addDoublePoint(0, 1, 2).
					addTimeseries(2, []string{"label1-value2"}).addDoublePoint(1, 3, 2).
					addTimeseries(0, []string{"label1-value0"}).addDoublePoint(2, 4, 2).
					build(),
			},
		},
		{
			name: "metric_label_values_aggregation_sum_int_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabelValues,
								NewValue:        "new/label2-value",
								AggregationType: Sum,
								Label:           "label2",
							},
							aggregatedValuesSet: map[string]bool{"label2-value1": true, "label2-value2": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, []string{"label1-value1", "label2-value1"}).
					addInt64Point(0, 3, 2).
					addTimeseries(3, []string{"label1-value1", "label2-value2"}).
					addInt64Point(1, 1, 2).addInt64Point(1, 2, 3).
					addTimeseries(1, []string{"label1-value1", "label2-value3"}).
					addInt64Point(2, 1, 2).
					addTimeseries(0, []string{"label1-value1", "label2-value4"}).
					addInt64Point(3, 4, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "label2-value3"}).
					addInt64Point(0, 1, 2).
					addTimeseries(3, []string{"label1-value1", "new/label2-value"}).
					addInt64Point(1, 4, 2).
					addTimeseries(3, []string{"label1-value1", "new/label2-value"}).
					addInt64Point(2, 2, 3).
					addTimeseries(0, []string{"label1-value1", "label2-value4"}).
					addInt64Point(3, 4, 2).
					build(),
			},
		},
		// this test case also tests the correctness of the SumOfSquaredDeviation merging
		{
			name: "metric_label_values_aggregation_sum_distribution_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addTimeseries(1, []string{"label1-value1", "label2-value3"}).
					addDistributionPoints(0, 3, 6, []float64{1, 2, 3}, []int64{0, 1, 1, 1}).  // pointGroup1: {1, 2, 3}, SumOfSquaredDeviation = 2
					addDistributionPoints(1, 5, 10, []float64{1, 2, 3}, []int64{0, 2, 1, 2}). // pointGroup2: {1, 2, 3, 3, 1}, SumOfSquaredDeviation = 4
					addDistributionPoints(2, 7, 14, []float64{1, 2, 3}, []int64{0, 3, 1, 3}). // pointGroup3: {1, 1, 2, 3, 3, 1, 3}, SumOfSquaredDeviation = 6
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1"}).
					addDistributionPoints(0, 15, 30, []float64{1, 2, 3}, []int64{0, 6, 3, 6}). // pointGroupCombined: {1, 2, 3, 1, 2, 3, 3, 1, 1, 1, 2, 3, 3, 1, 3}, SumOfSquaredDeviation = 12
					build(),
			},
		},
		{
			name: "metric_label_values_aggregation_not_sum_distribution_update",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Mean,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addDistributionPoints(0, 3, 6, []float64{1, 2, 3}, []int64{0, 1, 1, 1}).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1"}).
					addDistributionPoints(0, 3, 6, []float64{1, 2, 3}, []int64{0, 1, 1, 1}).
					build(),
			},
		},
		// INSERT
		{
			name: "metric_name_insert",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("new/metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
		},
		{
			name: "metric_name_insert_multiple",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Insert,
					NewName:             "new/metric1",
				},
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric2"},
					Action:              Insert,
					NewName:             "new/metric2",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("metric2").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("metric2").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("new/metric1").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
				metricBuilder().setName("new/metric2").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_strict",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1", matchLabels: map[string]StringMatcher{"label1": strictMatcher("value1")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
				metricBuilder().setName("new/metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"), matchLabels: map[string]StringMatcher{"label1": regexp.MustCompile(`(.|\s)*\S(.|\s)*`)}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
				metricBuilder().setName("new/metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_two_datapoints_positive",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"), matchLabels: map[string]StringMatcher{"label1": regexp.MustCompile("value3")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addTimeseries(2, []string{"value3", "value4"}).
					addInt64Point(0, 3, 2).
					addInt64Point(1, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addTimeseries(2, []string{"value3", "value4"}).
					addInt64Point(0, 3, 2).
					addInt64Point(1, 3, 2).build(),
				metricBuilder().setName("new/metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, []string{"value3", "value4"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_two_datapoints_negative",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"), matchLabels: map[string]StringMatcher{"label1": regexp.MustCompile("value3")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addTimeseries(2, []string{"value11", "value22"}).
					addInt64Point(0, 3, 2).
					addInt64Point(1, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addTimeseries(2, []string{"value11", "value22"}).
					addInt64Point(0, 3, 2).
					addInt64Point(1, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_with_full_value",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"), matchLabels: map[string]StringMatcher{"label1": regexp.MustCompile("value1")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
				metricBuilder().setName("new/metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_strict_negative",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1", matchLabels: map[string]StringMatcher{"label1": strictMatcher("wrong_value")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_negative",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"), matchLabels: map[string]StringMatcher{"label1": regexp.MustCompile(".*wrong_ending")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_strict_missing_key",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1", matchLabels: map[string]StringMatcher{"missing_key": strictMatcher("value1")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_missing_key",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"), matchLabels: map[string]StringMatcher{"missing_key": regexp.MustCompile("value1")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_missing_and_present_key",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"), matchLabels: map[string]StringMatcher{"label1": regexp.MustCompile("value1"), "missing_key": regexp.MustCompile("value2")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_missing_key_with_empty_expression",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"), matchLabels: map[string]StringMatcher{"label1": regexp.MustCompile("value1"), "missing_key": regexp.MustCompile("^$")}},
					Action:              Insert,
					NewName:             "new/metric1",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
				metricBuilder().setName("new/metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_label_update_with_metric_insert",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Insert,
					NewName:             "new/metric1",
					Operations: []internalOperation{
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
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).build(),
				metricBuilder().setName("new/metric1").
					setLabels([]string{"label2", "new/label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value2", "value1"}).
					addInt64Point(0, 3, 2).build(),
			},
		},
		{
			name: "metric_label_value_update_with_metric_insert",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Insert,
					NewName:             "new/metric1",
					Operations: []internalOperation{
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
				metricBuilder().setName("metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 3, 2).
					addTimeseries(1, []string{"label1-value2"}).
					addInt64Point(1, 4, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 3, 2).
					addTimeseries(1, []string{"label1-value2"}).
					addInt64Point(1, 4, 2).
					build(),

				metricBuilder().setName("new/metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"new/label1-value1"}).
					addInt64Point(0, 3, 2).
					addTimeseries(1, []string{"label1-value2"}).
					addInt64Point(1, 4, 2).
					build(),
			},
		},
		{
			name: "metric_label_aggregation_sum_int_insert",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Insert,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).
					addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
				metricBuilder().setName("metric1").
					setLabels([]string{"label1"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1"}).
					addInt64Point(0, 4, 2).
					build(),
			},
		},
		{
			name: "metric_label_values_aggregation_sum_int_insert",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Insert,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabelValues,
								NewValue:        "new/label2-value",
								AggregationType: Sum,
								Label:           "label2",
							},
							aggregatedValuesSet: map[string]bool{"label2-value1": true, "label2-value2": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addInt64Point(0, 3, 2).addInt64Point(1, 1, 2).
					build(),
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1-value1", "new/label2-value"}).
					addInt64Point(0, 4, 2).
					build(),
			},
		},
		{
			name: "metric_labels_aggregation_sum_distribution_insert",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Insert,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
								LabelSet:        []string{"label1"},
							},
							labelSetMap: map[string]bool{"label1": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addDistributionPoints(0, 3, 6, []float64{1, 2}, []int64{0, 1, 2}).
					addDistributionPoints(1, 5, 10, []float64{1, 2}, []int64{1, 1, 3}).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).setDataType(metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1", "label2-value1"}).addTimeseries(1, []string{"label1-value1", "label2-value2"}).
					addDistributionPoints(0, 3, 6, []float64{1, 2}, []int64{0, 1, 2}).
					addDistributionPoints(1, 5, 10, []float64{1, 2}, []int64{1, 1, 3}).
					build(),
				metricBuilder().setName("metric1").setLabels([]string{"label1"}).setDataType(metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION).
					addTimeseries(1, []string{"label1-value1"}).
					addDistributionPoints(0, 8, 16, []float64{1, 2}, []int64{1, 2, 5}).
					build(),
			},
		},
		// COMBINE
		{
			name: "combine",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^([mM]etric)(?P<namedsubmatch>[12])$")},
					Action:              Combine,
					NewName:             "new",
					SubmatchCase:        "lower",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("Metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, nil).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
				metricBuilder().setName("new").
					setLabels([]string{"$1", "namedsubmatch"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"metric", "1"}).addInt64Point(0, 1, 1).
					addTimeseries(2, []string{"metric", "2"}).addInt64Point(1, 2, 1).
					build(),
			},
		},
		{
			name: "combine_no_matches",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^X(metric)(?P<namedsubmatch>[12])$")},
					Action:              Combine,
					NewName:             "new",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, nil).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, nil).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
		},
		{
			name: "combine_single_match",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^([mM]etric)(?P<namedsubmatch>[1])$")},
					Action:              Combine,
					NewName:             "new",
					SubmatchCase:        "upper",
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("Metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, nil).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(2, nil).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
				metricBuilder().setName("new").
					setLabels([]string{"$1", "namedsubmatch"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"METRIC", "1"}).addInt64Point(0, 1, 1).
					build(),
			},
		},
		{
			name: "combine_aggregate",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^metric[12]$")},
					Action:              Combine,
					NewName:             "new",
					AggregationType:     Sum,
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
				metricBuilder().setName("new").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 3, 1).
					build(),
			},
		},
		{
			name: "combine_with_operations",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^(metric)(?P<namedsubmatch>[12])$")},
					Action:              Combine,
					NewName:             "new",
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:   AddLabel,
								NewLabel: "new_label",
								NewValue: "new_label_value",
							},
						},
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
								LabelSet:        []string{"$1", "new_label"},
							},
							labelSetMap: map[string]bool{"$1": true, "new_label": true},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
				metricBuilder().setName("new").
					setLabels([]string{"$1", "new_label"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"metric", "new_label_value"}).addInt64Point(0, 3, 1).
					build(),
			},
		},
		{
			name: "combine_error_type",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^metric[12]$")},
					Action:              Combine,
					NewName:             "new",
					AggregationType:     Sum,
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, nil).addDoublePoint(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, nil).addDoublePoint(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
		},
		{
			name: "combine_error_units",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^metric[12]$")},
					Action:              Combine,
					NewName:             "new",
					AggregationType:     Sum,
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).setUnit("ms").
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).setUnit("s").
					addTimeseries(1, nil).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).setUnit("ms").
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).setUnit("s").
					addTimeseries(1, nil).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
		},
		{
			name: "combine_error_labels1",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^metric[12]$")},
					Action:              Combine,
					NewName:             "new",
					AggregationType:     Sum,
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"a", "b"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"1", "2"}).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setLabels([]string{"a", "b", "c"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"1", "2", "3"}).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"a", "b"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"1", "2"}).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setLabels([]string{"a", "b", "c"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"1", "2", "3"}).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
		},
		{
			name: "combine_error_labels2",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^metric[12]$")},
					Action:              Combine,
					NewName:             "new",
					AggregationType:     Sum,
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"a", "b"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"1", "2"}).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setLabels([]string{"a", "c"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"1", "3"}).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setLabels([]string{"a", "b"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"1", "2"}).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").
					setLabels([]string{"a", "c"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"1", "3"}).addInt64Point(0, 2, 1).
					build(),
				metricBuilder().setName("metric3").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(3, nil).addInt64Point(0, 3, 1).
					build(),
			},
		},
		// Toggle Data Type
		{
			name: "metric_toggle_scalar_data_type_int64_to_double",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ToggleScalarDataType,
							},
						},
					},
				},
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric2"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ToggleScalarDataType,
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).build(),
				metricBuilder().setName("metric2").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).build(),
				metricBuilder().setName("metric2").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
		},
		{
			name: "metric_toggle_scalar_data_type_double_to_int64",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ToggleScalarDataType,
							},
						},
					},
				},
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric2"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ToggleScalarDataType,
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_DOUBLE).build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).build(),
				metricBuilder().setName("metric2").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).build(),
			},
		},
		{
			name: "metric_toggle_scalar_data_type_no_effect",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ToggleScalarDataType,
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION).build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION).build(),
			},
		},
		// Scale Value
		{
			name: "metric_experimental_scale_value_int64",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ScaleValue,
								Scale:  100,
							},
						},
					},
				},
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric2"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ScaleValue,
								Scale:  10,
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 3, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setDataType(metricspb.MetricDescriptor_CUMULATIVE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 100, 1).
					build(),
				metricBuilder().setName("metric2").setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).addInt64Point(0, 30, 1).
					build(),
			},
		},
		{
			name: "metric_experimental_scale_value_double",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ScaleValue,
								Scale:  100,
							},
						},
					},
				},
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric2"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ScaleValue,
								Scale:  .1,
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setDataType(metricspb.MetricDescriptor_CUMULATIVE_DOUBLE).
					addTimeseries(1, nil).addDoublePoint(0, 1, 1).
					build(),
				metricBuilder().setName("metric2").setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, nil).addDoublePoint(0, 300, 1).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setDataType(metricspb.MetricDescriptor_CUMULATIVE_DOUBLE).
					addTimeseries(1, nil).addDoublePoint(0, 100, 1).
					build(),
				metricBuilder().setName("metric2").setDataType(metricspb.MetricDescriptor_GAUGE_DOUBLE).
					addTimeseries(1, nil).addDoublePoint(0, 30, 1).
					build(),
			},
		},
		// Add Label to a metric
		{
			name: "update existing metric by adding a new label when there are no labels",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:   AddLabel,
								NewLabel: "foo",
								NewValue: "bar",
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, nil).
					addInt64Point(0, 3, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"foo"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"bar"}).
					addInt64Point(0, 3, 2).
					build(),
			},
		},
		{
			name: "update existing metric by adding a new label when there are labels",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:   AddLabel,
								NewLabel: "foo",
								NewValue: "bar",
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"foo", "label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"bar", "value1", "value2"}).
					addInt64Point(0, 3, 2).
					build(),
			},
		},
		{
			name: "update existing metric by adding a label that is duplicated in the list",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:   AddLabel,
								NewLabel: "label1",
								NewValue: "value3",
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).
					build(),
			},
		},
		{
			name: "update_does_not_happen_because_target_metric_doesn't_exist",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "mymetric"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:   AddLabel,
								NewLabel: "foo",
								NewValue: "bar",
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric1").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"value1", "value2"}).
					addInt64Point(0, 3, 2).
					build(),
			},
		},
		// delete label value
		{
			name: "delete_a_label_value",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric"},
					Action:              Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:     DeleteLabelValue,
								Label:      "label1",
								LabelValue: "label1value1",
							},
						},
					},
				},
			},
			in: []*metricspb.Metric{
				metricBuilder().setName("metric").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1value1", "label2value"}).
					addInt64Point(0, 3, 2).
					addTimeseries(1, []string{"label1value2", "label2value"}).
					addInt64Point(1, 4, 2).
					build(),
			},
			out: []*metricspb.Metric{
				metricBuilder().setName("metric").setLabels([]string{"label1", "label2"}).
					setDataType(metricspb.MetricDescriptor_GAUGE_INT64).
					addTimeseries(1, []string{"label1value2", "label2value"}).
					addInt64Point(0, 4, 2).
					build(),
			},
		},
	}
)
