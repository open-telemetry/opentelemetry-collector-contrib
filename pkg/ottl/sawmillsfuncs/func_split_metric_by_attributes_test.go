// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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
			name:   "splits untracked metric that already starts with prefix",
			prefix: "prefix",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptySum()
				metricInput.SetName("prefix_sum_metric")

				input := metricInput.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key1", "val1")

				return metricInput
			},
			attributes: emptyAttr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("prefix_prefix_sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key1", "val1")
			},
		},
		{
			name:   "splits source metric that already starts with prefix",
			prefix: "prefix",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptySum()
				metricInput.SetName("prefix_sum_metric")

				input := metricInput.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key1", "val1")
				input.Attributes().PutStr("key2", "val2")

				return metricInput
			},
			attributes: emptyAttr,
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("prefix_prefix_sum_metric")

				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key1", "val1")

				input = sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutStr("key2", "val2")
			},
		},
		{
			name: "gauge metric keeps unsupported double attribute values distinct",
			input: func() pmetric.Metric {
				metricInput := pmetric.NewMetric()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutDouble("bad", math.Inf(1))

				input2 := metricInput.Gauge().DataPoints().AppendEmpty()
				input2.SetDoubleValue(50)
				input2.Attributes().PutDouble("bad", math.Inf(-1))

				return metricInput
			},
			attributes: emptyAttr,
			want: func(metrics pmetric.MetricSlice) {
				gaugeMetric := metrics.AppendEmpty()
				gaugeMetric.SetEmptyGauge()
				gaugeMetric.SetName("_gauge_metric")

				input := gaugeMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)
				input.Attributes().PutDouble("bad", math.Inf(1))

				input = gaugeMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutDouble("bad", math.Inf(-1))
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
			attributes: ottl.NewTestingOptional[[]string]([]string{"processor"}),
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
			attributes: ottl.NewTestingOptional[[]string]([]string{"processor"}),
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
			attributes: ottl.NewTestingOptional[[]string]([]string{"processor", "processor"}),
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
			attributes: ottl.NewTestingOptional[[]string]([]string{"processor"}),
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
			attributes: ottl.NewTestingOptional[[]string]([]string{"processor"}),
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptyGauge()
				sumMetric.SetName("_gauge_metric")

				input := sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key3", "val3")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)
				input.Attributes().PutStr("key4", "val4")

				input = sumMetric.Gauge().DataPoints().AppendEmpty()
				input.SetDoubleValue(20)
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
			attributes: ottl.NewTestingOptional[[]string]([]string{"processor", "pod"}),
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
				metricInput.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

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
			attributes: ottl.NewTestingOptional[[]string]([]string{"processor", "pod"}),
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyHistogram()
				metricInput.SetName("_histogram_metric")
				metricInput.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

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
				metricInput.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

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
			attributes: ottl.NewTestingOptional[[]string]([]string{"processor", "pod"}),
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyExponentialHistogram()
				metricInput.SetName("_exponential_histogram_metric")
				metricInput.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

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
			attributes: ottl.NewTestingOptional[[]string]([]string{"processor", "pod"}),
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
			evaluate := splitMetricByAttributes[*ottlmetric.TransformContext](tt.prefix, tt.attributes)

			ms := pmetric.NewMetricSlice()
			input := tt.input()
			input.CopyTo(ms.AppendEmpty())

			expected := pmetric.NewMetricSlice()
			ms.CopyTo(expected)
			tt.want(expected)

			rm := pmetric.NewResourceMetrics()
			sm := rm.ScopeMetrics().AppendEmpty()
			ms.CopyTo(sm.Metrics())
			ctx := ottlmetric.NewTransformContextPtr(rm, sm, sm.Metrics().At(0))
			defer ctx.Close()

			_, err := evaluate(nil, ctx)
			assert.Equal(t, tt.wantErr, err)

			inputMetrics := pmetric.NewMetrics()
			expectedMetrics := pmetric.NewMetrics()

			sm.Metrics().CopyTo(inputMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())
			expected.CopyTo(expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())

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

func Test_splitMetricByAttributesSkipsTrackedGeneratedMetric(t *testing.T) {
	evaluate := splitMetricByAttributes[*ottlmetric.TransformContext](
		"prefix",
		ottl.Optional[[]string]{},
	)

	rm := pmetric.NewResourceMetrics()
	sm := rm.ScopeMetrics().AppendEmpty()
	metrics := sm.Metrics()
	iteration := ottlmetric.NewMetricIteration()

	metricInput := metrics.AppendEmpty()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	input.Attributes().PutStr("key1", "val1")
	input.Attributes().PutStr("key2", "val2")

	ctx := ottlmetric.NewTransformContextPtr(rm, sm, metrics.At(0), ottlmetric.WithMetricIteration(iteration, 0))
	_, err := evaluate(t.Context(), ctx)
	ctx.Close()

	require.NoError(t, err)
	require.Equal(t, 2, metrics.Len())
	require.Equal(t, "prefix_sum_metric", metrics.At(1).Name())

	ctx = ottlmetric.NewTransformContextPtr(rm, sm, metrics.At(1), ottlmetric.WithMetricIteration(iteration, 1))
	_, err = evaluate(t.Context(), ctx)
	ctx.Close()

	require.NoError(t, err)
	require.Equal(t, 2, metrics.Len())
}

func Test_splitMetricByAttributesDoesNotRetainFallbackStateWithoutIteration(t *testing.T) {
	evaluate := splitMetricByAttributes[*ottlmetric.TransformContext](
		"prefix",
		ottl.Optional[[]string]{},
	)

	rm := pmetric.NewResourceMetrics()
	sm := rm.ScopeMetrics().AppendEmpty()
	metrics := sm.Metrics()

	metricInput := metrics.AppendEmpty()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	input.Attributes().PutStr("key1", "val1")
	input.Attributes().PutStr("key2", "val2")

	ctx := ottlmetric.NewTransformContextPtr(rm, sm, metrics.At(0))
	_, err := evaluate(t.Context(), ctx)
	ctx.Close()

	require.NoError(t, err)
	require.Equal(t, 2, metrics.Len())
	require.Equal(t, "prefix_sum_metric", metrics.At(1).Name())

	secondRM := pmetric.NewResourceMetrics()
	secondSM := secondRM.ScopeMetrics().AppendEmpty()
	secondMetrics := secondSM.Metrics()

	secondMetrics.AppendEmpty().SetName("unrelated_metric")
	secondMetricInput := secondMetrics.AppendEmpty()
	secondMetricInput.SetEmptySum()
	secondMetricInput.SetName("second_sum_metric")

	secondInput := secondMetricInput.Sum().DataPoints().AppendEmpty()
	secondInput.SetDoubleValue(100)
	secondInput.Attributes().PutStr("key1", "val1")
	secondInput.Attributes().PutStr("key2", "val2")

	ctx = ottlmetric.NewTransformContextPtr(secondRM, secondSM, secondMetrics.At(1))
	_, err = evaluate(t.Context(), ctx)
	ctx.Close()

	require.NoError(t, err)
	require.Equal(t, 3, secondMetrics.Len())
	require.Equal(t, "prefix_second_sum_metric", secondMetrics.At(2).Name())
}

func Test_splitMetricByAttributesDoesNotSkipSameNameSourceBeforeGeneratedOutput(t *testing.T) {
	evaluate := splitMetricByAttributes[*ottlmetric.TransformContext](
		"prefix",
		ottl.Optional[[]string]{},
	)

	rm := pmetric.NewResourceMetrics()
	sm := rm.ScopeMetrics().AppendEmpty()
	metrics := sm.Metrics()
	iteration := ottlmetric.NewMetricIteration()

	metricInput := metrics.AppendEmpty()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")
	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	input.Attributes().PutStr("key1", "val1")
	input.Attributes().PutStr("key2", "val2")

	sameNameSource := metrics.AppendEmpty()
	sameNameSource.SetEmptySum()
	sameNameSource.SetName("prefix_sum_metric")
	prefixedInput := sameNameSource.Sum().DataPoints().AppendEmpty()
	prefixedInput.SetDoubleValue(7)
	prefixedInput.Attributes().PutStr("key3", "val3")
	prefixedInput.Attributes().PutStr("key4", "val4")

	ctx := ottlmetric.NewTransformContextPtr(rm, sm, metrics.At(0), ottlmetric.WithMetricIteration(iteration, 0))
	_, err := evaluate(t.Context(), ctx)
	ctx.Close()

	require.NoError(t, err)
	require.Equal(t, 3, metrics.Len())
	require.Equal(t, "prefix_sum_metric", metrics.At(2).Name())

	ctx = ottlmetric.NewTransformContextPtr(rm, sm, metrics.At(1), ottlmetric.WithMetricIteration(iteration, 1))
	_, err = evaluate(t.Context(), ctx)
	ctx.Close()

	require.NoError(t, err)
	require.Equal(t, 4, metrics.Len())
	require.Equal(t, "prefix_prefix_sum_metric", metrics.At(3).Name())

	ctx = ottlmetric.NewTransformContextPtr(rm, sm, metrics.At(2), ottlmetric.WithMetricIteration(iteration, 2))
	_, err = evaluate(t.Context(), ctx)
	ctx.Close()

	require.NoError(t, err)
	require.Equal(t, 4, metrics.Len())
}

func Test_groupNumberDataPointsDoesNotMutateInputSlice(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()
	dp := dps.AppendEmpty()
	dp.SetDoubleValue(100)
	dp.Attributes().PutStr("key", "val")

	duplicate := dps.AppendEmpty()
	duplicate.SetDoubleValue(50)
	duplicate.Attributes().PutStr("key", "val")

	grouped := groupNumberDataPoints(dps, false)

	require.Equal(t, 1, grouped.Len())
	require.Equal(t, 150.0, grouped.At(0).DoubleValue())
	require.Equal(t, 100.0, dps.At(0).DoubleValue())
	require.Equal(t, 50.0, dps.At(1).DoubleValue())
}

func Test_groupNumberDataPointsPreservesFirstOccurrenceOrder(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()
	first := dps.AppendEmpty()
	first.SetIntValue(1)
	first.Attributes().PutStr("key", "first")

	second := dps.AppendEmpty()
	second.SetIntValue(2)
	second.Attributes().PutStr("key", "second")

	duplicateFirst := dps.AppendEmpty()
	duplicateFirst.SetIntValue(3)
	duplicateFirst.Attributes().PutStr("key", "first")

	grouped := groupNumberDataPoints(dps, false)

	require.Equal(t, 2, grouped.Len())
	value, ok := grouped.At(0).Attributes().Get("key")
	require.True(t, ok)
	require.Equal(t, "first", value.Str())
	require.Equal(t, int64(4), grouped.At(0).IntValue())
	value, ok = grouped.At(1).Attributes().Get("key")
	require.True(t, ok)
	require.Equal(t, "second", value.Str())
	require.Equal(t, int64(2), grouped.At(1).IntValue())
}

func Test_generatedMetricTrackerKeepsInterleavedIterationsIndependent(t *testing.T) {
	tracker := newGeneratedMetricTracker()

	firstIteration := ottlmetric.NewMetricIteration()
	tracker.mark(firstIteration, 1)

	secondIteration := ottlmetric.NewMetricIteration()
	tracker.mark(secondIteration, 1)
	require.Equal(t, 2, tracker.pendingLen())

	require.True(t, tracker.consume(firstIteration, 1))
	require.True(t, tracker.consume(secondIteration, 1))
	require.Equal(t, 0, tracker.pendingLen())
}

func Test_generatedMetricTrackerClearsIterationOnClose(t *testing.T) {
	tracker := newGeneratedMetricTracker()
	iteration := ottlmetric.NewMetricIteration()

	tracker.mark(iteration, 1)
	tracker.mark(iteration, 2)
	require.Equal(t, 2, tracker.pendingLen())

	iteration.Close()
	require.Equal(t, 0, tracker.pendingLen())
}

func Test_generatedMetricTrackerMarkClosedIterationDoesNotDeadlock(t *testing.T) {
	tracker := newGeneratedMetricTracker()
	iteration := ottlmetric.NewMetricIteration()
	iteration.Close()

	done := make(chan struct{})
	go func() {
		tracker.mark(iteration, 1)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "mark deadlocked registering cleanup for a closed iteration")
	}
	require.Equal(t, 0, tracker.pendingLen())
}
