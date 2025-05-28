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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_splitOnAttribute(t *testing.T) {
	tests := []struct {
		name        string
		input       pmetric.Metric
		metricName  string
		byAttribute string
		want        func(pmetric.MetricSlice)
		wantErr     error
	}{
		{
			name:        "single attribute",
			input:       getTestSumMetricSingleAttribute(),
			metricName:  "sum_metric",
			byAttribute: "state",
			want: func(metrics pmetric.MetricSlice) {
				metric1 := metrics.AppendEmpty()
				metric1.SetEmptySum()
				metric1.SetName("sum_metric.val1")
				input1 := metric1.Sum().DataPoints().AppendEmpty()
				input1.SetDoubleValue(100)

				metric2 := metrics.AppendEmpty()
				metric2.SetEmptySum()
				metric2.SetName("sum_metric.val2")
				input2 := metric2.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(20)

				metric3 := metrics.AppendEmpty()
				metric3.SetEmptySum()
				metric3.SetName("sum_metric.val3")
				input3 := metric3.Sum().DataPoints().AppendEmpty()
				input3.SetDoubleValue(50)
			},
		},
		{
			name:        "multiple data points with the same attribute name",
			input:       getTestSumMetricDuplicateAttribute(),
			metricName:  "sum_metric",
			byAttribute: "state",
			want: func(metrics pmetric.MetricSlice) {
				metric1 := metrics.AppendEmpty()
				metric1.SetEmptySum()
				metric1.SetName("sum_metric.val1")
				input1 := metric1.Sum().DataPoints().AppendEmpty()
				input1.SetDoubleValue(100)
				input2 := metric1.Sum().DataPoints().AppendEmpty()
				input2.SetDoubleValue(20)

				metric2 := metrics.AppendEmpty()
				metric2.SetEmptySum()
				metric2.SetName("sum_metric.val2")
				input3 := metric2.Sum().DataPoints().AppendEmpty()
				input3.SetDoubleValue(50)
			},
		},
		{
			name:        "many attributes",
			input:       getTestSumMetricManyAttributes(),
			metricName:  "gauge_metric",
			byAttribute: "direction",
			want: func(metrics pmetric.MetricSlice) {
				metric1 := metrics.AppendEmpty()
				metric1.SetEmptyGauge()
				metric1.SetName("gauge_metric.read")
				input1 := metric1.Gauge().DataPoints().AppendEmpty()
				input1.SetDoubleValue(100)
				input1.Attributes().PutStr("state", "val1")

				metric2 := metrics.AppendEmpty()
				metric2.SetEmptyGauge()
				metric2.SetName("gauge_metric.write")
				input2 := metric2.Gauge().DataPoints().AppendEmpty()
				input2.SetDoubleValue(20)
				input2.Attributes().PutStr("state", "val2")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := SplitOnAttribute(tt.byAttribute)
			require.NoError(t, err)

			metrics := pmetric.NewMetricSlice()
			tt.input.CopyTo(metrics.AppendEmpty())
			tCtx := ottlmetric.NewTransformContext(metrics.At(0), metrics, pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())
			_, err = evaluate(nil, tCtx)
			assert.Equal(t, tt.wantErr, err)

			if tt.want != nil {
				expected := pmetric.NewMetricSlice()
				tt.want(expected)

				expectedMetrics := pmetric.NewMetrics()
				sl := expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				expected.CopyTo(sl)

				actualMetrics := pmetric.NewMetrics()
				sl2 := actualMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				tCtx.GetMetrics().CopyTo(sl2)

				require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricDataPointsOrder()))
			}
		})
	}
}

func getTestSumMetricSingleAttribute() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	input.Attributes().PutStr("state", "val1")

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(20)
	input2.Attributes().PutStr("state", "val2")

	input3 := metricInput.Sum().DataPoints().AppendEmpty()
	input3.SetDoubleValue(50)
	input3.Attributes().PutStr("state", "val3")

	return metricInput
}

func getTestSumMetricDuplicateAttribute() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	input.Attributes().PutStr("state", "val1")

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(20)
	input2.Attributes().PutStr("state", "val1")

	input3 := metricInput.Sum().DataPoints().AppendEmpty()
	input3.SetDoubleValue(50)
	input3.Attributes().PutStr("state", "val2")

	return metricInput
}

func getTestSumMetricManyAttributes() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyGauge()
	metricInput.SetName("gauge_metric")

	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	input.Attributes().PutStr("state", "val1")
	input.Attributes().PutStr("direction", "read")

	input2 := metricInput.Gauge().DataPoints().AppendEmpty()
	input2.SetDoubleValue(20)
	input2.Attributes().PutStr("state", "val2")
	input2.Attributes().PutStr("direction", "write")
	return metricInput
}
