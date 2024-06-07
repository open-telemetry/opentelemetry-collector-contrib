// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_aggregateLabelValues(t *testing.T) {
	tests := []struct {
		name     string
		input    pmetric.Metric
		t        common.AggregationType
		label    string
		valueSet map[string]bool
		newValue string
		want     func(pmetric.MetricSlice)
	}{
		{
			name:  "sum sum",
			input: getTestSumMetricMultipleAggregateLabels(),
			t:     common.Sum,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(150)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "sum mean",
			input: getTestSumMetricMultipleAggregateLabels(),
			t:     common.Mean,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(75)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "sum max",
			input: getTestSumMetricMultipleAggregateLabels(),
			t:     common.Max,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "sum min",
			input: getTestSumMetricMultipleAggregateLabels(),
			t:     common.Min,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "gauge sum",
			input: getTestGaugeMetricMultipleAggregateLabels(),
			t:     common.Sum,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(17)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "gauge mean",
			input: getTestGaugeMetricMultipleAggregateLabels(),
			t:     common.Mean,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(8)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "gauge min",
			input: getTestGaugeMetricMultipleAggregateLabels(),
			t:     common.Min,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(5)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "gauge max",
			input: getTestGaugeMetricMultipleAggregateLabels(),
			t:     common.Max,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(12)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "histogram",
			input: getTestHistogramMetricMultipleAggregateLabels(),
			t:     common.Sum,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
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
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "exponential histogram",
			input: getTestExponentialHistogramMetricMultipleAggregateLabels(),
			t:     common.Sum,
			valueSet: map[string]bool{
				"test1": true,
				"test2": true,
			},
			label:    "test",
			newValue: "test_new",
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyExponentialHistogram()
				metricInput.SetName("exponential_histogram_metric")
				metricInput.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

				input := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
				input.SetScale(1)
				input.SetCount(10)
				input.SetSum(25)
				input.Attributes().PutStr("test", "test_new")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := AggregateLabelValue(tt.t, tt.label, tt.valueSet, tt.newValue)
			assert.NoError(t, err)

			_, err = evaluate(nil, ottlmetric.NewTransformContext(tt.input, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			require.Nil(t, err)

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

func getTestSumMetricMultipleAggregateLabels() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	input.Attributes().PutStr("test", "test1")

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(50)
	input2.Attributes().PutStr("test", "test2")

	return metricInput
}

func getTestGaugeMetricMultipleAggregateLabels() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyGauge()
	metricInput.SetName("gauge_metric")

	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetIntValue(12)
	input.Attributes().PutStr("test", "test1")

	input2 := metricInput.Gauge().DataPoints().AppendEmpty()
	input2.SetIntValue(5)
	input2.Attributes().PutStr("test", "test2")

	return metricInput
}

func getTestHistogramMetricMultipleAggregateLabels() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyHistogram()
	metricInput.SetName("histogram_metric")
	metricInput.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	input := metricInput.Histogram().DataPoints().AppendEmpty()
	input.SetCount(5)
	input.SetSum(12.34)

	input.BucketCounts().Append(2, 3)
	input.ExplicitBounds().Append(1)
	input.Attributes().PutStr("test", "test1")

	input2 := metricInput.Histogram().DataPoints().AppendEmpty()
	input2.SetCount(5)
	input2.SetSum(12.66)

	input2.BucketCounts().Append(2, 3)
	input2.ExplicitBounds().Append(1)
	input2.Attributes().PutStr("test", "test2")
	return metricInput
}

func getTestExponentialHistogramMetricMultipleAggregateLabels() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyExponentialHistogram()
	metricInput.SetName("exponential_histogram_metric")
	metricInput.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	input := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
	input.SetScale(1)
	input.SetCount(5)
	input.SetSum(12.34)
	input.Attributes().PutStr("test", "test1")

	input2 := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
	input2.SetScale(1)
	input2.SetCount(5)
	input2.SetSum(12.66)
	input2.Attributes().PutStr("test", "test2")
	return metricInput
}
