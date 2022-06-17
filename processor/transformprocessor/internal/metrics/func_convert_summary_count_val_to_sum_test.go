// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
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

func getTestGaugeMetric() pmetric.Metric {
	metricInput := pmetric.NewMetric()
	metricInput.SetDataType(pmetric.MetricDataTypeGauge)
	metricInput.SetName("gauge_metric")
	input := metricInput.Gauge().DataPoints().AppendEmpty()
	input.SetIntVal(12)

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

func summaryTest(tests []summaryTestCase, t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualMetrics := pmetric.NewMetricSlice()
			tt.input.CopyTo(actualMetrics.AppendEmpty())

			evaluate, err := common.NewFunctionCall(tt.inv, DefaultFunctions(), ParsePath)
			assert.NoError(t, err)
			evaluate(metricTransformContext{
				il:       pcommon.NewInstrumentationScope(),
				resource: pcommon.NewResource(),
				metric:   tt.input,
				metrics:  actualMetrics,
			})

			expected := pmetric.NewMetricSlice()
			tt.want(expected)
			assert.Equal(t, expected, actualMetrics)
		})
	}
}

type summaryTestCase struct {
	name  string
	input pmetric.Metric
	inv   common.Invocation
	want  func(pmetric.MetricSlice)
}

func Test_ConvertSummarySumValToSum(t *testing.T) {
	tests := []summaryTestCase{
		{
			name:  "convert_summary_sum_val_to_sum",
			input: getTestSummaryMetric(),
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
		{
			name:  "convert_summary_sum_val_to_sum (monotonic)",
			input: getTestSummaryMetric(),
			inv: common.Invocation{
				Function: "convert_summary_sum_val_to_sum",
				Arguments: []common.Value{
					{
						String: testhelper.Strp("delta"),
					},
					{
						Bool: (*common.Boolean)(testhelper.Boolp(true)),
					},
				},
			},
			want: func(metrics pmetric.MetricSlice) {
				summaryMetric := getTestSummaryMetric()
				summaryMetric.CopyTo(metrics.AppendEmpty())
				sumMetric := metrics.AppendEmpty()
				sumMetric.SetDataType(pmetric.MetricDataTypeSum)
				sumMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				sumMetric.Sum().SetIsMonotonic(true)

				sumMetric.SetName("summary_metric_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(12.34)

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:  "convert_summary_sum_val_to_sum (cumulative)",
			input: getTestSummaryMetric(),
			inv: common.Invocation{
				Function: "convert_summary_sum_val_to_sum",
				Arguments: []common.Value{
					{
						String: testhelper.Strp("cumulative"),
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
				sumMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				sumMetric.Sum().SetIsMonotonic(false)

				sumMetric.SetName("summary_metric_sum")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(12.34)

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:  "convert_summary_sum_val_to_sum (no op)",
			input: getTestGaugeMetric(),
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
				gaugeMetric := getTestGaugeMetric()
				gaugeMetric.CopyTo(metrics.AppendEmpty())
			},
		},
	}
	summaryTest(tests, t)
}

func Test_ConvertSummarySumValToSum_validation(t *testing.T) {
	tests := []struct {
		name          string
		stringAggTemp string
	}{
		{
			name:          "invalid aggregation temporality",
			stringAggTemp: "not a real aggregation temporality",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertSummarySumValToSum(tt.stringAggTemp, true)
			assert.Error(t, err, "unknown aggregation temporality: not a real aggregation temporality")
		})
	}
}
