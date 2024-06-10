// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_aggregateLabels(t *testing.T) {
	tests := []struct {
		name     string
		input    pmetric.Metric
		t        common.AggregationType
		labelSet map[string]bool
		want     func(pmetric.MetricSlice)
		wantErr  error
	}{
		{
			name:     "sum sum",
			input:    getTestSummaryMetric(),
			t:        common.Sum,
			labelSet: map[string]bool{},
			want:     nil,
			wantErr:  fmt.Errorf("aggregation function is not supported for Summary metrics"),
		},
		{
			name:  "sum sum",
			input: getTestSumMetricMultiple(),
			t:     common.Sum,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(150)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "sum max",
			input: getTestSumMetricMultiple(),
			t:     common.Max,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "sum min",
			input: getTestSumMetricMultiple(),
			t:     common.Min,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "sum mean",
			input: getTestSumMetricMultiple(),
			t:     common.Mean,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(75)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "sum median even",
			input: getTestSumMetricMultiple(),
			t:     common.Median,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(75)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "sum median odd",
			input: getTestSumMetricMultipleOdd(),
			t:     common.Median,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "gauge sum",
			input: getTestGaugeMetricMultiple(),
			t:     common.Sum,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(17)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "gauge min",
			input: getTestGaugeMetricMultiple(),
			t:     common.Min,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(5)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "gauge max",
			input: getTestGaugeMetricMultiple(),
			t:     common.Max,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(12)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "gauge mean",
			input: getTestGaugeMetricMultiple(),
			t:     common.Mean,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(8)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "gauge median even",
			input: getTestGaugeMetricMultiple(),
			t:     common.Median,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(8)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "gauge median odd",
			input: getTestGaugeMetricMultipleOdd(),
			t:     common.Median,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(5)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "histogram",
			input: getTestHistogramMetricMultiple(),
			t:     common.Sum,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyHistogram()
				metricInput.SetName("histogram_metric")
				metricInput.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

				input := metricInput.Histogram().DataPoints().AppendEmpty()
				input.SetCount(10)
				input.SetSum(25)

				input.BucketCounts().Append(4, 6)
				input.ExplicitBounds().Append(1)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
		{
			name:  "exponential histogram",
			input: getTestExponentialHistogramMetricMultiple(),
			t:     common.Sum,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyExponentialHistogram()
				metricInput.SetName("exponential_histogram_metric")
				metricInput.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

				input := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
				input.SetScale(1)
				input.SetCount(10)
				input.SetSum(25)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := AggregateLabel(tt.t, tt.labelSet)
			require.Nil(t, err)

			_, err = evaluate(nil, ottlmetric.NewTransformContext(tt.input, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			assert.Equal(t, tt.wantErr, err)

			actualMetrics := pmetric.NewMetricSlice()
			tt.input.CopyTo(actualMetrics.AppendEmpty())

			if tt.want != nil {
				expected := pmetric.NewMetricSlice()
				tt.want(expected)
				assert.Equal(t, expected, actualMetrics)
			}
		})
	}
}

func getTestSumMetricMultiple() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	attrs := getAggregateTestAttributes()
	attrs.CopyTo(input.Attributes())

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(50)
	attrs2 := getAggregateTestAttributes()
	attrs2.CopyTo(input2.Attributes())

	return metricInput
}

func getTestSumMetricMultipleOdd() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	attrs := getAggregateTestAttributes()
	attrs.CopyTo(input.Attributes())

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(50)
	attrs2 := getAggregateTestAttributes()
	attrs2.CopyTo(input2.Attributes())

	input3 := metricInput.Sum().DataPoints().AppendEmpty()
	input3.SetDoubleValue(30)
	attrs3 := getAggregateTestAttributes()
	attrs3.CopyTo(input3.Attributes())

	return metricInput
}

func getTestGaugeMetricMultiple() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyGauge()
	metricInput.SetName("gauge_metric")

	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetIntValue(12)
	attrs := getAggregateTestAttributes()
	attrs.CopyTo(input.Attributes())

	input2 := metricInput.Gauge().DataPoints().AppendEmpty()
	input2.SetIntValue(5)
	attrs2 := getAggregateTestAttributes()
	attrs2.CopyTo(input2.Attributes())

	return metricInput
}

func getTestGaugeMetricMultipleOdd() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyGauge()
	metricInput.SetName("gauge_metric")

	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetIntValue(12)
	attrs := getAggregateTestAttributes()
	attrs.CopyTo(input.Attributes())

	input2 := metricInput.Gauge().DataPoints().AppendEmpty()
	input2.SetIntValue(5)
	attrs2 := getAggregateTestAttributes()
	attrs2.CopyTo(input2.Attributes())

	input3 := metricInput.Gauge().DataPoints().AppendEmpty()
	input3.SetIntValue(3)
	attrs3 := getAggregateTestAttributes()
	attrs3.CopyTo(input3.Attributes())

	return metricInput
}

func getTestHistogramMetricMultiple() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyHistogram()
	metricInput.SetName("histogram_metric")
	metricInput.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	input := metricInput.Histogram().DataPoints().AppendEmpty()
	input.SetCount(5)
	input.SetSum(12.34)

	input.BucketCounts().Append(2, 3)
	input.ExplicitBounds().Append(1)

	attrs := getAggregateTestAttributes()
	attrs.CopyTo(input.Attributes())

	input2 := metricInput.Histogram().DataPoints().AppendEmpty()
	input2.SetCount(5)
	input2.SetSum(12.66)

	input2.BucketCounts().Append(2, 3)
	input2.ExplicitBounds().Append(1)

	attrs2 := getAggregateTestAttributes()
	attrs2.CopyTo(input2.Attributes())
	return metricInput
}

func getTestExponentialHistogramMetricMultiple() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyExponentialHistogram()
	metricInput.SetName("exponential_histogram_metric")
	metricInput.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	input := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
	input.SetScale(1)
	input.SetCount(5)
	input.SetSum(12.34)

	attrs := getTestAttributes()
	attrs.CopyTo(input.Attributes())

	input2 := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
	input2.SetScale(1)
	input2.SetCount(5)
	input2.SetSum(12.66)

	attrs2 := getTestAttributes()
	attrs2.CopyTo(input2.Attributes())
	return metricInput
}

func getAggregateTestAttributes() pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("test", "hello world")
	return attrs
}
