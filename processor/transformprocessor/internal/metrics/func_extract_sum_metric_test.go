// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

func getTestHistogramMetric() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyHistogram()
	metricInput.SetName("histogram_metric")
	input := metricInput.Histogram().DataPoints().AppendEmpty()
	input.SetCount(5)
	input.SetSum(12.34)

	input.BucketCounts().Append(2, 3)
	input.ExplicitBounds().Append(1)

	attrs := getTestAttributes()
	attrs.CopyTo(input.Attributes())
	return metricInput
}

func getTestExponentialHistogramMetric() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyExponentialHistogram()
	metricInput.SetName("exponential_histogram_metric")
	input := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
	input.SetScale(1)
	input.SetCount(5)
	input.SetSum(12.34)

	attrs := getTestAttributes()
	attrs.CopyTo(input.Attributes())
	return metricInput
}

func getTestSummaryMetric() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySummary()
	metricInput.SetName("summary_metric")
	input := metricInput.Summary().DataPoints().AppendEmpty()
	input.SetCount(100)
	input.SetSum(12.34)

	qVal1 := input.QuantileValues().AppendEmpty()
	qVal1.SetValue(1)
	qVal1.SetQuantile(.99)

	qVal2 := input.QuantileValues().AppendEmpty()
	qVal2.SetValue(2)
	qVal2.SetQuantile(.95)

	qVal3 := input.QuantileValues().AppendEmpty()
	qVal3.SetValue(3)
	qVal3.SetQuantile(.50)

	attrs := getTestAttributes()
	attrs.CopyTo(input.Attributes())
	return metricInput
}

func getTestGaugeMetric() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyGauge()
	metricInput.SetName("gauge_metric")
	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetIntValue(12)

	attrs := getTestAttributes()
	attrs.CopyTo(input.Attributes())
	return metricInput
}

func getTestAttributes() pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("test", "hello world")
	attrs.PutInt("test2", 3)
	attrs.PutBool("test3", true)
	return attrs
}

type histogramTestCase struct {
	name         string
	input        pmetric.Metric
	temporality  pmetric.AggregationTemporality
	monotonicity bool
	want         func(pmetric.MetricSlice)
}

func Test_extractSumMetric(t *testing.T) {
	tests := []histogramTestCase{
		{
			name:         "histogram",
			input:        getTestHistogramMetric(),
			temporality:  pmetric.AggregationTemporalityDelta,
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(false)

				sumMetric.SetName(histogramMetric.Name() + "_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(histogramMetric.Histogram().DataPoints().At(0).Sum())

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "histogram (monotonic)",
			input:        getTestHistogramMetric(),
			temporality:  pmetric.AggregationTemporalityDelta,
			monotonicity: true,
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)

				sumMetric.SetName(histogramMetric.Name() + "_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(histogramMetric.Histogram().DataPoints().At(0).Sum())

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "histogram (cumulative)",
			input:        getTestHistogramMetric(),
			temporality:  pmetric.AggregationTemporalityCumulative,
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sumMetric.Sum().SetIsMonotonic(false)

				sumMetric.SetName(histogramMetric.Name() + "_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(histogramMetric.Histogram().DataPoints().At(0).Sum())

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "exponential histogram",
			input:        getTestExponentialHistogramMetric(),
			temporality:  pmetric.AggregationTemporalityDelta,
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				expHistogramMetric := getTestExponentialHistogramMetric()
				expHistogramMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(false)

				sumMetric.SetName(expHistogramMetric.Name() + "_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Sum())

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "exponential histogram (monotonic)",
			input:        getTestExponentialHistogramMetric(),
			temporality:  pmetric.AggregationTemporalityDelta,
			monotonicity: true,
			want: func(metrics pmetric.MetricSlice) {
				expHistogramMetric := getTestExponentialHistogramMetric()
				expHistogramMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)

				sumMetric.SetName(expHistogramMetric.Name() + "_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Sum())

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "exponential histogram (cumulative)",
			input:        getTestExponentialHistogramMetric(),
			temporality:  pmetric.AggregationTemporalityCumulative,
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				expHistogramMetric := getTestExponentialHistogramMetric()
				expHistogramMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sumMetric.Sum().SetIsMonotonic(false)

				sumMetric.SetName(expHistogramMetric.Name() + "_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Sum())

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "summary",
			input:        getTestSummaryMetric(),
			temporality:  pmetric.AggregationTemporalityDelta,
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(false)

				sumMetric.SetName("summary_metric_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(12.34)

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "summary (monotonic)",
			input:        getTestSummaryMetric(),
			temporality:  pmetric.AggregationTemporalityDelta,
			monotonicity: true,
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)

				sumMetric.SetName("summary_metric_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(12.34)

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "summary (cumulative)",
			input:        getTestSummaryMetric(),
			temporality:  pmetric.AggregationTemporalityCumulative,
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sumMetric.Sum().SetIsMonotonic(false)

				sumMetric.SetName("summary_metric_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(12.34)

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "gauge (no op)",
			input:        getTestGaugeMetric(),
			temporality:  pmetric.AggregationTemporalityDelta,
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				gaugeMetric := getTestGaugeMetric()
				gaugeMetric.CopyTo(metrics.AppendEmpty())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualMetrics := pmetric.NewMetricSlice()
			tt.input.CopyTo(actualMetrics.AppendEmpty())

			evaluate, err := extractSumMetric(ottl.Enum(tt.temporality), tt.monotonicity)
			assert.NoError(t, err)

			var datapoint interface{}
			switch tt.input.Type() {
			case pmetric.MetricTypeHistogram:
				datapoint = tt.input.Histogram().DataPoints().At(0)
			case pmetric.MetricTypeExponentialHistogram:
				datapoint = tt.input.ExponentialHistogram().DataPoints().At(0)
			case pmetric.MetricTypeSummary:
				datapoint = tt.input.Summary().DataPoints().At(0)
			case pmetric.MetricTypeGauge:
				datapoint = tt.input.Gauge().DataPoints().At(0)
			}

			_, err = evaluate(nil, ottldatapoint.NewTransformContext(datapoint, tt.input, actualMetrics, pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			assert.Nil(t, err)

			expected := pmetric.NewMetricSlice()
			tt.want(expected)
			assert.Equal(t, expected, actualMetrics)
		})
	}
}

func Test_extractSumMetric_validation(t *testing.T) {
	tests := []struct {
		name    string
		aggTemp ottl.Enum
	}{
		{
			name:    "invalid aggregation temporality",
			aggTemp: ottl.Enum(pmetric.MetricTypeEmpty),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractSumMetric(tt.aggTemp, true)
			assert.Error(t, err, "unknown aggregation temporality: not a real aggregation temporality")
		})
	}
}
