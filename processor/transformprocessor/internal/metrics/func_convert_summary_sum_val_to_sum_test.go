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
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
)

func Test_ConvertSummaryCountValToSum(t *testing.T) {
	tests := []summaryTestCase{
		{
			name:  "convert_summary_count_val_to_sum",
			input: getTestSummaryMetric(),
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
			name:  "convert_summary_count_val_to_sum (monotonic)",
			input: getTestSummaryMetric(),
			inv: common.Invocation{
				Function: "convert_summary_count_val_to_sum",
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

				sumMetric.SetName("summary_metric_count")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(100)

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:  "convert_summary_count_val_to_sum",
			input: getTestSummaryMetric(),
			inv: common.Invocation{
				Function: "convert_summary_count_val_to_sum",
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

				sumMetric.SetName("summary_metric_count")
				dp := sumMetric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(100)

				attrs := getTestAttributes()
				attrs.CopyTo(dp.Attributes())
			},
		},
		{
			name:  "convert_summary_count_val_to_sum (no op)",
			input: getTestGaugeMetric(),
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
				gaugeMetric := getTestGaugeMetric()
				gaugeMetric.CopyTo(metrics.AppendEmpty())
			},
		},
	}
	summaryTest(tests, t)
}

func Test_ConvertSummaryCountValToSum_validation(t *testing.T) {
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
			_, err := convertSummaryCountValToSum(tt.stringAggTemp, true)
			assert.Error(t, err, "unknown aggregation temporality: not a real aggregation temporality")
		})
	}
}
