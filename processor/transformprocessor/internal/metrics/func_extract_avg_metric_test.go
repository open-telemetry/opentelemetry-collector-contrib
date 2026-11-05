// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

func getTestSumMetric() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")
	metricInput.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	metricInput.Sum().SetIsMonotonic(true)
	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(12.34)

	attrs := getTestAttributes()
	attrs.CopyTo(input.Attributes())
	return metricInput
}

func getTestHistogramMetricEmpty() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyHistogram()
	metricInput.SetName("histogram_metric_empty")
	metricInput.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	return metricInput
}

func getTestHistogramMetricAllSkipped() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyHistogram()
	metricInput.SetName("histogram_metric_all_skipped")
	metricInput.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dps := metricInput.Histogram().DataPoints()

	countZero := dps.AppendEmpty()
	countZero.SetCount(0)
	countZero.SetSum(5)

	noSum := dps.AppendEmpty()
	noSum.SetCount(3)

	return metricInput
}

func getTestHistogramMetricMixedDataPoints() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyHistogram()
	metricInput.SetName("histogram_metric_mixed")
	metricInput.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dps := metricInput.Histogram().DataPoints()

	valid1 := dps.AppendEmpty()
	valid1.SetCount(5)
	valid1.SetSum(10)
	valid1.Attributes().PutStr("dp", "valid1")

	countZero := dps.AppendEmpty()
	countZero.SetCount(0)
	countZero.SetSum(99)
	countZero.Attributes().PutStr("dp", "count-zero")

	noSum := dps.AppendEmpty()
	noSum.SetCount(7)
	noSum.Attributes().PutStr("dp", "no-sum")

	valid2 := dps.AppendEmpty()
	valid2.SetCount(4)
	valid2.SetSum(8)
	valid2.Attributes().PutStr("dp", "valid2")

	return metricInput
}

func getTestExponentialHistogramMetricMultiDataPoints() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyExponentialHistogram()
	metricInput.SetName("exponential_histogram_metric_multi")
	metricInput.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dps := metricInput.ExponentialHistogram().DataPoints()

	dp0 := dps.AppendEmpty()
	dp0.SetCount(10)
	dp0.SetSum(100)
	dp0.Attributes().PutStr("dp", "a")

	dp1 := dps.AppendEmpty()
	dp1.SetCount(2)
	dp1.SetSum(7)
	dp1.Attributes().PutStr("dp", "b")

	dp2 := dps.AppendEmpty()
	dp2.SetCount(3)
	dp2.SetSum(1)
	dp2.Attributes().PutStr("dp", "c")

	return metricInput
}

