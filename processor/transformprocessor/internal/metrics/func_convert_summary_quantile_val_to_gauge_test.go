// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
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
				gaugeMetric.SetName(summaryMetric.Name() + ".quantiles")
				gaugeMetric.SetUnit(summaryMetric.Unit())
				gauge := gaugeMetric.SetEmptyGauge()

				attrs := getTestAttributes()

				gaugeDp := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp.Attributes())
				gaugeDp.Attributes().PutDouble("quantile", 0.99)
				gaugeDp.SetDoubleValue(1)

				gaugeDp1 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp1.Attributes())
				gaugeDp1.Attributes().PutDouble("quantile", 0.95)
				gaugeDp1.SetDoubleValue(2)

				gaugeDp2 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp2.Attributes())
				gaugeDp2.Attributes().PutDouble("quantile", 0.50)
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
				gaugeMetric.SetName(summaryMetric.Name() + ".quantiles")
				gaugeMetric.SetUnit(summaryMetric.Unit())
				gauge := gaugeMetric.SetEmptyGauge()

				attrs := getTestAttributes()

				gaugeDp := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp.Attributes())
				gaugeDp.Attributes().PutDouble("custom_quantile", 0.99)
				gaugeDp.SetDoubleValue(1)

				gaugeDp1 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp1.Attributes())
				gaugeDp1.Attributes().PutDouble("custom_quantile", 0.95)
				gaugeDp1.SetDoubleValue(2)

				gaugeDp2 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp2.Attributes())
				gaugeDp2.Attributes().PutDouble("custom_quantile", 0.50)
				gaugeDp2.SetDoubleValue(3)
			},
		},
		{
			name:   "convert_summary_quantile_val_to_gauge custom attribute key and suffix",
			input:  getTestSummaryMetric(),
			key:    ottl.NewTestingOptional[string]("custom_quantile"),
			suffix: ottl.NewTestingOptional[string](".custom_suffix"),
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())

				gaugeMetric := metrics.AppendEmpty()
				gaugeMetric.SetDescription(summaryMetric.Description())
				gaugeMetric.SetName(summaryMetric.Name() + ".custom_suffix")
				gaugeMetric.SetUnit(summaryMetric.Unit())
				gauge := gaugeMetric.SetEmptyGauge()

				attrs := getTestAttributes()

				gaugeDp := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp.Attributes())
				gaugeDp.Attributes().PutDouble("custom_quantile", 0.99)
				gaugeDp.SetDoubleValue(1)

				gaugeDp1 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp1.Attributes())
				gaugeDp1.Attributes().PutDouble("custom_quantile", 0.95)
				gaugeDp1.SetDoubleValue(2)

				gaugeDp2 := gauge.DataPoints().AppendEmpty()
				attrs.CopyTo(gaugeDp2.Attributes())
				gaugeDp2.Attributes().PutDouble("custom_quantile", 0.50)
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

			evaluate, err := convertSummaryQuantileValToGauge(tt.key, tt.suffix)
			require.NoError(t, err)

			_, err = evaluate(nil, ottlmetric.NewTransformContext(tt.input, actualMetric, pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics()))
			require.NoError(t, err)

			expected := pmetric.NewMetricSlice()
			tt.want(expected)

			expectedMetrics := pmetric.NewMetrics()
			sl := expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			expected.CopyTo(sl)

			actualMetrics := pmetric.NewMetrics()
			sl2 := actualMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			actualMetric.CopyTo(sl2)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics))
		})
	}
}
