// Copyright 2020 OpenTelemetry Authors
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

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor/internal/datapoint"
)

// toggleScalarDataTypeOp translates the numeric value type to the opposite type, int -> double and double -> int.
// Applicable to sum and gauge metrics only.
func toggleScalarDataTypeOp(metric pmetric.Metric, f internalFilter) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge, pmetric.MetricTypeSum:
		rangeDatapoints(metric, func(dp datapoint.Datapoint) bool {
			if !f.matchAttrs(dp.Attributes()) {
				return true
			}
			v := dp.(pmetric.NumberDataPoint)
			switch v.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				v.SetDoubleValue(float64(v.IntValue()))
			case pmetric.NumberDataPointValueTypeDouble:
				v.SetIntValue(int64(v.DoubleValue()))
			}
			return true
		})
	default:
		return
	}
}
