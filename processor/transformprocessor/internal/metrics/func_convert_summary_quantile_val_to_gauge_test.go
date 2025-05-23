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
				gaugeMetric.SetName(summaryMetric.Name())
				gaugeMetric.SetUnit(summaryMetric.Unit())
				gauge := gaugeMetric.SetEmptyGauge()

				attrs := getTestAttributes()

				gaugeDp := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp.Attributes())
				gaugeDp.Attributes().PutStr("quantile", "0.99")
				gaugeDp.SetDoubleValue(1)

				gaugeDp1 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp1.Attributes())
				gaugeDp1.Attributes().PutStr("quantile", "0.95")
				gaugeDp1.SetDoubleValue(2)

				gaugeDp2 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp2.Attributes())
				gaugeDp2.Attributes().PutStr("quantile", "0.50")
				gaugeDp2.SetDoubleValue(3)
			},
		},
		{
			name:  "convert_summary_quantile_val_to_gauge custom attribute key",
			input: getTestSummaryMetric(),
			key:   ottl.NewTestingOptional[string]("custom_quantile"),
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())

				gaugeMetric := metrics.AppendEmpty()
				gaugeMetric.SetDescription(summaryMetric.Description())
				gaugeMetric.SetName(summaryMetric.Name())
				gaugeMetric.SetUnit(summaryMetric.Unit())
				gauge := gaugeMetric.SetEmptyGauge()

				attrs := getTestAttributes()

				gaugeDp := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp.Attributes())
				gaugeDp.Attributes().PutStr("custom_quantile", "0.99")
				gaugeDp.SetDoubleValue(1)

				gaugeDp1 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp1.Attributes())
				gaugeDp1.Attributes().PutStr("custom_quantile", "0.95")
				gaugeDp1.SetDoubleValue(2)

				gaugeDp2 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp2.Attributes())
				gaugeDp2.Attributes().PutStr("custom_quantile", "0.50")
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

			evaluate, err := convertSummaryQuantileValToGauge(tt.key)
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
	assert.Equal(t, "0", quantileToStringValue(0))
	assert.Equal(t, "0", quantileToStringValue(0.0))
	assert.Equal(t, "1", quantileToStringValue(1))
	assert.Equal(t, "1", quantileToStringValue(1.0))
	assert.Equal(t, "0.55", quantileToStringValue(0.55))
	assert.Equal(t, "0.99999", quantileToStringValue(0.99999))
	assert.Equal(t, "0.01", quantileToStringValue(0.01))
	assert.Equal(t, "0.00000001", quantileToStringValue(0.00000001))
	assert.Equal(t, "0.50", quantileToStringValue(0.5))
	assert.Equal(t, "0.50", quantileToStringValue(0.50))
}
