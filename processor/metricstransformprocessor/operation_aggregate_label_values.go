// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
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

	ag := aggregateutil.AggGroups{}
	newMetric := pmetric.NewMetric()
	copyMetricDetails(metric, newMetric)
	aggregateutil.GroupDataPoints(metric, &ag)
	aggregateutil.MergeDataPoints(newMetric, mtpOp.configOperation.AggregationType, ag)
	newMetric.MoveTo(metric)
}
