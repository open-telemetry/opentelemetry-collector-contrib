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

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func convertGaugeToSum(stringAggTemp string, monotonic bool) (common.ExprFunc, error) {
	var aggTemp pmetric.MetricAggregationTemporality
	switch stringAggTemp {
	case "delta":
		aggTemp = pmetric.MetricAggregationTemporalityDelta
	case "cumulative":
		aggTemp = pmetric.MetricAggregationTemporalityCumulative
	default:
		return nil, fmt.Errorf("unknown aggregation temporality: %s", stringAggTemp)
	}

	return func(ctx common.TransformContext) interface{} {
		mtc, ok := ctx.(metricTransformContext)
		if !ok {
			return nil
		}

		metric := mtc.GetMetric()
		if metric.DataType() != pmetric.MetricDataTypeGauge {
			return nil
		}

		dps := metric.Gauge().DataPoints()

		metric.SetDataType(pmetric.MetricDataTypeSum)
		metric.Sum().SetAggregationTemporality(aggTemp)
		metric.Sum().SetIsMonotonic(monotonic)

		// Setting the data type removed all the data points, so we must copy them back to the metric.
		dps.CopyTo(metric.Sum().DataPoints())

		return nil
	}, nil
}
