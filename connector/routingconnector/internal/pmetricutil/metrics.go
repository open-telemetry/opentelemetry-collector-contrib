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
		rs.MoveTo(to.ResourceMetrics().AppendEmpty())
		return true
	})
}

// MoveMetricsWithContextIf calls f sequentially for each Metric present in the first pmetric.Metrics.
// If f returns true, the element is removed from the first pmetric.Metrics and added to the second pmetric.Metrics.
// Notably, the Resource and Scope associated with the Metric are created in the second pmetric.Metrics only once.
// Resources or Scopes are removed from the original if they become empty. All ordering is preserved.
func MoveMetricsWithContextIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric) bool) {
	rms := from.ResourceMetrics()
	rms.RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		sms := rm.ScopeMetrics()
		var rmCopy *pmetric.ResourceMetrics
		sms.RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			ms := sm.Metrics()
			var smCopy *pmetric.ScopeMetrics
			ms.RemoveIf(func(m pmetric.Metric) bool {
				if !f(rm, sm, m) {
					return false
				}
				if rmCopy == nil {
					rmCopy = copyResourceMetrics(rm, to.ResourceMetrics())
				}
				if smCopy == nil {
					smCopy = copyScopeMetrics(sm, rmCopy.ScopeMetrics())
				}
				m.MoveTo(smCopy.Metrics().AppendEmpty())
				return true
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
}

// MoveDataPointsWithContextIf calls f sequentially for each DataPoint present in the first pmetric.Metrics.
// If f returns true, the element is removed from the first pmetric.Metrics and added to the second pmetric.Metrics.
// Notably, the Resource, Scope, and Metric associated with the DataPoint are created in the second pmetric.Metrics only once.
// Resources, Scopes, or Metrics are removed from the original if they become empty. All ordering is preserved.
func MoveDataPointsWithContextIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric, any) bool) {
	rms := from.ResourceMetrics()
	rms.RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		sms := rm.ScopeMetrics()
		var rmCopy *pmetric.ResourceMetrics
		sms.RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			ms := sm.Metrics()
			var smCopy *pmetric.ScopeMetrics
			ms.RemoveIf(func(m pmetric.Metric) bool {
				var mCopy *pmetric.Metric

				// TODO condense this code
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					dps := m.Gauge().DataPoints()
					dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if rmCopy == nil {
							rmCopy = copyResourceMetrics(rm, to.ResourceMetrics())
						}
						if smCopy == nil {
							smCopy = copyScopeMetrics(sm, rmCopy.ScopeMetrics())
						}
						if mCopy == nil {
							mCopy = copyMetricDescription(m, smCopy.Metrics())
							mCopy.SetEmptyGauge()
						}
						dp.MoveTo(mCopy.Gauge().DataPoints().AppendEmpty())
						return true
					})
					return dps.Len() == 0
				case pmetric.MetricTypeSum:
					dps := m.Sum().DataPoints()
					dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if rmCopy == nil {
							rmCopy = copyResourceMetrics(rm, to.ResourceMetrics())
						}
						if smCopy == nil {
							smCopy = copyScopeMetrics(sm, rmCopy.ScopeMetrics())
						}
						if mCopy == nil {
							mCopy = copyMetricDescription(m, smCopy.Metrics())
							mCopy.SetEmptySum()
						}
						dp.MoveTo(mCopy.Sum().DataPoints().AppendEmpty())
						return true
					})
					return dps.Len() == 0
				case pmetric.MetricTypeHistogram:
					dps := m.Histogram().DataPoints()
					dps.RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if rmCopy == nil {
							rmCopy = copyResourceMetrics(rm, to.ResourceMetrics())
						}
						if smCopy == nil {
							smCopy = copyScopeMetrics(sm, rmCopy.ScopeMetrics())
						}
						if mCopy == nil {
							mCopy = copyMetricDescription(m, smCopy.Metrics())
							mCopy.SetEmptyHistogram()
						}
						dp.MoveTo(mCopy.Histogram().DataPoints().AppendEmpty())
						return true
					})
					return dps.Len() == 0
				case pmetric.MetricTypeExponentialHistogram:
					dps := m.ExponentialHistogram().DataPoints()
					dps.RemoveIf(func(dp pmetric.ExponentialHistogramDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if rmCopy == nil {
							rmCopy = copyResourceMetrics(rm, to.ResourceMetrics())
						}
						if smCopy == nil {
							smCopy = copyScopeMetrics(sm, rmCopy.ScopeMetrics())
						}
						if mCopy == nil {
							mCopy = copyMetricDescription(m, smCopy.Metrics())
							mCopy.SetEmptyExponentialHistogram()
						}
						dp.MoveTo(mCopy.ExponentialHistogram().DataPoints().AppendEmpty())
						return true
					})
					return dps.Len() == 0
				case pmetric.MetricTypeSummary:
					dps := m.Summary().DataPoints()
					dps.RemoveIf(func(dp pmetric.SummaryDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if rmCopy == nil {
							rmCopy = copyResourceMetrics(rm, to.ResourceMetrics())
						}
						if smCopy == nil {
							smCopy = copyScopeMetrics(sm, rmCopy.ScopeMetrics())
						}
						if mCopy == nil {
							mCopy = copyMetricDescription(m, smCopy.Metrics())
							mCopy.SetEmptySummary()
						}
						dp.MoveTo(mCopy.Summary().DataPoints().AppendEmpty())
						return true
					})
					return dps.Len() == 0
				}
				// Do not remove unknown type.
				return false
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
}

func copyResourceMetrics(from pmetric.ResourceMetrics, to pmetric.ResourceMetricsSlice) *pmetric.ResourceMetrics {
	rmc := to.AppendEmpty()
	from.Resource().CopyTo(rmc.Resource())
	rmc.SetSchemaUrl(from.SchemaUrl())
	return &rmc
}

func copyScopeMetrics(from pmetric.ScopeMetrics, to pmetric.ScopeMetricsSlice) *pmetric.ScopeMetrics {
	smc := to.AppendEmpty()
	from.Scope().CopyTo(smc.Scope())
	smc.SetSchemaUrl(from.SchemaUrl())
	return &smc
}

func copyMetricDescription(from pmetric.Metric, to pmetric.MetricSlice) *pmetric.Metric {
	mc := to.AppendEmpty()
	mc.SetName(from.Name())
	mc.SetDescription(from.Description())
	mc.SetUnit(from.Unit())
	return &mc
}
