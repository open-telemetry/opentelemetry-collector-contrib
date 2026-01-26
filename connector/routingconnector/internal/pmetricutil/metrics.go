// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pmetricutil"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pdatautil"
)

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

// CopyResourcesIf calls f sequentially for each ResourceSpans present in the first pmetric.Metrics.
// If f returns true, the element is copied from the first pmetric.Metrics to the second pmetric.Metrics.
func CopyResourcesIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics) bool) {
	for i := 0; i < from.ResourceMetrics().Len(); i++ {
		rm := from.ResourceMetrics().At(i)
		if f(rm) {
			rm.CopyTo(to.ResourceMetrics().AppendEmpty())
		}
	}
}

// MoveMetricsWithContextIf calls f sequentially for each Metric present in the first pmetric.Metrics.
// If f returns true, the element is removed from the first pmetric.Metrics and added to the second pmetric.Metrics.
// Notably, the Resource and Scope associated with the Metric are created in the second pmetric.Metrics only once.
// Resources or Scopes are removed from the original if they become empty. All ordering is preserved.
func MoveMetricsWithContextIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric) bool) {
	rms := from.ResourceMetrics()
	rms.RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		sms := rm.ScopeMetrics()
		var rmCopy pdatautil.OnceValue[pmetric.ResourceMetrics]
		sms.RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			ms := sm.Metrics()
			var smCopy pdatautil.OnceValue[pmetric.ScopeMetrics]
			ms.RemoveIf(func(m pmetric.Metric) bool {
				if !f(rm, sm, m) {
					return false
				}
				if !rmCopy.IsInit() {
					rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
				}
				if !smCopy.IsInit() {
					smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
				}
				m.MoveTo(smCopy.Value().Metrics().AppendEmpty())
				return true
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
}

// CopyMetricsWithContextIf calls f sequentially for each Metric present in the first pmetric.Metrics.
// If f returns true, the element is copied from the first pmetric.Metrics to the second pmetric.Metrics.
// Notably, the Resource and Scope associated with the Metric are created in the second pmetric.Metrics only once.
func CopyMetricsWithContextIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric) bool) {
	for i := 0; i < from.ResourceMetrics().Len(); i++ {
		rm := from.ResourceMetrics().At(i)
		var rmCopy pdatautil.OnceValue[pmetric.ResourceMetrics]
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			var smCopy pdatautil.OnceValue[pmetric.ScopeMetrics]
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				if f(rm, sm, m) {
					if !rmCopy.IsInit() {
						rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
					}
					if !smCopy.IsInit() {
						smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
					}
					m.CopyTo(smCopy.Value().Metrics().AppendEmpty())
				}
			}
		}
	}
}

