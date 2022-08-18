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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// deleteLabelValueOp deletes a label value and all data associated with it
func deleteLabelValueOp(metric pmetric.Metric, mtpOp internalOperation) {
	op := mtpOp.configOperation
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		metric.Gauge().DataPoints().RemoveIf(func(dp pmetric.NumberDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	case pmetric.MetricDataTypeSum:
		metric.Sum().DataPoints().RemoveIf(func(dp pmetric.NumberDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	case pmetric.MetricDataTypeHistogram:
		metric.Histogram().DataPoints().RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	case pmetric.MetricDataTypeExponentialHistogram:
		metric.ExponentialHistogram().DataPoints().RemoveIf(func(dp pmetric.ExponentialHistogramDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	case pmetric.MetricDataTypeSummary:
		metric.Summary().DataPoints().RemoveIf(func(dp pmetric.SummaryDataPoint) bool {
			return hasAttr(dp.Attributes(), op.Label, op.LabelValue)
		})
	}
}

func hasAttr(attrs pcommon.Map, k, v string) bool {
	if val, ok := attrs.Get(k); ok {
		return val.StringVal() == v
	}
	return false
}
