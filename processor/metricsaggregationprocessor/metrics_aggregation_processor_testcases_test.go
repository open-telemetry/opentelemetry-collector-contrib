// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaggregationprocessor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type metricsaggregationTest struct {
	name       string // test name
	transforms []internalTransform
	in         []pmetric.Metric
	out        []pmetric.Metric
}

const (
	ts = 1_000_000_000 // in nanos
)

// greedy sum free sequence
// 1, 3, 4, 5, 7, 9, 10, 11, 12, 13, 15, ...

var aggregationTests = []metricsaggregationTest{
	{
		name: "agg_combine_gauge_sum_15s",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Interval:            ts * 15,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Sum,
							LabelSet:        []string{"label1"},
						},
						labelSetMap: map[string]bool{"label1": true},
					},
				},
			},
		},
		in: []pmetric.Metric{
			metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
				addIntDatapoint(ts*00, ts*01, 1, "label1-value1", "label2-value1").
				addIntDatapoint(ts*02, ts*02, 3, "label1-value1", "label2-value2").
				addIntDatapoint(ts*16, ts*18, 4, "label1-value1", "label2-value1").
				addIntDatapoint(ts*17, ts*16, 5, "label1-value1", "label2-value2").
				addIntDatapoint(ts*30, ts*44, 7, "label1-value1", "label2-value1").
				addIntDatapoint(ts*30, ts*30, 9, "label1-value1", "label2-value2").
				addIntDatapoint(ts*285, ts*298, 10, "label1-value1", "label2-value1"). // 15s interval
				addIntDatapoint(ts*000, ts*299, 11, "label1-value1", "label2-value2"). //  5m interval
				addDescription("foobar").build(),
		},
		out: []pmetric.Metric{
			metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
				addIntDatapoint(ts*00, ts*15, 4, "label1-value1").
				addIntDatapoint(ts*15, ts*30, 9, "label1-value1").
				addIntDatapoint(ts*30, ts*45, 16, "label1-value1").
				addIntDatapoint(ts*0, ts*300, 21, "label1-value1").
				addDescription("foobar").build(),
		},
	},
	{
		name: "agg_combine_counter_sum_15s",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Interval:            ts * 15,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Sum,
							LabelSet:        []string{"label1"},
						},
						labelSetMap: map[string]bool{"label1": true},
					},
				},
			},
		},
		in: []pmetric.Metric{
			metricBuilder(pmetric.MetricTypeSum, "metric1", "label1", "label2").
				addIntDatapoint(ts*00, ts*01, 1, "label1-value1", "label2-value1").
				addIntDatapoint(ts*02, ts*02, 3, "label1-value1", "label2-value2").
				addIntDatapoint(ts*16, ts*18, 4, "label1-value1", "label2-value1").
				addIntDatapoint(ts*17, ts*16, 5, "label1-value1", "label2-value2").
				addIntDatapoint(ts*30, ts*44, 7, "label1-value1", "label2-value1").
				addIntDatapoint(ts*30, ts*30, 9, "label1-value1", "label2-value2").
				addIntDatapoint(ts*285, ts*298, 10, "label1-value1", "label2-value1"). // 15s interval
				addIntDatapoint(ts*000, ts*299, 11, "label1-value1", "label2-value2"). //  5m interval
				addDescription("foobar").build(),
		},
		out: []pmetric.Metric{
			metricBuilder(pmetric.MetricTypeSum, "metric1", "label1").
				addIntDatapoint(ts*00, ts*15, 4, "label1-value1").
				addIntDatapoint(ts*15, ts*30, 9, "label1-value1").
				addIntDatapoint(ts*30, ts*45, 16, "label1-value1").
				addIntDatapoint(ts*0, ts*300, 21, "label1-value1").
				addDescription("foobar").build(),
		},
	},
}

