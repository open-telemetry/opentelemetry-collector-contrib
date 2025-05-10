// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsaggregationprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"time"

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

func truncateTimestamps(metric pmetric.Metric, interval time.Duration) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
			dp := metric.Gauge().DataPoints().At(i)
			dp.SetStartTimestamp(truncatePrior(dp.StartTimestamp(), interval))
			dp.SetTimestamp(truncateNext(dp.Timestamp(), interval))
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			dp := metric.Sum().DataPoints().At(i)
			dp.SetStartTimestamp(truncatePrior(dp.StartTimestamp(), interval))
			dp.SetTimestamp(truncateNext(dp.Timestamp(), interval))
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
			dp := metric.Histogram().DataPoints().At(i)
			dp.SetStartTimestamp(truncatePrior(dp.StartTimestamp(), interval))
			dp.SetTimestamp(truncateNext(dp.Timestamp(), interval))
		}
	case pmetric.MetricTypeExponentialHistogram:
		for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
			dp := metric.ExponentialHistogram().DataPoints().At(i)
			dp.SetStartTimestamp(truncatePrior(dp.StartTimestamp(), interval))
			dp.SetTimestamp(truncateNext(dp.Timestamp(), interval))
		}
	case pmetric.MetricTypeSummary:
		for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
			dp := metric.ExponentialHistogram().DataPoints().At(i)
			dp.SetStartTimestamp(truncatePrior(dp.StartTimestamp(), interval))
			dp.SetTimestamp(truncateNext(dp.Timestamp(), interval))
		}
	case pmetric.MetricTypeEmpty:
		return
	}
}

func truncateNext(ts pcommon.Timestamp, interval time.Duration) pcommon.Timestamp {
	iv := uint64(interval)
	// Truncate the timestamp to the next interval
	return pcommon.Timestamp(uint64(ts)/iv*iv + iv)
}

func truncatePrior(ts pcommon.Timestamp, interval time.Duration) pcommon.Timestamp {
	iv := uint64(interval)
	return pcommon.Timestamp(uint64(ts) / iv * iv)
}
