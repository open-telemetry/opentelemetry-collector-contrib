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

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestConvertSummaryCountValToSum(t *testing.T) {
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
