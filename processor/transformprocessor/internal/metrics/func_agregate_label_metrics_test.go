// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_aggregateLabels(t *testing.T) {
	tests := []struct {
		name     string
		input    pmetric.Metric
		t        common.AggregationType
		labelSet map[string]bool
		want     func(pmetric.MetricSlice)
		wantErr  error
	}{
		{
			name:  "sum sum",
			input: getTestSumMetricMultiple(),
			t:     common.Sum,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(150)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
			wantErr: nil,
		},
		{
			name:  "sum max",
			input: getTestSumMetricMultiple(),
			t:     common.Max,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(100)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
			wantErr: nil,
		},
		{
			name:  "sum min",
			input: getTestSumMetricMultiple(),
			t:     common.Min,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(50)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
			wantErr: nil,
		},
		{
			name:  "sum mean",
			input: getTestSumMetricMultiple(),
			t:     common.Mean,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetEmptySum()
				sumMetric.SetName("sum_metric")
				input := sumMetric.Sum().DataPoints().AppendEmpty()
				input.SetDoubleValue(75)

				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
			wantErr: nil,
		},
		{
			name:  "gauge sum",
			input: getTestGaugeMetricMultiple(),
			t:     common.Sum,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(17)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
			wantErr: nil,
		},
		{
			name:  "gauge min",
			input: getTestGaugeMetricMultiple(),
			t:     common.Min,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(5)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
			wantErr: nil,
		},
		{
			name:  "gauge Max",
			input: getTestGaugeMetricMultiple(),
			t:     common.Max,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(12)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
			wantErr: nil,
		},
		{
			name:  "gauge mean",
			input: getTestGaugeMetricMultiple(),
			t:     common.Mean,
			labelSet: map[string]bool{
				"test": true,
			},
			want: func(metrics pmetric.MetricSlice) {
				metricInput := metrics.AppendEmpty()
				metricInput.SetEmptyGauge()
				metricInput.SetName("gauge_metric")

				input := metricInput.Gauge().DataPoints().AppendEmpty()
				input.SetIntValue(8)
				attrs := getAggregateTestAttributes()
				attrs.CopyTo(input.Attributes())
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := AggregateLabel(tt.t, tt.labelSet)
			assert.NoError(t, err)

			_, err = evaluate(nil, ottlmetric.NewTransformContext(tt.input, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			assert.Equal(t, tt.wantErr, err)

			actualMetrics := pmetric.NewMetricSlice()
			tt.input.CopyTo(actualMetrics.AppendEmpty())

			if tt.want != nil {
				expected := pmetric.NewMetricSlice()
				tt.want(expected)
				assert.Equal(t, expected, actualMetrics)
			}
		})
	}
}

func getTestSumMetricMultiple() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptySum()
	metricInput.SetName("sum_metric")

	input := metricInput.Sum().DataPoints().AppendEmpty()
	input.SetDoubleValue(100)
	attrs := getAggregateTestAttributes()
	attrs.CopyTo(input.Attributes())

	input2 := metricInput.Sum().DataPoints().AppendEmpty()
	input2.SetDoubleValue(50)
	attrs2 := getAggregateTestAttributes()
	attrs2.CopyTo(input2.Attributes())

	return metricInput
}

func getTestGaugeMetricMultiple() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetEmptyGauge()
	metricInput.SetName("gauge_metric")

	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetIntValue(12)
	attrs := getAggregateTestAttributes()
	attrs.CopyTo(input.Attributes())

	input2 := metricInput.Gauge().DataPoints().AppendEmpty()
	input2.SetIntValue(5)
	attrs2 := getAggregateTestAttributes()
	attrs2.CopyTo(input2.Attributes())

	return metricInput
}

func getAggregateTestAttributes() pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("test", "hello world")
	return attrs
}
