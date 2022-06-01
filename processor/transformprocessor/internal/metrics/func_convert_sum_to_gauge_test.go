package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Test_convert_sum_to_gauge(t *testing.T) {
	sumInput := pmetric.NewMetric()
	sumInput.SetDataType(pmetric.MetricDataTypeSum)

	dp1 := sumInput.Sum().DataPoints().AppendEmpty()
	dp1.SetIntVal(10)

	dp2 := sumInput.Sum().DataPoints().AppendEmpty()
	dp2.SetDoubleVal(14.5)

	gaugeInput := pmetric.NewMetric()
	gaugeInput.SetDataType(pmetric.MetricDataTypeGauge)

	histogramInput := pmetric.NewMetric()
	histogramInput.SetDataType(pmetric.MetricDataTypeHistogram)

	expoHistogramInput := pmetric.NewMetric()
	expoHistogramInput.SetDataType(pmetric.MetricDataTypeExponentialHistogram)

	summaryInput := pmetric.NewMetric()
	summaryInput.SetDataType(pmetric.MetricDataTypeSummary)

	tests := []struct {
		name  string
		input pmetric.Metric
		want  func(pmetric.Metric)
	}{
		{
			name:  "convert sum to gauge",
			input: sumInput,
			want: func(metric pmetric.Metric) {
				sumInput.CopyTo(metric)

				dps := sumInput.Sum().DataPoints()
				metric.SetDataType(pmetric.MetricDataTypeGauge)
				dps.CopyTo(metric.Gauge().DataPoints())
			},
		},
		{
			name:  "noop for gauge",
			input: gaugeInput,
			want: func(metric pmetric.Metric) {
				gaugeInput.CopyTo(metric)
			},
		},
		{
			name:  "noop for histogram",
			input: histogramInput,
			want: func(metric pmetric.Metric) {
				histogramInput.CopyTo(metric)
			},
		},
		{
			name:  "noop for exponential histogram",
			input: expoHistogramInput,
			want: func(metric pmetric.Metric) {
				expoHistogramInput.CopyTo(metric)
			},
		},
		{
			name:  "noop for summary",
			input: summaryInput,
			want: func(metric pmetric.Metric) {
				summaryInput.CopyTo(metric)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input.CopyTo(metric)

			ctx := metricTransformContext{
				metric:   metric,
				il:       pcommon.NewInstrumentationScope(),
				resource: pcommon.NewResource(),
			}

			exprFunc, _ := convertSumToGauge()
			exprFunc(ctx)

			expected := pmetric.NewMetric()
			tt.want(expected)

			assert.Equal(t, expected, metric)
		})
	}
}
