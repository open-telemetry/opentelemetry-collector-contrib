package sawmillsfuncs

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

func Test_splitMetricByAttributes(t *testing.T) {
	emptyAttr := ottl.Optional[[]string]{}
	tests := []struct {
		name       string
		input      func() pmetric.Metric
		attributes ottl.Optional[[]string]
		prefix     string
		want       func(pmetric.MetricSlice)
		wantErr    error
	}{
		{
			name:   "sum metric with empty attributes",
			prefix: "prefix",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptySum()
				metricInput.SetName("sum_metric")

				input := metricInput.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				input2 := metricInput.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("key3", "val3")

				input3 := metricInput.Sum().DataPoints().AppendEmpty()
				input3.SetDoubleValue(20)
				input3.Attributes().PutStr("key4", "val4")

				input4 := metricInput.Sum().DataPoints().AppendEmpty()
				input4.SetDoubleValue(30)

				return metricInput
			},
			attributes: emptyAttr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("prefix_sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key3", "val3")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(20)
				input.Attributes().PutStr("key4", "val4")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(30)

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key1", "val1")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key2", "val2")
			},
		},
		{
			name: "sum metric with marching processor attribute",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptySum()
				metricInput.SetName("sum_metric")

				input := metricInput.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				input2 := metricInput.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("key3", "val3")

				input3 := metricInput.Sum().DataPoints().AppendEmpty()
				input3.SetDoubleValue(20)
				input3.Attributes().PutStr("key4", "val4")

				return metricInput
			},
			attributes: ottl.NewTestingOptional[[]string](
				[]string{"processor"},
			),
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("_sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key3", "val3")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(20)
				input.Attributes().PutStr("key4", "val4")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key2", "val2")
			},
		},
		{
			name: "sum metric with marching processor attribute and aggregate values",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptySum()
				metricInput.SetName("sum_metric")

				input := metricInput.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				input2 := metricInput.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("key3", "val3")
				input2.Attributes().PutStr("key4", "val4")

				input3 := metricInput.Sum().DataPoints().AppendEmpty()
				input3.SetDoubleValue(20)
				input3.Attributes().PutStr("key4", "val4")

				return metricInput
			},
			attributes: ottl.NewTestingOptional[[]string](
				[]string{"processor"},
			),
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("_sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key3", "val3")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(70)
				input.Attributes().PutStr("key4", "val4")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key2", "val2")
			},
		},
		{
			name: "sum metric with duplicate processor attributes",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptySum()
				metricInput.SetName("sum_metric")

				input := metricInput.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				input2 := metricInput.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("key3", "val3")

				input3 := metricInput.Sum().DataPoints().AppendEmpty()
				input3.SetDoubleValue(20)
				input3.Attributes().PutStr("key4", "val4")

				return metricInput
			},
			attributes: ottl.NewTestingOptional[[]string](
				[]string{"processor", "processor"},
			),
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("_sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key3", "val3")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(20)
				input.Attributes().PutStr("key4", "val4")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key2", "val2")
			},
		},
		{
			name: "gauge metric with empty attributes",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				input2 := metricInput.Gauge().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("key3", "val3")

				input3 := metricInput.Gauge().DataPoints().AppendEmpty()
				input3.SetDoubleValue(20)
				input3.Attributes().PutStr("key4", "val4")

				return metricInput
			},
			attributes: emptyAttr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptyGauge()
				sumMetric.SetName("_gauge_metric")

				input := sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key3", "val3")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(20)
				input.Attributes().PutStr("key4", "val4")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key1", "val1")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key2", "val2")
			},
		},
		{
			name: "gauge metric with marching processor attribute",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				input2 := metricInput.Gauge().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("key3", "val3")

				input3 := metricInput.Gauge().DataPoints().AppendEmpty()
				input3.SetDoubleValue(20)
				input3.Attributes().PutStr("key4", "val4")

				return metricInput
			},
			attributes: ottl.NewTestingOptional[[]string](
				[]string{"processor"},
			),
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptyGauge()
				sumMetric.SetName("_gauge_metric")

				input := sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key3", "val3")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(20)
				input.Attributes().PutStr("key4", "val4")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key2", "val2")
			},
		},
		{
			name: "gauge metric with marching processor attribute and aggregation by value",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				input2 := metricInput.Gauge().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("key3", "val3")
				input2.Attributes().PutStr("key4", "val4")

				input3 := metricInput.Gauge().DataPoints().AppendEmpty()
				input3.SetDoubleValue(20)
				input3.Attributes().PutStr("key4", "val4")
				input3.Attributes().PutStr("key3", "val31")

				return metricInput
			},
			attributes: ottl.NewTestingOptional[[]string](
				[]string{"processor"},
			),
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptyGauge()
				sumMetric.SetName("_gauge_metric")

				input := sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key3", "val3")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(70)
				input.Attributes().PutStr("key4", "val4")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(20)
				input.Attributes().PutStr("key3", "val31")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key1", "val1")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutStr("key2", "val2")
			},
		},
		{
			name: "gauge metric with two marching attributes",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutDouble("pod", 15154.15)
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				input2 := metricInput.Gauge().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutStr("key3", "val3")

				input3 := metricInput.Gauge().DataPoints().AppendEmpty()
				input3.SetDoubleValue(20)
				input3.Attributes().PutStr("key4", "val4")

				return metricInput
			},
			attributes: ottl.NewTestingOptional[[]string](
				[]string{"processor", "pod"},
			),
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptyGauge()
				sumMetric.SetName("_gauge_metric")

				input := sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key3", "val3")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(20)
				input.Attributes().PutStr("key4", "val4")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutDouble("pod", 15154.15)
				input.Attributes().PutStr("key1", "val1")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutDouble("pod", 15154.15)
				input.Attributes().PutStr("key2", "val2")
			},
		},
		{
			name: "histogram metric with two marching attributes",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptyHistogram()
				metricInput.SetName("histogram_metric")
				metricInput.Histogram().
					SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

				input := metricInput.Histogram().DataPoints().AppendEmpty()
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutDouble("pod", 15154.15)
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")
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
			},
			attributes: ottl.NewTestingOptional[[]string](
				[]string{"processor", "pod"},
			),
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyHistogram()
				metricInput.SetName("_histogram_metric")
				metricInput.Histogram().
					SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

				input1 := metricInput.Histogram().DataPoints().AppendEmpty()
				input1.SetCount(5)
				input1.SetSum(12.66)

				input1.BucketCounts().Append(2, 3)
				input1.ExplicitBounds().Append(1)

				input2 := metricInput.Histogram().DataPoints().AppendEmpty()
				input2.Attributes().PutStr("processor", "processorval")
				input2.Attributes().PutDouble("pod", 15154.15)
				input2.Attributes().PutStr("key1", "val1")
				input2.SetCount(5)
				input2.SetSum(12.34)

				input2.BucketCounts().Append(2, 3)
				input2.ExplicitBounds().Append(1)

				input3 := metricInput.Histogram().DataPoints().AppendEmpty()
				input3.Attributes().PutStr("processor", "processorval")
				input3.Attributes().PutDouble("pod", 15154.15)
				input3.Attributes().PutStr("key2", "val2")
				input3.SetCount(5)
				input3.SetSum(12.34)

				input3.BucketCounts().Append(2, 3)
				input3.ExplicitBounds().Append(1)
			},
		},
		{
			name: "exponential histogram metric with two marching attributes",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptyExponentialHistogram()
				metricInput.SetName("exponential_histogram_metric")
				metricInput.ExponentialHistogram().
					SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

				input := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutDouble("pod", 15154.15)
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")
				input.SetScale(1)
				input.SetCount(5)
				input.SetSum(12.34)

				input2 := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
				input2.SetScale(1)
				input2.SetCount(5)
				input2.SetSum(12.66)

				return metricInput
			},
			attributes: ottl.NewTestingOptional[[]string](
				[]string{"processor", "pod"},
			),
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyExponentialHistogram()
				metricInput.SetName("_exponential_histogram_metric")
				metricInput.ExponentialHistogram().
					SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

				input1 := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
				input1.SetScale(1)
				input1.SetCount(5)
				input1.SetSum(12.66)

				input2 := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
				input2.Attributes().PutStr("processor", "processorval")
				input2.Attributes().PutDouble("pod", 15154.15)
				input2.Attributes().PutStr("key1", "val1")
				input2.SetScale(1)
				input2.SetCount(5)
				input2.SetSum(12.34)

				input3 := metricInput.ExponentialHistogram().DataPoints().AppendEmpty()
				input3.Attributes().PutStr("processor", "processorval")
				input3.Attributes().PutDouble("pod", 15154.15)
				input3.Attributes().PutStr("key2", "val2")
				input3.SetScale(1)
				input3.SetCount(5)
				input3.SetSum(12.34)
			},
		},
		{
			name: "summary metric with two marching attributes",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetName("summary_metric")
				metricInput.SetEmptySummary()
				datapoints := metricInput.Summary().DataPoints()
				input := datapoints.AppendEmpty()
				input.SetSum(5)
				input.SetCount(5)
				input.Attributes().PutStr("processor", "processorval")
				input.Attributes().PutDouble("pod", 15154.15)
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				input2 := datapoints.AppendEmpty()
				input2.SetCount(5)
				input2.SetSum(12.66)

				return metricInput
			},
			attributes: ottl.NewTestingOptional[[]string](
				[]string{"processor", "pod"},
			),
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetName("_summary_metric")
				metricInput.SetEmptySummary()
				datapoints := metricInput.Summary().DataPoints()

				input1 := datapoints.AppendEmpty()
				input1.SetCount(5)
				input1.SetSum(12.66)

				input2 := datapoints.AppendEmpty()
				input2.SetSum(5)
				input2.SetCount(5)
				input2.Attributes().PutStr("processor", "processorval")
				input2.Attributes().PutDouble("pod", 15154.15)
				input2.Attributes().PutStr("key1", "val1")

				input3 := datapoints.AppendEmpty()
				input3.SetSum(5)
				input3.SetCount(5)
				input3.Attributes().PutStr("processor", "processorval")
				input3.Attributes().PutDouble("pod", 15154.15)
				input3.Attributes().PutStr("key2", "val2")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := splitMetricByAttributes[ottlmetric.TransformContext](
				tt.prefix,
				tt.attributes,
			)
			require.NoError(t, err)

			ms := pmetric.NewMetricSlice()
			input := tt.input()
			input.CopyTo(ms.AppendEmpty())

			expected := pmetric.NewMetricSlice()
			ms.CopyTo(expected)
			tt.want(expected)

			_, err = evaluate(
				nil,
				ottlmetric.NewTransformContext(
					input,
					ms,
					pcommon.NewInstrumentationScope(),
					pcommon.NewResource(),
					pmetric.NewScopeMetrics(),
					pmetric.NewResourceMetrics(),
				),
			)
			assert.Equal(t, tt.wantErr, err)

			inputMetrics := pmetric.NewMetrics()
			expectedMetrics := pmetric.NewMetrics()

			ms.CopyTo(
				inputMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics(),
			)

			expected.CopyTo(
				expectedMetrics.ResourceMetrics().
					AppendEmpty().
					ScopeMetrics().
					AppendEmpty().
					Metrics(),
			)

			assert.NoError(
				t,
				pmetrictest.CompareMetrics(
					expectedMetrics,
					inputMetrics,
					pmetrictest.IgnoreMetricDataPointsOrder(),
				),
			)
		})
	}
}
