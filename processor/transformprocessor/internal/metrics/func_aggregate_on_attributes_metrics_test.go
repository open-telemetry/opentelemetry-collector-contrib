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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_aggregateOnAttributes(t *testing.T) {
	attr := ottl.Optional[[]string]{}
	tests := []struct {
		name       string
		input      pmetric.Metric
		t          common.AggregationType
		attributes ottl.Optional[[]string]
		want       func(pmetric.MetricSlice)
		wantErr    error
	}{
		{
			name:       "sum sum",
			input:      getTestSummaryMetric(),
			t:          common.Sum,
			attributes: attr,
			want:       nil,
			wantErr:    fmt.Errorf("aggregation function is not supported for Summary metrics"),
		},
		{
			name:       "sum sum",
			input:      getTestSumMetricMultiple(),
			t:          common.Sum,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(150)
			},
		},
		{
			name:       "sum max",
			input:      getTestSumMetricMultiple(),
			t:          common.Max,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
			},
		},
		{
			name:       "sum min",
			input:      getTestSumMetricMultiple(),
			t:          common.Min,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
			},
		},
		{
			name:       "sum mean",
			input:      getTestSumMetricMultiple(),
			t:          common.Mean,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(75)
			},
		},
		{
			name:       "sum median even",
			input:      getTestSumMetricMultiple(),
			t:          common.Median,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(75)
			},
		},
		{
			name:       "sum median odd",
			input:      getTestSumMetricMultipleOdd(),
			t:          common.Median,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
			},
		},
		{
			name:       "gauge sum",
			input:      getTestGaugeMetricMultiple(),
			t:          common.Sum,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(17)
			},
		},
		{
			name:       "gauge min",
			input:      getTestGaugeMetricMultiple(),
			t:          common.Min,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(5)
			},
		},
		{
			name:       "gauge max",
			input:      getTestGaugeMetricMultiple(),
			t:          common.Max,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(12)
			},
		},
		{
			name:       "gauge mean",
			input:      getTestGaugeMetricMultiple(),
			t:          common.Mean,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(8)
			},
		},
		{
			name:       "gauge median even",
			input:      getTestGaugeMetricMultiple(),
			t:          common.Median,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(8)
			},
		},
		{
			name:       "gauge median odd",
			input:      getTestGaugeMetricMultipleOdd(),
			t:          common.Median,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(5)
			},
		},
		{
			name:       "histogram",
			input:      getTestHistogramMetricMultiple(),
			t:          common.Sum,
			attributes: attr,
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
			},
		},
		{
			name:       "exponential histogram",
			input:      getTestExponentialHistogramMetricMultiple(),
			t:          common.Sum,
			attributes: attr,
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyExponentialHistogram()
				metricInput.SetName("exponential_histogram_metric")
				metricInput.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

				input := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
				input.SetScale(1)
				input.SetCount(10)
				input.SetSum(25)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := AggregateOnAttributes(tt.t, tt.attributes)
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

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(50)

	return metricInput
}

func getTestSumMetricMultipleOdd() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(50)

	input3 := metricInput.Sum().DataPoints().AppendEmpty()
	input3.SetDoubleValue(30)

	return metricInput
}

func getTestGaugeMetricMultiple() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyGauge()
	metricInput.SetName("gauge_metric")

	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetIntValue(12)

	input2 := metricInput.Gauge().DataPoints().AppendEmpty()
	input2.SetIntValue(5)

	return metricInput
}

func getTestGaugeMetricMultipleOdd() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyGauge()
	metricInput.SetName("gauge_metric")

	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetIntValue(12)

	input2 := metricInput.Gauge().DataPoints().AppendEmpty()
	input2.SetIntValue(5)

	input3 := metricInput.Gauge().DataPoints().AppendEmpty()
	input3.SetIntValue(3)

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

	input2 := metricInput.Histogram().DataPoints().AppendEmpty()
	input2.SetCount(5)
	input2.SetSum(12.66)

	input2.BucketCounts().Append(2, 3)
	input2.ExplicitBounds().Append(1)
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

	input2 := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
	input2.SetScale(1)
	input2.SetCount(5)
	input2.SetSum(12.66)

	return metricInput
}
