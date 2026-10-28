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
		input         pmetric.Metric
		want          func(pmetric.Metric)
	}{
		{
			name:          "summary to cumulative histogram",
			stringAggTemp: "cumulative",
			input:         getTestSummaryMetric(),
			want: func(m pmetric.Metric) {
				m.SetName("summary_metric")
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
			want: func(m pmetric.Metric) {
				m.SetName("summary_metric")
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
			name:          "multiple data points keep metadata, timestamps and attributes",
			stringAggTemp: "cumulative",
			input:         multiDpSummary(),
			want: func(m pmetric.Metric) {
				m.SetName("summary_metric")
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
				m.SetEmptySummary()
				return m
			}(),
			want: func(m pmetric.Metric) {
				m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
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
			want: func(m pmetric.Metric) {
				m.SetName("summary_metric")
				h := m.SetEmptyHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := h.DataPoints().AppendEmpty()
				dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				dp.SetSum(0)
				dp.BucketCounts().FromRaw([]uint64{0})
			},
		},
		{
			name:          "non summary is untouched",
			stringAggTemp: "cumulative",
			input:         getTestGaugeMetric(),
			want:          func(m pmetric.Metric) { getTestGaugeMetric().CopyTo(m) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input.CopyTo(metric)

			tCtx := ottlmetric.NewTransformContext(pmetric.NewResourceMetrics(), pmetric.NewScopeMetrics(), metric)
			defer tCtx.Close()

			evaluate, err := convertSummaryToHistogram(tt.stringAggTemp)
			require.NoError(t, err)

			_, err = evaluate(t.Context(), tCtx)
			require.NoError(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)
			assert.Equal(t, expected, metric)
		})
	}
}

func Test_convertSummaryToHistogram_validation(t *testing.T) {
	_, err := convertSummaryToHistogram("not a real aggregation temporality")
	assert.EqualError(t, err, "unknown aggregation temporality: not a real aggregation temporality")
}

func BenchmarkConvertSummaryToHistogram(b *testing.B) {
	template := getTestSummaryMetric()

	exprFunc, err := convertSummaryToHistogram("cumulative")
	if err != nil {
		b.Fatal(err)
	}

	metric := pmetric.NewMetric()
	transformContext := ottlmetric.NewTransformContext(pmetric.NewResourceMetrics(), pmetric.NewScopeMetrics(), metric)
	b.Cleanup(transformContext.Close)
	b.ReportAllocs()
	for b.Loop() {
		template.CopyTo(metric)
		if _, err = exprFunc(b.Context(), transformContext); err != nil {
			b.Fatal(err)
		}
	}
}
