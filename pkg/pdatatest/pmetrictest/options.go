// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetrictest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"

import (
	"bytes"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// CompareMetricsOption can be used to mutate expected and/or actual metrics before comparing.
type CompareMetricsOption interface {
	applyOnMetrics(expected, actual pmetric.Metrics)
}

type compareMetricsOptionFunc func(expected, actual pmetric.Metrics)

func (f compareMetricsOptionFunc) applyOnMetrics(expected, actual pmetric.Metrics) {
	f(expected, actual)
}

// IgnoreMetricValues is a CompareMetricsOption that clears all metric values.
func IgnoreMetricValues(metricNames ...string) CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		maskMetricValues(expected, metricNames...)
		maskMetricValues(actual, metricNames...)
	})
}

func maskMetricValues(metrics pmetric.Metrics, metricNames ...string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			maskMetricSliceValues(ilms.At(j).Metrics(), metricNames...)
		}
	}
}

// maskMetricSliceValues sets all data point values to zero.
func maskMetricSliceValues(metrics pmetric.MetricSlice, metricNames ...string) {
	metricNameSet := make(map[string]bool, len(metricNames))
	for _, metricName := range metricNames {
		metricNameSet[metricName] = true
	}
	for i := 0; i < metrics.Len(); i++ {
		if len(metricNames) == 0 || metricNameSet[metrics.At(i).Name()] {
			maskDataPointSliceValues(getDataPointSlice(metrics.At(i)))
		}
	}
}

func getDataPointSlice(metric pmetric.Metric) pmetric.NumberDataPointSlice {
	var dataPointSlice pmetric.NumberDataPointSlice
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dataPointSlice = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dataPointSlice = metric.Sum().DataPoints()
	default:
		panic(fmt.Sprintf("data type not supported: %s", metric.Type()))
	}
	return dataPointSlice
}

// maskDataPointSliceValues sets all data point values to zero.
func maskDataPointSliceValues(dataPoints pmetric.NumberDataPointSlice) {
	for i := 0; i < dataPoints.Len(); i++ {
		dataPoint := dataPoints.At(i)
		dataPoint.SetIntValue(0)
		dataPoint.SetDoubleValue(0)
	}
}

// IgnoreTimestamp is a CompareMetricsOption that clears Timestamp fields on all the data points.
func IgnoreTimestamp() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		now := pcommon.NewTimestampFromTime(time.Now())
		maskTimestamp(expected, now)
		maskTimestamp(actual, now)
	})
}

func maskTimestamp(metrics pmetric.Metrics, ts pcommon.Timestamp) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		for j := 0; j < rms.At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < rms.At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				m := rms.At(i).ScopeMetrics().At(j).Metrics().At(k)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					for l := 0; l < m.Gauge().DataPoints().Len(); l++ {
						m.Gauge().DataPoints().At(l).SetTimestamp(ts)
					}
				case pmetric.MetricTypeSum:
					for l := 0; l < m.Sum().DataPoints().Len(); l++ {
						m.Sum().DataPoints().At(l).SetTimestamp(ts)
					}
				case pmetric.MetricTypeHistogram:
					for l := 0; l < m.Histogram().DataPoints().Len(); l++ {
						m.Histogram().DataPoints().At(l).SetTimestamp(ts)
					}
				case pmetric.MetricTypeExponentialHistogram:
					for l := 0; l < m.ExponentialHistogram().DataPoints().Len(); l++ {
						m.ExponentialHistogram().DataPoints().At(l).SetTimestamp(ts)
					}
				case pmetric.MetricTypeSummary:
					for l := 0; l < m.Summary().DataPoints().Len(); l++ {
						m.Summary().DataPoints().At(l).SetTimestamp(ts)
					}
				}
			}
		}
	}
}

// IgnoreStartTimestamp is a CompareMetricsOption that clears StartTimestamp fields on all the data points.
func IgnoreStartTimestamp() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		now := pcommon.NewTimestampFromTime(time.Now())
		maskStartTimestamp(expected, now)
		maskStartTimestamp(actual, now)
	})
}

