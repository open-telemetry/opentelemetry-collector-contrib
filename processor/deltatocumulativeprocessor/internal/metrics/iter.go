// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/putil/pslice"
)

func All(md pmetric.Metrics) func(func(Metric) bool) {
	return func(yield func(Metric) bool) {
		var ok bool
		pslice.All(md.ResourceMetrics())(func(rm pmetric.ResourceMetrics) bool {
			pslice.All(rm.ScopeMetrics())(func(sm pmetric.ScopeMetrics) bool {
				pslice.All(sm.Metrics())(func(m pmetric.Metric) bool {
					ok = yield(From(rm.Resource(), sm.Scope(), m))
					return ok
				})
				return ok
			})
			return ok
		})
	}
}

func Filter(md pmetric.Metrics, keep func(Metric) bool) {
	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				return !keep(From(rm.Resource(), sm.Scope(), m))
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
}