// MoveDataPointsWithContextIf calls f sequentially for each DataPoint present in the first pmetric.Metrics.
// If f returns true, the element is removed from the first pmetric.Metrics and added to the second pmetric.Metrics.
// Notably, the Resource, Scope, and Metric associated with the DataPoint are created in the second pmetric.Metrics only once.
// Resources, Scopes, or Metrics are removed from the original if they become empty. All ordering is preserved.
func MoveDataPointsWithContextIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric, any) bool) {
	rms := from.ResourceMetrics()
	rms.RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		sms := rm.ScopeMetrics()
		var rmCopy pdatautil.OnceValue[pmetric.ResourceMetrics]
		sms.RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			ms := sm.Metrics()
			var smCopy pdatautil.OnceValue[pmetric.ScopeMetrics]
			ms.RemoveIf(func(m pmetric.Metric) bool {
				var mCopy pdatautil.OnceValue[pmetric.Metric]

				// TODO condense this code
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					dps := m.Gauge().DataPoints()
					dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if !rmCopy.IsInit() {
							rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
						}
						if !smCopy.IsInit() {
							smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
						}
						if !mCopy.IsInit() {
							mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
							mCopy.Value().SetEmptyGauge()
						}
						dp.MoveTo(mCopy.Value().Gauge().DataPoints().AppendEmpty())
						return true
					})
					return dps.Len() == 0
				case pmetric.MetricTypeSum:
					dps := m.Sum().DataPoints()
					dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if !rmCopy.IsInit() {
							rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
						}
						if !smCopy.IsInit() {
							smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
						}
						if !mCopy.IsInit() {
							mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
							mCopy.Value().SetEmptySum().SetAggregationTemporality(m.Sum().AggregationTemporality())
							mCopy.Value().Sum().SetIsMonotonic(m.Sum().IsMonotonic())
						}
						dp.MoveTo(mCopy.Value().Sum().DataPoints().AppendEmpty())
						return true
					})
					return dps.Len() == 0
				case pmetric.MetricTypeHistogram:
					dps := m.Histogram().DataPoints()
					dps.RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if !rmCopy.IsInit() {
							rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
						}
						if !smCopy.IsInit() {
							smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
						}
						if !mCopy.IsInit() {
							mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
							mCopy.Value().SetEmptyHistogram().SetAggregationTemporality(m.Histogram().AggregationTemporality())
						}
						dp.MoveTo(mCopy.Value().Histogram().DataPoints().AppendEmpty())
						return true
					})
					return dps.Len() == 0
				case pmetric.MetricTypeExponentialHistogram:
					dps := m.ExponentialHistogram().DataPoints()
					dps.RemoveIf(func(dp pmetric.ExponentialHistogramDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if !rmCopy.IsInit() {
							rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
						}
						if !smCopy.IsInit() {
							smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
						}
						if !mCopy.IsInit() {
							mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
							mCopy.Value().SetEmptyExponentialHistogram().SetAggregationTemporality(m.ExponentialHistogram().AggregationTemporality())
						}
						dp.MoveTo(mCopy.Value().ExponentialHistogram().DataPoints().AppendEmpty())
						return true
					})
					return dps.Len() == 0
				case pmetric.MetricTypeSummary:
					dps := m.Summary().DataPoints()
					dps.RemoveIf(func(dp pmetric.SummaryDataPoint) bool {
						if !f(rm, sm, m, dp) {
							return false
						}
						if !rmCopy.IsInit() {
							rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
						}
						if !smCopy.IsInit() {
							smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
						}
						if !mCopy.IsInit() {
							mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
							mCopy.Value().SetEmptySummary()
						}
						dp.MoveTo(mCopy.Value().Summary().DataPoints().AppendEmpty())
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

// CopyDataPointsWithContextIf calls f sequentially for each DataPoint present in the first pmetric.Metrics.
// If f returns true, the element is copied from the first pmetric.Metrics to the second pmetric.Metrics.
// Notably, the Resource, Scope, and Metric associated with the DataPoint are created in the second pmetric.Metrics only once.
func CopyDataPointsWithContextIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric, any) bool) {
	for i := 0; i < from.ResourceMetrics().Len(); i++ {
		rm := from.ResourceMetrics().At(i)
		var rmCopy pdatautil.OnceValue[pmetric.ResourceMetrics]
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			var smCopy pdatautil.OnceValue[pmetric.ScopeMetrics]
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				var mCopy pdatautil.OnceValue[pmetric.Metric]
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					for l := 0; l < m.Gauge().DataPoints().Len(); l++ {
						dp := m.Gauge().DataPoints().At(l)
						if f(rm, sm, m, dp) {
							if !rmCopy.IsInit() {
								rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
							}
							if !smCopy.IsInit() {
								smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
							}
							if !mCopy.IsInit() {
								mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
								mCopy.Value().SetEmptyGauge()
							}
							dp.CopyTo(mCopy.Value().Gauge().DataPoints().AppendEmpty())
						}
					}
				case pmetric.MetricTypeSum:
					for l := 0; l < m.Sum().DataPoints().Len(); l++ {
						dp := m.Sum().DataPoints().At(l)
						if f(rm, sm, m, dp) {
							if !rmCopy.IsInit() {
								rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
							}
							if !smCopy.IsInit() {
								smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
							}
							if !mCopy.IsInit() {
								mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
								mCopy.Value().SetEmptySum().SetAggregationTemporality(m.Sum().AggregationTemporality())
								mCopy.Value().Sum().SetIsMonotonic(m.Sum().IsMonotonic())
							}
							dp.CopyTo(mCopy.Value().Sum().DataPoints().AppendEmpty())
						}
					}
				case pmetric.MetricTypeHistogram:
					for l := 0; l < m.Histogram().DataPoints().Len(); l++ {
						dp := m.Histogram().DataPoints().At(l)
						if f(rm, sm, m, dp) {
							if !rmCopy.IsInit() {
								rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
							}
							if !smCopy.IsInit() {
								smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
							}
							if !mCopy.IsInit() {
								mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
								mCopy.Value().SetEmptyHistogram().SetAggregationTemporality(m.Histogram().AggregationTemporality())
							}
							dp.CopyTo(mCopy.Value().Histogram().DataPoints().AppendEmpty())
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					for l := 0; l < m.ExponentialHistogram().DataPoints().Len(); l++ {
						dp := m.ExponentialHistogram().DataPoints().At(l)
						if f(rm, sm, m, dp) {
							if !rmCopy.IsInit() {
								rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
							}
							if !smCopy.IsInit() {
								smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
							}
							if !mCopy.IsInit() {
								mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
								mCopy.Value().SetEmptyExponentialHistogram().SetAggregationTemporality(m.ExponentialHistogram().AggregationTemporality())
							}
							dp.CopyTo(mCopy.Value().ExponentialHistogram().DataPoints().AppendEmpty())
						}
					}
				case pmetric.MetricTypeSummary:
					for l := 0; l < m.Summary().DataPoints().Len(); l++ {
						dp := m.Summary().DataPoints().At(l)
						if f(rm, sm, m, dp) {
							if !rmCopy.IsInit() {
								rmCopy.Init(copyResourceMetrics(rm, to.ResourceMetrics()))
							}
							if !smCopy.IsInit() {
								smCopy.Init(copyScopeMetrics(sm, rmCopy.Value().ScopeMetrics()))
							}
							if !mCopy.IsInit() {
								mCopy.Init(copyMetricDescription(m, smCopy.Value().Metrics()))
								mCopy.Value().SetEmptySummary()
							}
							dp.CopyTo(mCopy.Value().Summary().DataPoints().AppendEmpty())
						}
					}
				}
			}
		}
	}
}

func copyResourceMetrics(from pmetric.ResourceMetrics, to pmetric.ResourceMetricsSlice) pmetric.ResourceMetrics {
	rmc := to.AppendEmpty()
	from.Resource().CopyTo(rmc.Resource())
	rmc.SetSchemaUrl(from.SchemaUrl())
	return rmc
}

func copyScopeMetrics(from pmetric.ScopeMetrics, to pmetric.ScopeMetricsSlice) pmetric.ScopeMetrics {
	smc := to.AppendEmpty()
	from.Scope().CopyTo(smc.Scope())
	smc.SetSchemaUrl(from.SchemaUrl())
	return smc
}

func copyMetricDescription(from pmetric.Metric, to pmetric.MetricSlice) pmetric.Metric {
	mc := to.AppendEmpty()
	mc.SetName(from.Name())
	mc.SetDescription(from.Description())
	mc.SetUnit(from.Unit())
	return mc
}
