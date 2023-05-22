// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// addLabelOp add a new attribute to metric data points.
func addLabelOp(metric pmetric.Metric, op internalOperation) {
	rangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
		if _, found := attrs.Get(op.configOperation.NewLabel); !found {
			attrs.PutStr(op.configOperation.NewLabel, op.configOperation.NewValue)
		}
		return true
	})
}
