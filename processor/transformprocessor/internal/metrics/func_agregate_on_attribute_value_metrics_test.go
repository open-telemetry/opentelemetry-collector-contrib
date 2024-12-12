// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_aggregateOnAttributeValues(t *testing.T) {
	tests := []struct {
		name      string
		input     pmetric.Metric
		t         aggregateutil.AggregationType
		attribute string
		values    []string
		newValue  string
		want      func(pmetric.MetricSlice)
	}{
		{
			name:  "non-existing value",
			input: getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Sum,
			values: []string{
				"test44",
			},
			attribute: "test",
			newValue:  "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("test", "test1")

				input2 := sumMetric.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("test", "test2")
			},
		},
		{
			name:      "empty values",
			input:     getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:         aggregateutil.Sum,
			values:    []string{},
			attribute: "test",
			newValue:  "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("test", "test1")

				input2 := sumMetric.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("test", "test2")
			},
		},
		{
			name:  "non-existing attribute",
			input: getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Sum,
			values: []string{
				"test1",
			},
			attribute: "testyy",
			newValue:  "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("test", "test1")

				input2 := sumMetric.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("test", "test2")
			},
		},
		{
			name:  "non-matching attribute",
			input: getTestSumMetricMultipleAggregateOnAttributeValueAdditionalAttribute(),
			t:     aggregateutil.Sum,
			values: []string{
				"test1",
			},
			attribute: "testyy",
			newValue:  "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("test", "test1")

				input2 := sumMetric.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("test", "test2")
				input2.Attributes().PutStr("test3", "test3")
			},
		},
		{
			name:  "duplicated values",
			input: getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Sum,
			values: []string{
				"test1",
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			name:  "2 datapoints aggregated, one left unaggregated",
			input: getTestSumMetricMultipleAggregateOnAttributeValueOdd(),
			t:     aggregateutil.Sum,
			values: []string{
				"test1",
			},
			attribute: "test",
			newValue:  "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(150)
				input.Attributes().PutStr("test", "test_new")

				input3 := sumMetric.Sum().DataPoints().AppendEmpty()
				input3.SetDoubleValue(30)
				input3.Attributes().PutStr("test", "test2")
			},
		},
		{
			name:  "sum sum",
			input: getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Sum,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			input: getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Mean,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			input: getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Max,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			input: getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Min,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			name:  "sum count",
			input: getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Count,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(2)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "sum median even",
			input: getTestSumMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Median,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			name:  "sum median odd",
			input: getTestSumMetricMultipleAggregateOnAttributeValueOdd(),
			t:     aggregateutil.Median,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			input: getTestGaugeMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Sum,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			input: getTestGaugeMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Mean,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			name:  "gauge count",
			input: getTestGaugeMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Count,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(2)
				input.Attributes().PutStr("test", "test_new")
			},
		},
		{
			name:  "gauge median even",
			input: getTestGaugeMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Median,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			name:  "gauge median odd",
			input: getTestGaugeMetricMultipleAggregateOnAttributeValueOdd(),
			t:     aggregateutil.Median,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			name:  "gauge min",
			input: getTestGaugeMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Min,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			input: getTestGaugeMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Max,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			input: getTestHistogramMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Sum,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			input: getTestExponentialHistogramMetricMultipleAggregateOnAttributeValue(),
			t:     aggregateutil.Sum,
			values: []string{
				"test1",
				"test2",
			},
			attribute: "test",
			newValue:  "test_new",
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
			evaluate, err := AggregateOnAttributeValue(tt.t, tt.attribute, tt.values, tt.newValue)
			assert.NoError(t, err)

			_, err = evaluate(nil, ottlmetric.NewTransformContext(tt.input, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics()))
			require.NoError(t, err)

			actualMetric := pmetric.NewMetricSlice()
			tt.input.CopyTo(actualMetric.AppendEmpty())

			if tt.want != nil {
				expected := pmetric.NewMetricSlice()
				tt.want(expected)

				expectedMetrics := pmetric.NewMetrics()
				sl := expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				expected.CopyTo(sl)

				actualMetrics := pmetric.NewMetrics()
				sl2 := actualMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				actualMetric.CopyTo(sl2)

				require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricDataPointsOrder()))
			}
		})
	}
}

func Test_createAggregateOnAttributeValueFunction(t *testing.T) {
	// invalid input arguments
	_, err := createAggregateOnAttributeValueFunction(ottl.FunctionContext{}, nil)
	require.ErrorContains(t, err, "AggregateOnAttributeValueFactory args must be of type *AggregateOnAttributeValueArguments")

	// invalid aggregation function
	_, err = createAggregateOnAttributeValueFunction(ottl.FunctionContext{}, &aggregateOnAttributeValueArguments{
		AggregationFunction: "invalid",
		Attribute:           "attr",
		Values:              []string{"val"},
		NewValue:            "newVal",
	})
	require.ErrorContains(t, err, "invalid aggregation function")
}

func getTestSumMetricMultipleAggregateOnAttributeValue() pmetric.Metric {
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

func getTestSumMetricMultipleAggregateOnAttributeValueAdditionalAttribute() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	input.Attributes().PutStr("test", "test1")

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(50)
	input2.Attributes().PutStr("test", "test2")
	input2.Attributes().PutStr("test3", "test3")

	return metricInput
}

func getTestSumMetricMultipleAggregateOnAttributeValueOdd() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	input.Attributes().PutStr("test", "test1")

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(50)
	input2.Attributes().PutStr("test", "test1")

	input3 := metricInput.Sum().DataPoints().AppendEmpty()
	input3.SetDoubleValue(30)
	input3.Attributes().PutStr("test", "test2")

	return metricInput
}

func getTestGaugeMetricMultipleAggregateOnAttributeValue() pmetric.Metric {
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

func getTestGaugeMetricMultipleAggregateOnAttributeValueOdd() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyGauge()
	metricInput.SetName("gauge_metric")

	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetIntValue(12)
	input.Attributes().PutStr("test", "test1")

	input2 := metricInput.Gauge().DataPoints().AppendEmpty()
	input2.SetIntValue(5)
	input2.Attributes().PutStr("test", "test2")

	input3 := metricInput.Gauge().DataPoints().AppendEmpty()
	input3.SetIntValue(3)
	input3.Attributes().PutStr("test", "test1")

	return metricInput
}

func getTestHistogramMetricMultipleAggregateOnAttributeValue() pmetric.Metric {
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

func getTestExponentialHistogramMetricMultipleAggregateOnAttributeValue() pmetric.Metric {
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