// test cases (forked from MetricsTransformProcessor)
var standardTests = []metricsaggregationTest{
	{
		name: "metric_label_aggregation_sum_int_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Sum,
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
				addIntDatapoint(1, 2, 1, "label1-value1", "label2-value2").
				addDescription("foobar").build(),
		},
		out: []pmetric.Metric{
			metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
				addIntDatapoint(1, 2, 4, "label1-value1").
				addDescription("foobar").build(),
		},
	},
	{
		name: "metric_label_aggregation_mean_int_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Mean,
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
		name: "metric_label_aggregation_max_int_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Max,
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
		name: "metric_label_aggregation_count_int_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Count,
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
				addIntDatapoint(1, 2, 4, "label1-value1", "label2-value2").
				addIntDatapoint(1, 2, 2, "label1-value1", "label2-value3").build(),
		},
		out: []pmetric.Metric{
			metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
				addIntDatapoint(1, 2, 3, "label1-value1").build(),
		},
	},
	{
		name: "metric_label_aggregation_median_int_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Median,
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
				addIntDatapoint(1, 2, 4, "label1-value1", "label2-value2").
				addIntDatapoint(1, 2, 2, "label1-value1", "label2-value2").build(),
		},
		out: []pmetric.Metric{
			metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
				addIntDatapoint(1, 2, 2, "label1-value1").build(),
		},
	},
	{
		name: "metric_label_aggregation_min_int_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Min,
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
		name: "metric_label_aggregation_sum_double_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Sum,
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
		name: "metric_label_aggregation_mean_double_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Mean,
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
		name: "metric_label_aggregation_max_double_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Max,
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
		name: "metric_label_aggregation_count_double_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Count,
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
		name: "metric_label_aggregation_median_double_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Median,
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
		name: "metric_label_aggregation_min_double_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Min,
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
				MetricIncludeFilter: internalFilterStrict{
					include:      "metric1",
					attrMatchers: map[string]StringMatcher{"label0": strictMatcher("label0-value1")},
				},
				Action: Insert,
				//NewName: "new/metric1",
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Sum,
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
			metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
				addDoubleDatapoint(1, 2, 4, "label1-value1", "label2-value1").build(),
		},
	},
	{
		name: "metric_label_aggregation_empty_label_set",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Sum,
							LabelSet:        []string{},
						},
						labelSetMap: map[string]bool{},
					},
				},
			},
		},
		in: []pmetric.Metric{
			metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2", "label3").
				addIntDatapoint(0, 1, 1, "a", "b", "c").
				build(),
		},
		out: []pmetric.Metric{
			metricBuilder(pmetric.MetricTypeGauge, "metric1").
				addIntDatapoint(0, 1, 1).
				build(),
		},
	},
	{
		name: "metric_label_aggregation_ignored_for_partial_metric_match",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{
					include:      "metric1",
					attrMatchers: map[string]StringMatcher{"label1": strictMatcher("label1-value1")},
				},
				Action: Combine, // CRL was update
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Sum,
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
		out: []pmetric.Metric{ //points should NOT be combined because attrMatchers only matches one metric
			metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1").
				addIntDatapoint(1, 2, 3, "label1-value1").
				build(),
			// FIXME note, this test fails in metricstransformprocessor as well because attrMatchers is experimental
			//metricBuilder(pmetric.MetricTypeGauge, "metric1", "label1", "label2").
			//	addIntDatapoint(1, 2, 3, "label1-value1", "label2-value1").
			//	addIntDatapoint(0, 2, 1, "label1-value2", "label2-value2").
			//	build(),
		},
	},
	{
		name: "metric_label_values_aggregation_not_sum_distribution_combine",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Combine,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Mean,
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
	{
		name: "metric_label_aggregation_sum_int_insert",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Insert,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Sum,
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
		name: "metric_labels_aggregation_sum_distribution_insert",
		transforms: []internalTransform{
			{
				MetricIncludeFilter: internalFilterStrict{include: "metric1"},
				Action:              Insert,
				Operations: []internalOperation{
					{
						configOperation: Operation{
							Action:          aggregateLabels,
							AggregationType: aggregateutil.Sum,
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
}
