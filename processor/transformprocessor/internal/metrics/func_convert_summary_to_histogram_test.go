// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_convertSummaryToHistogram(t *testing.T) {
	multiDpSummary := func() pmetric.Metric {
		m := getTestSummaryMetric()
		m.SetDescription("desc")
		m.SetUnit("ms")
		dp := m.Summary().DataPoints().AppendEmpty()
		dp.SetCount(7)
		dp.SetSum(3.5)
		dp.SetStartTimestamp(pcommon.Timestamp(10))
		dp.SetTimestamp(pcommon.Timestamp(20))
		dp.Attributes().PutStr("k", "v")
		return m
	}

	tests := []struct {
		name          string
		stringAggTemp string
		suffix        ottl.Optional[string]
		input         pmetric.Metric
		want          func(pmetric.MetricSlice)
	}{
		{
			name:          "summary to cumulative histogram",
			stringAggTemp: "cumulative",
			input:         getTestSummaryMetric(),
			want: func(metrics pmetric.MetricSlice) {
				getTestSummaryMetric().CopyTo(metrics.AppendEmpty())
				m := metrics.AppendEmpty()
				m.SetName("summary_metric_histogram")
				h := m.SetEmptyHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := h.DataPoints().AppendEmpty()
				dp.SetCount(100)
				dp.SetSum(12.34)
				dp.BucketCounts().FromRaw([]uint64{100})
				getTestAttributes().CopyTo(dp.Attributes())
			},
		},
		{
			name:          "summary to delta histogram",
			stringAggTemp: "delta",
			input:         getTestSummaryMetric(),
			want: func(metrics pmetric.MetricSlice) {
				getTestSummaryMetric().CopyTo(metrics.AppendEmpty())
				m := metrics.AppendEmpty()
				m.SetName("summary_metric_histogram")
				h := m.SetEmptyHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				dp := h.DataPoints().AppendEmpty()
				dp.SetCount(100)
				dp.SetSum(12.34)
				dp.BucketCounts().FromRaw([]uint64{100})
				getTestAttributes().CopyTo(dp.Attributes())
			},
		},
		{
			name:          "custom suffix",
			stringAggTemp: "cumulative",
			suffix:        ottl.NewTestingOptional("_custom_suf"),
			input:         getTestSummaryMetric(),
			want: func(metrics pmetric.MetricSlice) {
				getTestSummaryMetric().CopyTo(metrics.AppendEmpty())
				m := metrics.AppendEmpty()
				m.SetName("summary_metric_custom_suf")
				h := m.SetEmptyHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := h.DataPoints().AppendEmpty()
				dp.SetCount(100)
				dp.SetSum(12.34)
				dp.BucketCounts().FromRaw([]uint64{100})
				getTestAttributes().CopyTo(dp.Attributes())
			},
		},
		{
			name:          "empty suffix uses the default",
			stringAggTemp: "cumulative",
			suffix:        ottl.NewTestingOptional(""),
			input:         getTestSummaryMetric(),
			want: func(metrics pmetric.MetricSlice) {
				getTestSummaryMetric().CopyTo(metrics.AppendEmpty())
				m := metrics.AppendEmpty()
				m.SetName("summary_metric_histogram")
				h := m.SetEmptyHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := h.DataPoints().AppendEmpty()
				dp.SetCount(100)
				dp.SetSum(12.34)
				dp.BucketCounts().FromRaw([]uint64{100})
				getTestAttributes().CopyTo(dp.Attributes())
			},
		},
		{
			name:          "multiple data points keep metadata, timestamps and attributes",
			stringAggTemp: "cumulative",
			input:         multiDpSummary(),
			want: func(metrics pmetric.MetricSlice) {
				multiDpSummary().CopyTo(metrics.AppendEmpty())
				m := metrics.AppendEmpty()
				m.SetName("summary_metric_histogram")
				m.SetDescription("desc")
				m.SetUnit("ms")
				h := m.SetEmptyHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp1 := h.DataPoints().AppendEmpty()
				dp1.SetCount(100)
				dp1.SetSum(12.34)
				dp1.BucketCounts().FromRaw([]uint64{100})
				getTestAttributes().CopyTo(dp1.Attributes())
				dp2 := h.DataPoints().AppendEmpty()
				dp2.SetCount(7)
				dp2.SetSum(3.5)
				dp2.SetStartTimestamp(pcommon.Timestamp(10))
				dp2.SetTimestamp(pcommon.Timestamp(20))
				dp2.BucketCounts().FromRaw([]uint64{7})
				dp2.Attributes().PutStr("k", "v")
			},
		},
		{
			name:          "summary with no data points",
			stringAggTemp: "cumulative",
			input: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("summary_metric")
				m.SetEmptySummary()
				return m
			}(),
			want: func(metrics pmetric.MetricSlice) {
				m := pmetric.NewMetric()
				m.SetName("summary_metric")
				m.SetEmptySummary()
				m.CopyTo(metrics.AppendEmpty())
				h := metrics.AppendEmpty()
				h.SetName("summary_metric_histogram")
				h.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			},
		},
		{
			name:          "no recorded value flag is preserved",
			stringAggTemp: "cumulative",
			input: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("summary_metric")
				dp := m.SetEmptySummary().DataPoints().AppendEmpty()
				dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				return m
			}(),
			want: func(metrics pmetric.MetricSlice) {
				m := pmetric.NewMetric()
				m.SetName("summary_metric")
				m.SetEmptySummary().DataPoints().AppendEmpty().SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				m.CopyTo(metrics.AppendEmpty())
				h := metrics.AppendEmpty()
				h.SetName("summary_metric_histogram")
				hist := h.SetEmptyHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := hist.DataPoints().AppendEmpty()
				dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				dp.SetSum(0)
				dp.BucketCounts().FromRaw([]uint64{0})
			},
		},
		{
			name:          "non summary is untouched",
			stringAggTemp: "cumulative",
			input:         getTestGaugeMetric(),
			want: func(metrics pmetric.MetricSlice) {
				getTestGaugeMetric().CopyTo(metrics.AppendEmpty())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sMetrics := pmetric.NewScopeMetrics()
			tt.input.CopyTo(sMetrics.Metrics().AppendEmpty())

			evaluate, err := convertSummaryToHistogram(tt.stringAggTemp, tt.suffix)
			require.NoError(t, err)

			tCtx := ottlmetric.NewTransformContext(pmetric.NewResourceMetrics(), sMetrics, sMetrics.Metrics().At(0))
			defer tCtx.Close()
			_, err = evaluate(t.Context(), tCtx)
			require.NoError(t, err)

			expected := pmetric.NewMetricSlice()
			tt.want(expected)

			expectedMetrics := pmetric.NewMetrics()
			expected.CopyTo(expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())

			actualMetrics := pmetric.NewMetrics()
			sMetrics.MoveTo(actualMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty())

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics))
		})
	}
}

func Test_convertSummaryToHistogram_validation(t *testing.T) {
	_, err := convertSummaryToHistogram("not a real aggregation temporality", ottl.Optional[string]{})
	assert.EqualError(t, err, "unknown aggregation temporality: not a real aggregation temporality")
}

func BenchmarkConvertSummaryToHistogram(b *testing.B) {
	template := pmetric.NewScopeMetrics()
	getTestSummaryMetric().CopyTo(template.Metrics().AppendEmpty())

	sMetrics := pmetric.NewScopeMetrics()
	template.CopyTo(sMetrics)

	exprFunc, err := convertSummaryToHistogram("cumulative", ottl.Optional[string]{})
	if err != nil {
		b.Fatal(err)
	}

	transformContext := ottlmetric.NewTransformContext(pmetric.NewResourceMetrics(), sMetrics, sMetrics.Metrics().At(0))
	b.Cleanup(transformContext.Close)
	b.ReportAllocs()
	for b.Loop() {
		template.CopyTo(sMetrics)
		if _, err = exprFunc(b.Context(), transformContext); err != nil {
			b.Fatal(err)
		}
	}
}
