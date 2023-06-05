// Copyright The OpenTelemetry Authors
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

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type metricsTransformTest struct {
	name       string // test name
	transforms []internalTransform
	in         []pmetric.Metric
	out        []pmetric.Metric
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric2").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "new/new/metric1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric/2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 3, "value1").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "new/label1").
					addIntDatapoint(1, 2, 3, "value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 3, "label1-value1").
					addIntDatapoint(1, 2, 3, "label1-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 3, "new/label1-value1").
					addIntDatapoint(1, 2, 3, "label1-value2").build(),
			},
		},
		{
			name: "metric_label_update_label_and_label_value",
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
							valueActionsMapping: map[string]string{"label1-value1": "new/label1-value1"},
						},
					},
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 3, "label1-value1").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "new/label1").
					addIntDatapoint(1, 2, 3, "new/label1-value1").build(),
			},
		},
		{
			name: "metric_label_update_with_regexp_filter",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^matched.*$")},
					Action:              Update,
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "matched-metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "unmatched-metric2", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "matched-metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "new/label1-value1", "label2-value1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "unmatched-metric2", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value2").addDescription("foobar").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 4, "label1-value1").addDescription("foobar").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 2, "label1-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value1").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value2").
					addIntDatapoint(1, 2, 2, "label1-value1", "label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 3, "label1-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value1").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value2").
					addIntDatapoint(1, 2, 2, "label1-value1", "label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 1, "label1-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 4, "label1-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addDoubleDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addDoubleDatapoint(1, 2, 1, "label1-value1", "label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addDoubleDatapoint(1, 2, 2, "label1-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addDoubleDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addDoubleDatapoint(1, 2, 1, "label1-value1", "label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addDoubleDatapoint(1, 2, 3, "label1-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addDoubleDatapoint(2, 2, 3, "label1-value2", "label2-value2").
					addDoubleDatapoint(0, 2, 4, "label1-value0", "label2-value0").
					addDoubleDatapoint(1, 2, 1, "label1-value1", "label2-value1").
					addDoubleDatapoint(1, 2, 3, "label1-value1", "label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addDoubleDatapoint(2, 2, 3, "label1-value2").
					addDoubleDatapoint(0, 2, 4, "label1-value0").
					addDoubleDatapoint(1, 2, 1, "label1-value1").build(),
			},
		},
		{
			name: "metric_label_aggregation_insert_sum_with_several_attrs_match",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1",
						attrMatchers: map[string]StringMatcher{"label0": strictMatcher("label0-value1")}},
					Action:  Insert,
					NewName: "new/metric1",
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action:          AggregateLabels,
								AggregationType: Sum,
								LabelSet:        []string{"label1", "label2"},
							},
							labelSetMap: map[string]bool{"label1": true, "label2": true},
						},
					},
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label0", "label1", "label2", "label3").
					addDoubleDatapoint(1, 2, 3, "label0-value1", "label1-value1", "label2-value1", "label3-value1").
					addDoubleDatapoint(1, 2, 1, "label0-value1", "label1-value1", "label2-value1", "label3-value2").
					addDoubleDatapoint(1, 2, 1, "label0-value2", "label1-value1", "label2-value1", "label3-value1").
					build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label0", "label1", "label2", "label3").
					addDoubleDatapoint(1, 2, 3, "label0-value1", "label1-value1", "label2-value1", "label3-value1").
					addDoubleDatapoint(1, 2, 1, "label0-value1", "label1-value1", "label2-value1", "label3-value2").
					addDoubleDatapoint(1, 2, 1, "label0-value2", "label1-value1", "label2-value1", "label3-value1").
					build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1", "label1", "label2").
					addDoubleDatapoint(1, 2, 4, "label1-value1", "label2-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(0, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(0, 2, 1, "label1-value1", "label2-value2").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value3").
					addIntDatapoint(2, 2, 4, "label1-value1", "label2-value4").
					build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(0, 2, 4, "label1-value1", "new/label2-value").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value3").
					addIntDatapoint(2, 2, 4, "label1-value1", "label2-value4").
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1", "label1", "label2").
					addHistogramDatapoint(0, 1, 3, 6, []float64{1, 2, 3}, []uint64{0, 1, 1, 1}, "label1-value1",
						"label2-value1"). // pointGroup1: {1, 2, 3}, SumOfSquaredDeviation = 2
					addHistogramDatapoint(0, 1, 5, 10, []float64{1, 2, 3}, []uint64{0, 2, 1, 2}, "label1-value1",
						"label2-value2"). // pointGroup2: {1, 2, 3, 3, 1}, SumOfSquaredDeviation = 4
					addHistogramDatapoint(1, 1, 7, 14, []float64{1, 2, 3}, []uint64{0, 3, 1, 3}, "label1-value1",
						"label2-value3"). // pointGroup3: {1, 1, 2, 3, 3, 1, 3}, SumOfSquaredDeviation = 6
					build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1", "label1").
					addHistogramDatapoint(0, 1, 15, 30, []float64{1, 2, 3}, []uint64{0, 6, 3, 6},
						"label1-value1"). // pointGroupCombined: {1, 2, 3, 1, 2, 3, 3, 1, 1, 1, 2, 3, 3, 1, 3}, SumOfSquaredDeviation = 12
					build(),
			},
		},
		{
			name: "metric_label_aggregation_ignored_for_partial_metric_match",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1",
						attrMatchers: map[string]StringMatcher{"label1": strictMatcher("label1-value1")}},
					Action: Update,
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(0, 2, 1, "label1-value2", "label2-value2").
					build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(0, 2, 1, "label1-value2", "label2-value2").
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1", "label1", "label2").
					addHistogramDatapoint(1, 2, 3, 6, []float64{1, 2, 3}, []uint64{0, 1, 1, 1}, "label1-value1",
						"label2-value1").
					build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1", "label1").
					addHistogramDatapoint(1, 2, 3, 6, []float64{1, 2, 3}, []uint64{0, 1, 1, 1}, "label1-value1").
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric2").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_strict",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1",
						attrMatchers: map[string]StringMatcher{"label1": strictMatcher("value1")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 3, 2, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 3, 2, "value1", "value2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1", "label1", "label2").
					addIntDatapoint(1, 3, 2, "value1", "value2").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"),
						attrMatchers: map[string]StringMatcher{"label1": regexp.MustCompile(`(.|\s)*\S(.|\s)*`)}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_two_datapoints_positive",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"),
						attrMatchers: map[string]StringMatcher{"label1": regexp.MustCompile("value3")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").
					addIntDatapoint(2, 2, 3, "value3", "value4").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").
					addIntDatapoint(2, 2, 3, "value3", "value4").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1", "label1", "label2").
					addIntDatapoint(2, 2, 3, "value3", "value4").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_two_datapoints_negative",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"),
						attrMatchers: map[string]StringMatcher{"label1": regexp.MustCompile("value3")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").
					addIntDatapoint(2, 2, 3, "value11", "value22").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").
					addIntDatapoint(2, 2, 3, "value11", "value22").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_with_full_value",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"),
						attrMatchers: map[string]StringMatcher{"label1": regexp.MustCompile("value1")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_strict_negative",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1",
						attrMatchers: map[string]StringMatcher{"label1": strictMatcher("wrong_value")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_negative",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"),
						attrMatchers: map[string]StringMatcher{"label1": regexp.MustCompile(".*wrong_ending")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_strict_missing_key",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1",
						attrMatchers: map[string]StringMatcher{"missing_key": strictMatcher("value1")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_missing_key",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"),
						attrMatchers: map[string]StringMatcher{"missing_key": regexp.MustCompile("value1")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_missing_and_present_key",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"),
						attrMatchers: map[string]StringMatcher{"label1": regexp.MustCompile("value1"),
							"missing_key": regexp.MustCompile("value2")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
		},
		{
			name: "metric_name_insert_with_match_label_regexp_missing_key_with_empty_expression",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("metric1"),
						attrMatchers: map[string]StringMatcher{"label1": regexp.MustCompile("value1"),
							"missing_key": regexp.MustCompile("^$")}},
					Action:  Insert,
					NewName: "new/metric1",
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1", "new/label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 3, "label1-value1").
					addIntDatapoint(1, 2, 4, "label1-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 3, "label1-value1").
					addIntDatapoint(1, 2, 4, "label1-value2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "new/metric1", "label1").
					addIntDatapoint(1, 2, 3, "new/label1-value1").
					addIntDatapoint(1, 2, 4, "label1-value2").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
					addIntDatapoint(1, 2, 4, "label1-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
					addIntDatapoint(1, 2, 1, "label1-value1", "label2-value2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 4, "label1-value1", "new/label2-value").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1", "label1", "label2").
					addHistogramDatapoint(1, 2, 3, 6, []float64{1, 2}, []uint64{0, 1, 2}, "label1-value1",
						"label2-value1").
					addHistogramDatapoint(1, 2, 5, 10, []float64{1, 2}, []uint64{1, 1, 3}, "label1-value1",
						"label2-value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1", "label1", "label2").
					addHistogramDatapoint(1, 2, 3, 6, []float64{1, 2}, []uint64{0, 1, 2}, "label1-value1",
						"label2-value1").
					addHistogramDatapoint(1, 2, 5, 10, []float64{1, 2}, []uint64{1, 1, 3}, "label1-value1",
						"label2-value2").build(),
				metricBuilder(pmetric.MetricTypeHistogram, "metric1", "label1").
					addHistogramDatapoint(1, 2, 8, 16, []float64{1, 2}, []uint64{1, 2, 5}, "label1-value1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "Metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
				metricBuilder(pmetric.MetricTypeGauge, "new", "$1", "namedsubmatch").
					addIntDatapoint(1, 1, 1, "metric", "1").
					addIntDatapoint(1, 1, 2, "metric", "2").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "Metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
				metricBuilder(pmetric.MetricTypeGauge, "new", "$1", "namedsubmatch").addIntDatapoint(1, 1, 1, "METRIC", "1").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
				metricBuilder(pmetric.MetricTypeGauge, "new").addIntDatapoint(1, 1, 3).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
				metricBuilder(pmetric.MetricTypeGauge, "new", "$1", "new_label").
					addIntDatapoint(1, 1, 3, "metric", "new_label_value").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeSum, "metric2").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeSum, "metric2").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").setUnit("s").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").setUnit("ms").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").setUnit("s").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").setUnit("ms").addIntDatapoint(1, 1, 2).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "a", "b").addIntDatapoint(1, 1, 1, "1", "2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2", "a", "b", "c").
					addIntDatapoint(1, 1, 2, "1", "2", "3").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "a", "b").addIntDatapoint(1, 1, 1, "1", "2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2", "a", "b", "c").
					addIntDatapoint(1, 1, 2, "1", "2", "3").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "a", "b").addIntDatapoint(1, 1, 1, "1", "2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2", "a", "c").addIntDatapoint(1, 1, 2, "1", "3").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "a", "b").addIntDatapoint(1, 1, 1, "1", "2").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2", "a", "c").addIntDatapoint(1, 1, 2, "1", "3").build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric3").addIntDatapoint(1, 1, 3).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 1).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1").addDoubleDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addDoubleDatapoint(1, 1, 1).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1").addDoubleDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addDoubleDatapoint(1, 1, 1).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 1).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1").
					addHistogramDatapoint(0, 2, 3, 6, []float64{1, 2, 3}, []uint64{0, 1, 1, 1}).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1").
					addHistogramDatapoint(0, 2, 3, 6, []float64{1, 2, 3}, []uint64{0, 1, 1, 1}).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1").addIntDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1").addIntDatapoint(1, 1, 100).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addIntDatapoint(1, 1, 30).build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1").addDoubleDatapoint(1, 1, 1).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addDoubleDatapoint(1, 1, 300).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1").addDoubleDatapoint(1, 1, 100).build(),
				metricBuilder(pmetric.MetricTypeGauge, "metric2").addDoubleDatapoint(1, 1, 30).build(),
			},
		},
		{
			name: "metric_experimental_scale_value_histogram",
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1").
					addHistogramDatapoint(1, 1, 1, 1, []float64{1}, []uint64{1, 1}).build(),
				metricBuilder(pmetric.MetricTypeHistogram, "metric2").
					addHistogramDatapoint(1, 1, 2, 400, []float64{200}, []uint64{1, 2}).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1").
					addHistogramDatapoint(1, 1, 1, 100, []float64{100}, []uint64{1, 1}).build(),
				metricBuilder(pmetric.MetricTypeHistogram, "metric2").
					addHistogramDatapoint(1, 1, 2, 40, []float64{20}, []uint64{1, 2}).build(),
			},
		},
		{
			name: "metric_experimental_scale_value_histogram_with_exemplars",
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1").
					addHistogramDatapointWithMinMaxAndExemplars(1, 1, 1, 1, 1, 1, []float64{1}, []uint64{1, 1}, []float64{1}).build(),
				metricBuilder(pmetric.MetricTypeHistogram, "metric2").
					addHistogramDatapointWithMinMaxAndExemplars(2, 2, 2, 400, 100, 300, []float64{200}, []uint64{1, 2}, []float64{100, 300}).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeHistogram, "metric1").
					addHistogramDatapointWithMinMaxAndExemplars(1, 1, 1, 100, 100, 100, []float64{100}, []uint64{1, 1}, []float64{100}).build(),
				metricBuilder(pmetric.MetricTypeHistogram, "metric2").
					addHistogramDatapointWithMinMaxAndExemplars(2, 2, 2, 40, 10, 30, []float64{20}, []uint64{1, 2}, []float64{10, 30}).build(),
			},
		},
		{
			name: "metric_experimental_scale_with_attr_filtering",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric1",
						attrMatchers: map[string]StringMatcher{"label1": strictMatcher("value1")}},
					Action: Update,
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
					MetricIncludeFilter: internalFilterStrict{include: "metric2",
						attrMatchers: map[string]StringMatcher{"label1": strictMatcher("value1")}},
					Action: Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ScaleValue,
								Scale:  10,
							},
						},
					},
				},
				{
					MetricIncludeFilter: internalFilterStrict{include: "metric3",
						attrMatchers: map[string]StringMatcher{"label1": strictMatcher("value1")}},
					Action: Update,
					Operations: []internalOperation{
						{
							configOperation: Operation{
								Action: ScaleValue,
								Scale:  0.1,
							},
						},
					},
				},
			},
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1", "label1").
					addIntDatapoint(1, 1, 1, "value1").
					addIntDatapoint(1, 1, 3, "value2").build(),
				metricBuilder(pmetric.MetricTypeHistogram, "metric2", "label1").
					addHistogramDatapoint(1, 1, 1, 1, []float64{1}, []uint64{1, 1}, "value1").
					addHistogramDatapoint(1, 1, 2, 4, []float64{2}, []uint64{1, 2}, "value2").build(),
				metricBuilder(pmetric.MetricTypeHistogram, "metric3", "label1").
					addHistogramDatapointWithMinMaxAndExemplars(1, 1, 1, 1, 1, 1, []float64{1}, []uint64{1, 1}, []float64{1}, "value1").
					addHistogramDatapointWithMinMaxAndExemplars(2, 2, 1, 1, 1, 1, []float64{1}, []uint64{1, 1}, []float64{1}, "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeSum, "metric1", "label1").
					addIntDatapoint(1, 1, 100, "value1").
					addIntDatapoint(1, 1, 3, "value2").build(),
				metricBuilder(pmetric.MetricTypeHistogram, "metric2", "label1").
					addHistogramDatapoint(1, 1, 1, 10, []float64{10}, []uint64{1, 1}, "value1").
					addHistogramDatapoint(1, 1, 2, 4, []float64{2}, []uint64{1, 2}, "value2").build(),
				metricBuilder(pmetric.MetricTypeHistogram, "metric3", "label1").
					addHistogramDatapointWithMinMaxAndExemplars(1, 1, 1, 0.1, 0.1, 0.1, []float64{0.1}, []uint64{1, 1}, []float64{0.1}, "value1").
					addHistogramDatapointWithMinMaxAndExemplars(2, 2, 1, 1, 1, 1, []float64{1}, []uint64{1, 1}, []float64{1}, "value2").build(),
			},
		},
		// Add Label to a metric
		{
			name: "update_existing_metric_by_adding_a_new_label_when_there_are_no_labels",
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1").addIntDatapoint(1, 2, 3).build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "foo").addIntDatapoint(1, 2, 3, "bar").build(),
			},
		},
		{
			name: "update_existing_metric_by_adding_a_new_label_when_there_are_labels",
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2", "foo").
					addIntDatapoint(1, 2, 3, "value1", "value2", "bar").build(),
			},
		},
		{
			name: "update_existing_metric_by_adding_a_label_that_is_duplicated_in_the_list",
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
					addIntDatapoint(1, 2, 3, "value1", "value2").build(),
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1value1", "label2value").
					addIntDatapoint(1, 2, 4, "label1value2", "label2value").build(),
			},
			out: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric", "label1", "label2").
					addIntDatapoint(1, 2, 4, "label1value2", "label2value").build(),
			},
		},
		{
			name: "delete_all_metric_datapoints",
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
			in: []pmetric.Metric{
				metricBuilder(pmetric.MetricTypeGauge, "metric", "label1", "label2").
					addIntDatapoint(1, 2, 3, "label1value1", "label2value").build(),
			},
			out: []pmetric.Metric{},
		},
	}
)
