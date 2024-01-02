// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// updateLabelOp updates labels and label values in metric based on given operation
func updateLabelOp(metric pmetric.Metric, mtpOp internalOperation, f internalFilter) {
	op := mtpOp.configOperation
	rangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
		if !f.matchAttrs(attrs) {
			return true
		}

		attrKey := op.Label
		attrVal, ok := attrs.Get(attrKey)
		if !ok {
			return true
		}

		if op.NewLabel != "" {
			attrVal.CopyTo(attrs.PutEmpty(op.NewLabel))
			attrs.Remove(attrKey)
			attrKey = op.NewLabel
		}

		if newValue, ok := mtpOp.valueActionsMapping[attrVal.Str()]; ok {
			attrs.PutStr(attrKey, newValue)
		}
		return true
	})
}
