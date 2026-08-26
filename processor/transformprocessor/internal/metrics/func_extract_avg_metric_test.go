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

func Test_extractAvgMetric(t *testing.T) {
	tests := []histogramTestCase{
		{
			name:         "histogram (non-monotonic)",
			input:        getTestHistogramMetric(),
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptySum()
				avgMetric.Sum().SetAggregationTemporality(histogramMetric.Histogram().AggregationTemporality())
				avgMetric.Sum().SetIsMonotonic(false)

				avgMetric.SetName(histogramMetric.Name() + "_avg")
				dp := avgMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(histogramMetric.Histogram().DataPoints().At(0).Sum() / float64(histogramMetric.Histogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "histogram (monotonic)",
			input:        getTestHistogramMetric(),
			monotonicity: true,
			want: func(metrics pmetric.MetricSlice) {
				histogramMetric := getTestHistogramMetric()
				histogramMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptySum()
				avgMetric.Sum().SetAggregationTemporality(histogramMetric.Histogram().AggregationTemporality())
				avgMetric.Sum().SetIsMonotonic(true)

				avgMetric.SetName(histogramMetric.Name() + "_avg")
				dp := avgMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(histogramMetric.Histogram().DataPoints().At(0).Sum() / float64(histogramMetric.Histogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "exponential histogram (non-monotonic)",
			input:        getTestExponentialHistogramMetric(),
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				expHistogramMetric := getTestExponentialHistogramMetric()
				expHistogramMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptySum()
				avgMetric.Sum().SetAggregationTemporality(expHistogramMetric.ExponentialHistogram().AggregationTemporality())
				avgMetric.Sum().SetIsMonotonic(false)

				avgMetric.SetName(expHistogramMetric.Name() + "_avg")
				dp := avgMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Sum() / float64(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "exponential histogram (monotonic)",
			input:        getTestExponentialHistogramMetric(),
			monotonicity: true,
			want: func(metrics pmetric.MetricSlice) {
				expHistogramMetric := getTestExponentialHistogramMetric()
				expHistogramMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptySum()
				avgMetric.Sum().SetAggregationTemporality(expHistogramMetric.ExponentialHistogram().AggregationTemporality())
				avgMetric.Sum().SetIsMonotonic(true)

				avgMetric.SetName(expHistogramMetric.Name() + "_avg")
				dp := avgMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Sum() / float64(expHistogramMetric.ExponentialHistogram().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "summary (non-monotonic)",
			input:        getTestSummaryMetric(),
			monotonicity: false,
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptySum()
				avgMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				avgMetric.Sum().SetIsMonotonic(false)

				avgMetric.SetName("summary_metric_avg")
				dp := avgMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(summaryMetric.Summary().DataPoints().At(0).Sum() / float64(summaryMetric.Summary().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "summary (monotonic)",
			input:        getTestSummaryMetric(),
			monotonicity: true,
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptySum()
				avgMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				avgMetric.Sum().SetIsMonotonic(true)

				avgMetric.SetName("summary_metric_avg")
				dp := avgMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(summaryMetric.Summary().DataPoints().At(0).Sum() / float64(summaryMetric.Summary().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "summary custom suffix",
			input:        getTestSummaryMetric(),
			monotonicity: true,
			suffix:       ottl.NewTestingOptional("_custom_suf"),
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				avgMetric := metrics.AppendEmpty()
				avgMetric.SetEmptySum()
				avgMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				avgMetric.Sum().SetIsMonotonic(true)

				avgMetric.SetName("summary_metric_custom_suf")
				dp := avgMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(summaryMetric.Summary().DataPoints().At(0).Sum() / float64(summaryMetric.Summary().DataPoints().At(0).Count()))

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:         "gauge (error)",
			input:        getTestGaugeMetric(),
			monotonicity: false,
			wantErr:      errors.New("extract_avg_metric requires an input metric of type Histogram, ExponentialHistogram or Summary, got Gauge"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualMetrics := pmetric.NewMetricSlice()
			tt.input.CopyTo(actualMetrics.AppendEmpty())

			evaluate, err := extractAvgMetric(tt.monotonicity, tt.suffix)
			assert.NoError(t, err)

			_, err = evaluate(nil, ottlmetric.NewTransformContext(tt.input, actualMetrics, pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics()))
			assert.Equal(t, tt.wantErr, err)

			if tt.want != nil {
				expected := pmetric.NewMetricSlice()
				tt.want(expected)
				assert.Equal(t, expected, actualMetrics)
			}
		})
	}
}
