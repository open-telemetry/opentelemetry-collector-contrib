// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pmetricutil"

import "go.opentelemetry.io/collector/pdata/pmetric"

// MoveResourcesIf calls f sequentially for each ResourceSpans present in the first pmetric.Metrics.
// If f returns true, the element is removed from the first pmetric.Metrics and added to the second pmetric.Metrics.
func MoveResourcesIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics) bool) {
	from.ResourceMetrics().RemoveIf(func(rs pmetric.ResourceMetrics) bool {
		if !f(rs) {
			return false
		}
		rs.CopyTo(to.ResourceMetrics().AppendEmpty())
		return true
	})
}

// MoveMetricsWithContextIf calls f sequentially for each Metric present in the first pmetric.Metrics.
// If f returns true, the element is removed from the first pmetric.Metrics and added to the second pmetric.Metrics.
// Notably, the Resource and Scope associated with the Metric are created in the second pmetric.Metrics only once.
// Resources or Scopes are removed from the original if they become empty. All ordering is preserved.
func MoveMetricsWithContextIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric) bool) {
	rms := from.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		var rmCopy *pmetric.ResourceMetrics
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			ms := sm.Metrics()
			var smCopy *pmetric.ScopeMetrics
			ms.RemoveIf(func(m pmetric.Metric) bool {
				if !f(rm, sm, m) {
					return false
				}
				if rmCopy == nil {
					rmc := to.ResourceMetrics().AppendEmpty()
					rmCopy = &rmc
					rm.Resource().CopyTo(rmCopy.Resource())
					rmCopy.SetSchemaUrl(rm.SchemaUrl())
				}
				if smCopy == nil {
					smc := rmCopy.ScopeMetrics().AppendEmpty()
					smCopy = &smc
					sm.Scope().CopyTo(smCopy.Scope())
					smCopy.SetSchemaUrl(sm.SchemaUrl())
				}
				m.CopyTo(smCopy.Metrics().AppendEmpty())
				return true
			})
		}
		sms.RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			return sm.Metrics().Len() == 0
		})
	}
	rms.RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		return rm.ScopeMetrics().Len() == 0
	})
}
