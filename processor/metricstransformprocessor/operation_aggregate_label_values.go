// Copyright The OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// aggregateLabelValuesOp aggregates points that have the label values specified in aggregated_values
func aggregateLabelValuesOp(metric pmetric.Metric, mtpOp internalOperation) {
	rangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
		val, ok := attrs.Get(mtpOp.configOperation.Label)
		if !ok {
			return true
		}

		if _, ok := mtpOp.aggregatedValuesSet[val.Str()]; ok {
			val.SetStr(mtpOp.configOperation.NewValue)
		}
		return true
	})

	newMetric := pmetric.NewMetric()
	copyMetricDetails(metric, newMetric)
	ag := groupDataPoints(metric, aggGroups{})
	mergeDataPoints(newMetric, mtpOp.configOperation.AggregationType, ag)
	newMetric.MoveTo(metric)
}