func maskStartTimestamp(metrics pmetric.Metrics, ts pcommon.Timestamp) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		for j := 0; j < rms.At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < rms.At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				m := rms.At(i).ScopeMetrics().At(j).Metrics().At(k)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					for l := 0; l < m.Gauge().DataPoints().Len(); l++ {
						m.Gauge().DataPoints().At(l).SetStartTimestamp(ts)
					}
				case pmetric.MetricTypeSum:
					for l := 0; l < m.Sum().DataPoints().Len(); l++ {
						m.Sum().DataPoints().At(l).SetStartTimestamp(ts)
					}
				case pmetric.MetricTypeHistogram:
					for l := 0; l < m.Histogram().DataPoints().Len(); l++ {
						m.Histogram().DataPoints().At(l).SetStartTimestamp(ts)
					}
				case pmetric.MetricTypeExponentialHistogram:
					for l := 0; l < m.ExponentialHistogram().DataPoints().Len(); l++ {
						m.ExponentialHistogram().DataPoints().At(l).SetStartTimestamp(ts)
					}
				case pmetric.MetricTypeSummary:
					for l := 0; l < m.Summary().DataPoints().Len(); l++ {
						m.Summary().DataPoints().At(l).SetStartTimestamp(ts)
					}
				}
			}
		}
	}
}

// IgnoreMetricAttributeValue is a CompareMetricsOption that clears value of the metric attribute.
func IgnoreMetricAttributeValue(attributeName string, metricNames ...string) CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		maskMetricAttributeValue(expected, attributeName, metricNames)
		maskMetricAttributeValue(actual, attributeName, metricNames)
	})
}

func maskMetricAttributeValue(metrics pmetric.Metrics, attributeName string, metricNames []string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			maskMetricSliceAttributeValues(ilms.At(j).Metrics(), attributeName, metricNames)
		}
	}
}

// maskMetricSliceAttributeValues sets the value of the specified attribute to
// the zero value associated with the attribute data type.
// If metric names are specified, only the data points within those metrics will be masked.
// Otherwise, all data points with the attribute will be masked.
func maskMetricSliceAttributeValues(metrics pmetric.MetricSlice, attributeName string, metricNames []string) {
	metricNameSet := make(map[string]bool, len(metricNames))
	for _, metricName := range metricNames {
		metricNameSet[metricName] = true
	}

	for i := 0; i < metrics.Len(); i++ {
		if len(metricNames) == 0 || metricNameSet[metrics.At(i).Name()] {
			dps := getDataPointSlice(metrics.At(i))
			maskDataPointSliceAttributeValues(dps, attributeName)

			// If attribute values are ignored, some data points may become
			// indistinguishable from each other, but sorting by value allows
			// for a reasonably thorough comparison and a deterministic outcome.
			dps.Sort(func(a, b pmetric.NumberDataPoint) bool {
				if a.IntValue() < b.IntValue() {
					return true
				}
				if a.DoubleValue() < b.DoubleValue() {
					return true
				}
				return false
			})
		}
	}
}

// maskDataPointSliceAttributeValues sets the value of the specified attribute to
// the zero value associated with the attribute data type.
func maskDataPointSliceAttributeValues(dataPoints pmetric.NumberDataPointSlice, attributeName string) {
	for i := 0; i < dataPoints.Len(); i++ {
		attributes := dataPoints.At(i).Attributes()
		attribute, ok := attributes.Get(attributeName)
		if ok {
			switch attribute.Type() {
			case pcommon.ValueTypeStr:
				attribute.SetStr("")
			default:
				panic(fmt.Sprintf("data type not supported: %s", attribute.Type()))
			}
		}
	}
}

// IgnoreResourceAttributeValue is a CompareMetricsOption that removes a resource attribute
// from all resources.
func IgnoreResourceAttributeValue(attributeName string) CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		maskMetricsResourceAttributeValue(expected, attributeName)
		maskMetricsResourceAttributeValue(actual, attributeName)
	})
}

func maskMetricsResourceAttributeValue(metrics pmetric.Metrics, attributeName string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		internal.MaskResourceAttributeValue(rms.At(i).Resource(), attributeName)
	}
}

// IgnoreSubsequentDataPoints is a CompareMetricsOption that ignores data points after the first.
func IgnoreSubsequentDataPoints(metricNames ...string) CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		maskSubsequentDataPoints(expected, metricNames)
		maskSubsequentDataPoints(actual, metricNames)
	})
}

func maskSubsequentDataPoints(metrics pmetric.Metrics, metricNames []string) {
	metricNameSet := make(map[string]bool, len(metricNames))
	for _, metricName := range metricNames {
		metricNameSet[metricName] = true
	}

	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				if len(metricNames) == 0 || metricNameSet[ms.At(k).Name()] {
					dps := getDataPointSlice(ms.At(k))
					n := 0
					dps.RemoveIf(func(pmetric.NumberDataPoint) bool {
						n++
						return n > 1
					})
				}
			}
		}
	}
}

// IgnoreResourceMetricsOrder is a CompareMetricsOption that ignores the order of resource traces/metrics/logs.
func IgnoreResourceMetricsOrder() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		sortResourceMetricsSlice(expected.ResourceMetrics())
		sortResourceMetricsSlice(actual.ResourceMetrics())
	})
}

