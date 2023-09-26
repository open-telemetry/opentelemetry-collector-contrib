// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// deleteLabelValueOp deletes a label value and all data associated with it
func deleteLabelValueOp(metric pmetric.Metric, mtpOp internalOperation) {
	op := mtpOp.configOperation
	//exhaustive:enforce
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		metric.Gauge().DataPoints().RemoveIf(func(dp pmetric.NumberDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	case pmetric.MetricTypeSum:
		metric.Sum().DataPoints().RemoveIf(func(dp pmetric.NumberDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	case pmetric.MetricTypeHistogram:
		metric.Histogram().DataPoints().RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	case pmetric.MetricTypeExponentialHistogram:
		metric.ExponentialHistogram().DataPoints().RemoveIf(func(dp pmetric.ExponentialHistogramDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	case pmetric.MetricTypeSummary:
		metric.Summary().DataPoints().RemoveIf(func(dp pmetric.SummaryDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	}
}

func hasAttr(attrs pcommon.Map, k, v string) bool {
	if val, ok := attrs.Get(k); ok {
		return val.Str() == v
	}
	return false
}