func Test_extractAvgMetric(t *testing.T) {
	tests := []histogramTestCase{
		{
			name:  "histogram",
			input: getTestHistogramMetric(),
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptyGauge()

				avgMetric.SetName(histogramMetric.Name() + "_avg")
				dp := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(histogramMetric.Histogram().DataPoints().At(0).Sum() / float64(histogramMetric.Histogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name: "histogram (count zero)",
			input: func() pmetric.Metric {
				metric := getTestHistogramMetric()
				metric.Histogram().DataPoints().At(0).SetCount(0)
				return metric
			}(),
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.Histogram().DataPoints().At(0).SetCount(0)
				histogramMetric.CopyTo(metrics.AppendEmpty())
			},
		},
		{
			name: "histogram (no sum)",
			input: func() pmetric.Metric {
				metric := getTestHistogramMetric()
				metric.Histogram().DataPoints().At(0).RemoveSum()
				return metric
			}(),
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.Histogram().DataPoints().At(0).RemoveSum()
				histogramMetric.CopyTo(metrics.AppendEmpty())
			},
		},
		{
			name: "histogram (non-zero timestamps)",
			input: func() pmetric.Metric {
				metric := getTestHistogramMetric()
				dp := metric.Histogram().DataPoints().At(0)
				dp.SetStartTimestamp(pcommon.Timestamp(1000))
				dp.SetTimestamp(pcommon.Timestamp(2000))
				return metric
			}(),
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				inputDp := histogramMetric.Histogram().DataPoints().At(0)
				inputDp.SetStartTimestamp(pcommon.Timestamp(1000))
				inputDp.SetTimestamp(pcommon.Timestamp(2000))
				histogramMetric.CopyTo(metrics.AppendEmpty())

				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptyGauge()
				avgMetric.SetName(histogramMetric.Name() + "_avg")
				dp := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(inputDp.Sum() / float64(inputDp.Count()))
				dp.SetStartTimestamp(pcommon.Timestamp(1000))
				dp.SetTimestamp(pcommon.Timestamp(2000))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:  "histogram (multiple data points, mixed valid and skipped)",
			input: getTestHistogramMetricMixedDataPoints(),
			want: func(metrics pmetric.MetricSlice) {
				mixedMetric := getTestHistogramMetricMixedDataPoints()
				mixedMetric.CopyTo(metrics.AppendEmpty())

				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptyGauge()
				avgMetric.SetName(mixedMetric.Name() + "_avg")

				dp0 := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp0.SetDoubleValue(10.0 / 5.0)
				dp0.Attributes().PutStr("dp", "valid1")

				dp1 := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp1.SetDoubleValue(8.0 / 4.0)
				dp1.Attributes().PutStr("dp", "valid2")
			},
		},
		{
			name:  "histogram (all data points skipped)",
			input: getTestHistogramMetricAllSkipped(),
			want: func(metrics pmetric.MetricSlice) {
				getTestHistogramMetricAllSkipped().CopyTo(metrics.AppendEmpty())
			},
		},
		{
			name:  "histogram (zero data points)",
			input: getTestHistogramMetricEmpty(),
			want: func(metrics pmetric.MetricSlice) {
				getTestHistogramMetricEmpty().CopyTo(metrics.AppendEmpty())
			},
		},
		{
			name:  "exponential histogram",
			input: getTestExponentialHistogramMetric(),
			want: func(metrics pmetric.MetricSlice) {
				expHistogramMetric := getTestExponentialHistogramMetric()
				expHistogramMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptyGauge()

				avgMetric.SetName(expHistogramMetric.Name() + "_avg")
				dp := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Sum() / float64(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:  "exponential histogram (multiple data points, different sum/count)",
			input: getTestExponentialHistogramMetricMultiDataPoints(),
			want: func(metrics pmetric.MetricSlice) {
				multiMetric := getTestExponentialHistogramMetricMultiDataPoints()
				multiMetric.CopyTo(metrics.AppendEmpty())

				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptyGauge()
				avgMetric.SetName(multiMetric.Name() + "_avg")

				dp0 := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp0.SetDoubleValue(100.0 / 10.0)
				dp0.Attributes().PutStr("dp", "a")

				dp1 := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp1.SetDoubleValue(7.0 / 2.0)
				dp1.Attributes().PutStr("dp", "b")

				dp2 := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp2.SetDoubleValue(1.0 / 3.0)
				dp2.Attributes().PutStr("dp", "c")
			},
		},
		{
			name:  "summary",
			input: getTestSummaryMetric(),
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptyGauge()

				avgMetric.SetName("summary_metric_avg")
				dp := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(summaryMetric.Summary().DataPoints().At(0).Sum() / float64(summaryMetric.Summary().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:   "summary custom suffix",
			input:  getTestSummaryMetric(),
			suffix: ottl.NewTestingOptional("_custom_suf"),
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptyGauge()

				avgMetric.SetName("summary_metric_custom_suf")
				dp := avgMetric.Gauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(summaryMetric.Summary().DataPoints().At(0).Sum() / float64(summaryMetric.Summary().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:    "gauge (error)",
			input:   getTestGaugeMetric(),
			wantErr: errors.New("extract_avg_metric requires an input metric of type Histogram, ExponentialHistogram or Summary, got Gauge"),
		},
		{
			name:    "sum (error)",
			input:   getTestSumMetric(),
			wantErr: errors.New("extract_avg_metric requires an input metric of type Histogram, ExponentialHistogram or Summary, got Sum"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sMetrics := pmetric.NewScopeMetrics()
			tt.input.CopyTo(sMetrics.Metrics().AppendEmpty())

			evaluate, err := extractAvgMetric(tt.suffix)
			assert.NoError(t, err)

			tCtx := ottlmetric.NewTransformContext(pmetric.NewResourceMetrics(), sMetrics, tt.input)
			defer tCtx.Close()
			_, err = evaluate(t.Context(), tCtx)
			assert.Equal(t, tt.wantErr, err)

			if tt.want != nil {
				expected := pmetric.NewMetricSlice()
				tt.want(expected)
				assert.Equal(t, expected, sMetrics.Metrics())
			}
		})
	}
}

func BenchmarkExtractAvgMetric(b *testing.B) {
	template := getTestHistogramMetric()
	metric := pmetric.NewMetric()
	resourceMetrics := pmetric.NewResourceMetrics()
	scopeMetrics := pmetric.NewScopeMetrics()
	transformContext := ottlmetric.NewTransformContext(resourceMetrics, scopeMetrics, metric)
	b.Cleanup(transformContext.Close)

	expr, err := extractAvgMetric(ottl.Optional[string]{})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for b.Loop() {
		template.CopyTo(metric)
		scopeMetrics.Metrics().RemoveIf(func(pmetric.Metric) bool { return true })
		_, err = expr(b.Context(), transformContext)
		if err != nil {
			b.Fatal(err)
		}
	}
}