func sortResourceMetricsSlice(rms pmetric.ResourceMetricsSlice) {
	rms.Sort(func(a, b pmetric.ResourceMetrics) bool {
		if a.SchemaUrl() != b.SchemaUrl() {
			return a.SchemaUrl() < b.SchemaUrl()
		}
		aAttrs := pdatautil.MapHash(a.Resource().Attributes())
		bAttrs := pdatautil.MapHash(b.Resource().Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

// IgnoreScopeMetricsOrder is a CompareMetricsOption that ignores the order of instrumentation scope traces/metrics/logs.
func IgnoreScopeMetricsOrder() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		sortScopeMetricsSlices(expected)
		sortScopeMetricsSlices(actual)
	})
}

func sortScopeMetricsSlices(ms pmetric.Metrics) {
	for i := 0; i < ms.ResourceMetrics().Len(); i++ {
		ms.ResourceMetrics().At(i).ScopeMetrics().Sort(func(a, b pmetric.ScopeMetrics) bool {
			if a.SchemaUrl() != b.SchemaUrl() {
				return a.SchemaUrl() < b.SchemaUrl()
			}
			if a.Scope().Name() != b.Scope().Name() {
				return a.Scope().Name() < b.Scope().Name()
			}
			return a.Scope().Version() < b.Scope().Version()
		})
	}
}

// IgnoreMetricsOrder is a CompareMetricsOption that ignores the order of metrics.
func IgnoreMetricsOrder() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		sortMetricSlices(expected)
		sortMetricSlices(actual)
	})
}

func sortMetricSlices(ms pmetric.Metrics) {
	for i := 0; i < ms.ResourceMetrics().Len(); i++ {
		for j := 0; j < ms.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Sort(func(a, b pmetric.Metric) bool {
				return a.Name() < b.Name()
			})
		}
	}
}

// IgnoreMetricDataPointsOrder is a CompareMetricsOption that ignores the order of metrics.
func IgnoreMetricDataPointsOrder() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		sortMetricDataPointSlices(expected)
		sortMetricDataPointSlices(actual)
	})
}

func sortMetricDataPointSlices(ms pmetric.Metrics) {
	for i := 0; i < ms.ResourceMetrics().Len(); i++ {
		for j := 0; j < ms.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				m := ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					sortNumberDataPointSlice(m.Gauge().DataPoints())
				case pmetric.MetricTypeSum:
					sortNumberDataPointSlice(m.Sum().DataPoints())
				case pmetric.MetricTypeHistogram:
					sortHistogramDataPointSlice(m.Histogram().DataPoints())
				case pmetric.MetricTypeExponentialHistogram:
					sortExponentialHistogramDataPointSlice(m.ExponentialHistogram().DataPoints())
				case pmetric.MetricTypeSummary:
					sortSummaryDataPointSlice(m.Summary().DataPoints())
				}
			}
		}
	}
}

func sortNumberDataPointSlice(ndps pmetric.NumberDataPointSlice) {
	ndps.Sort(func(a, b pmetric.NumberDataPoint) bool {
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortHistogramDataPointSlice(hdps pmetric.HistogramDataPointSlice) {
	hdps.Sort(func(a, b pmetric.HistogramDataPoint) bool {
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortExponentialHistogramDataPointSlice(hdps pmetric.ExponentialHistogramDataPointSlice) {
	hdps.Sort(func(a, b pmetric.ExponentialHistogramDataPoint) bool {
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortSummaryDataPointSlice(sds pmetric.SummaryDataPointSlice) {
	sds.Sort(func(a, b pmetric.SummaryDataPoint) bool {
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

// IgnoreSummaryDataPointValueAtQuantileSliceOrder is a CompareMetricsOption that ignores the order of summary data point quantile slice.
func IgnoreSummaryDataPointValueAtQuantileSliceOrder() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		sortSummaryDataPointValueAtQuantileSlices(expected)
		sortSummaryDataPointValueAtQuantileSlices(actual)
	})
}

func sortSummaryDataPointValueAtQuantileSlices(ms pmetric.Metrics) {
	for i := 0; i < ms.ResourceMetrics().Len(); i++ {
		for j := 0; j < ms.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				m := ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
				if m.Type() == pmetric.MetricTypeSummary {
					for l := 0; l < m.Summary().DataPoints().Len(); l++ {
						m.Summary().DataPoints().At(l).QuantileValues().Sort(func(a, b pmetric.SummaryDataPointValueAtQuantile) bool {
							return a.Quantile() < b.Quantile()
						})
					}
				}
			}
		}
	}
}
