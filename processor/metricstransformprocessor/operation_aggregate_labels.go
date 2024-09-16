// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
)

// aggregateLabelsOp aggregates points that have the labels excluded in label_set
func aggregateLabelsOp(metric pmetric.Metric, attributes []string, aggrType aggregateutil.AggregationType) {
	ag := aggregateutil.AggGroups{}
	aggregateutil.FilterAttrs(metric, attributes)
	newMetric := pmetric.NewMetric()
	copyMetricDetails(metric, newMetric)
	aggregateutil.GroupDataPoints(metric, &ag)
	aggregateutil.MergeDataPoints(newMetric, aggrType, ag)
	newMetric.MoveTo(metric)
}
