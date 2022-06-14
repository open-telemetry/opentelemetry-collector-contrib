package metrics

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func getTestSummaryMetric() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetDataType(pmetric.MetricDataTypeSummary)
	metricInput.SetName("summary_metric")
	input := metricInput.Summary().DataPoints().AppendEmpty()
	input.SetCount(100)
	input.SetSum(12.34)

	qVal1 := input.QuantileValues().AppendEmpty()
	qVal1.SetValue(1)
	qVal1.SetQuantile(.99)

	qVal2 := input.QuantileValues().AppendEmpty()
	qVal2.SetValue(2)
	qVal2.SetQuantile(.95)

	qVal3 := input.QuantileValues().AppendEmpty()
	qVal3.SetValue(3)
	qVal3.SetQuantile(.50)

	attrs := getTestAttributes()
	attrs.CopyTo(input.Attributes())
	return metricInput
}

func getTestAttributes() pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.InsertString("test", "hello world")
	attrs.InsertInt("test2", 3)
	attrs.InsertBool("test3", true)
	return attrs
}

func TestConvertSummaryTransforms(t *testing.T) {
	tests := []struct {
		name string
		inv  common.Invocation
		want func(pmetric.MetricSlice)
	}{
		{
			name: "convert_summary_count_val_to_sum",
			inv: common.Invocation{
				Function: "convert_summary_count_val_to_sum",
				Arguments: []common.Value{
					{
						String: testhelper.Strp("delta"),
					},
					{
						Bool: (*common.Boolean)(testhelper.Boolp(false)),
					},
				},
			},
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetDataType(pmetric.MetricDataTypeSum)
				sumMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(false)

				sumMetric.SetName("summary_metric_count")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(100)

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name: "convert_summary_sum_val_to_sum",
			inv: common.Invocation{
				Function: "convert_summary_sum_val_to_sum",
				Arguments: []common.Value{
					{
						String: testhelper.Strp("delta"),
					},
					{
						Bool: (*common.Boolean)(testhelper.Boolp(false)),
					},
				},
			},
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetDataType(pmetric.MetricDataTypeSum)
				sumMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(false)

				sumMetric.SetName("summary_metric_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(12.34)

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualMetrics := pmetric.NewMetricSlice()
			summaryMetric := getTestSummaryMetric()
			summaryMetric.CopyTo(actualMetrics.AppendEmpty())

			evaluate, err := common.NewFunctionCall(tt.inv, DefaultFunctions(), ParsePath)
			assert.NoError(t, err)
			evaluate(metricTransformContext{
				il:       pcommon.NewInstrumentationScope(),
				resource: pcommon.NewResource(),
				metric:   summaryMetric,
				metrics:  actualMetrics,
			})

			expected := pmetric.NewMetricSlice()
			tt.want(expected)
			assert.Equal(t, expected, actualMetrics)
		})
	}
}
