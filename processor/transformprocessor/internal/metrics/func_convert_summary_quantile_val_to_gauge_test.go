// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_ConvertSummaryQuantileValToGauge(t *testing.T) {
	tests := []summaryTestCase{
		{
			name:  "convert_summary_quantile_val_to_gauge",
			input: getTestSummaryMetric(),
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())

				gaugeMetric := metrics.AppendEmpty()
				gaugeMetric.SetDescription(summaryMetric.Description())
				gaugeMetric.SetName(summaryMetric.Name() + ".quantile_99")
				gaugeMetric.SetUnit(summaryMetric.Unit())
				gaugeDp := gaugeMetric.SetEmptyGauge().DataPoints().AppendEmpty()
				attrs := getTestAttributes()
				attrs.CopyTo(gaugeDp.Attributes())
				gaugeDp.SetDoubleValue(1)

				gaugeMetric1 := metrics.AppendEmpty()
				gaugeMetric1.SetDescription(summaryMetric.Description())
				gaugeMetric1.SetName(summaryMetric.Name() + ".quantile_95")
				gaugeMetric1.SetUnit(summaryMetric.Unit())
				gaugeDp1 := gaugeMetric1.SetEmptyGauge().DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp1.Attributes())
				gaugeDp1.SetDoubleValue(2)

				gaugeMetric2 := metrics.AppendEmpty()
				gaugeMetric2.SetDescription(summaryMetric.Description())
				gaugeMetric2.SetName(summaryMetric.Name() + ".quantile_50")
				gaugeMetric2.SetUnit(summaryMetric.Unit())
				gaugeDp2 := gaugeMetric2.SetEmptyGauge().DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp2.Attributes())
				gaugeDp2.SetDoubleValue(3)
			},
		},
		{
			name:   "convert_summary_quantile_val_to_gauge custom suffix",
			input:  getTestSummaryMetric(),
			suffix: ottl.NewTestingOptional[string](".custom_quantile"),
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())

				gaugeMetric := metrics.AppendEmpty()
				gaugeMetric.SetDescription(summaryMetric.Description())
				gaugeMetric.SetName(summaryMetric.Name() + ".custom_quantile_99")
				gaugeMetric.SetUnit(summaryMetric.Unit())
				gaugeDp := gaugeMetric.SetEmptyGauge().DataPoints().AppendEmpty()
				attrs := getTestAttributes()
				attrs.CopyTo(gaugeDp.Attributes())
				gaugeDp.SetDoubleValue(1)

				gaugeMetric1 := metrics.AppendEmpty()
				gaugeMetric1.SetDescription(summaryMetric.Description())
				gaugeMetric1.SetName(summaryMetric.Name() + ".custom_quantile_95")
				gaugeMetric1.SetUnit(summaryMetric.Unit())
				gaugeDp1 := gaugeMetric1.SetEmptyGauge().DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp1.Attributes())
				gaugeDp1.SetDoubleValue(2)

				gaugeMetric2 := metrics.AppendEmpty()
				gaugeMetric2.SetDescription(summaryMetric.Description())
				gaugeMetric2.SetName(summaryMetric.Name() + ".custom_quantile_50")
				gaugeMetric2.SetUnit(summaryMetric.Unit())
				gaugeDp2 := gaugeMetric2.SetEmptyGauge().DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp2.Attributes())
				gaugeDp2.SetDoubleValue(3)
			},
		},
		{
			name:  "convert_summary_quantile_val_to_gauge (no op)",
			input: getTestGaugeMetric(),
			want: func(metrics pmetric.MetricSlice) {
				gaugeMetric := getTestGaugeMetric()
				gaugeMetric.CopyTo(metrics.AppendEmpty())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualMetric := pmetric.NewMetricSlice()
			tt.input.CopyTo(actualMetric.AppendEmpty())

			evaluate, err := convertSummaryQuantileValToGauge(tt.suffix)
			assert.NoError(t, err)

			_, err = evaluate(nil, ottldatapoint.NewTransformContext(pmetric.NewNumberDataPoint(), tt.input, actualMetric, pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics()))
			assert.NoError(t, err)

			expected := pmetric.NewMetricSlice()
			tt.want(expected)

			expectedMetrics := pmetric.NewMetrics()
			sl := expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			expected.CopyTo(sl)

			actualMetrics := pmetric.NewMetrics()
			sl2 := actualMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			actualMetric.CopyTo(sl2)

			assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics))
		})
	}
}

func TestQuantileToStringSuffix(t *testing.T) {
	assert.Equal(t, "0", quantileToStringSuffix(0))
	assert.Equal(t, "0", quantileToStringSuffix(0.0))
	assert.Equal(t, "1", quantileToStringSuffix(1))
	assert.Equal(t, "1", quantileToStringSuffix(1.0))
	assert.Equal(t, "55", quantileToStringSuffix(0.55))
	assert.Equal(t, "99999", quantileToStringSuffix(0.99999))
	assert.Equal(t, "01", quantileToStringSuffix(0.01))
	assert.Equal(t, "50", quantileToStringSuffix(0.5))
}
