// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetrictest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
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
			switch metrics.At(i).Type() {
			case pmetric.MetricTypeEmpty, pmetric.MetricTypeSum, pmetric.MetricTypeGauge:
				maskDataPointSliceValues(getDataPointSlice(metrics.At(i)))
			case pmetric.MetricTypeHistogram:
				maskHistogramDataPointSliceValues(metrics.At(i).Histogram().DataPoints())
			default:
				panic(fmt.Sprintf("data type not supported: %s", metrics.At(i).Type()))
			}
		}
	}
}

func getDataPointSlice(metric pmetric.Metric) pmetric.NumberDataPointSlice {
	var dataPointSlice pmetric.NumberDataPointSlice
	//exhaustive:enforce
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dataPointSlice = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dataPointSlice = metric.Sum().DataPoints()
	case pmetric.MetricTypeEmpty:
		dataPointSlice = pmetric.NewNumberDataPointSlice()
	case pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary:
		fallthrough
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

// maskHistogramDataPointSliceValues sets all data point values to zero.
func maskHistogramDataPointSliceValues(dataPoints pmetric.HistogramDataPointSlice) {
	for i := 0; i < dataPoints.Len(); i++ {
		dataPoint := dataPoints.At(i)
		dataPoint.SetCount(0)
		dataPoint.SetSum(0)
		dataPoint.SetMin(0)
		dataPoint.SetMax(0)
		dataPoint.BucketCounts().FromRaw([]uint64{})
		dataPoint.Exemplars().RemoveIf(func(pmetric.Exemplar) bool {
			return true
		})
		dataPoint.ExplicitBounds().FromRaw([]float64{})
	}
}

// IgnoreMetricFloatPrecision is a CompareMetricsOption that rounds away float precision discrepancies in metric values.
func IgnoreMetricFloatPrecision(precision int, metricNames ...string) CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		floatMetricValues(precision, expected, metricNames...)
		floatMetricValues(precision, actual, metricNames...)
	})
}

func floatMetricValues(precision int, metrics pmetric.Metrics, metricNames ...string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			floatMetricSliceValues(precision, ilms.At(j).Metrics(), metricNames...)
		}
	}
}

// floatMetricSliceValues sets all data point values to zero.
func floatMetricSliceValues(precision int, metrics pmetric.MetricSlice, metricNames ...string) {
	metricNameSet := make(map[string]bool, len(metricNames))
	for _, metricName := range metricNames {
		metricNameSet[metricName] = true
	}
	for i := 0; i < metrics.Len(); i++ {
		if len(metricNames) == 0 || metricNameSet[metrics.At(i).Name()] {
			switch metrics.At(i).Type() {
			case pmetric.MetricTypeEmpty, pmetric.MetricTypeSum, pmetric.MetricTypeGauge:
				roundDataPointSliceValues(getDataPointSlice(metrics.At(i)), precision)
			default:
				panic(fmt.Sprintf("data type not supported: %s", metrics.At(i).Type()))
			}
		}
	}
}

// maskDataPointSliceValues rounds all data point values at a given decimal.
func roundDataPointSliceValues(dataPoints pmetric.NumberDataPointSlice, precision int) {
	for i := 0; i < dataPoints.Len(); i++ {
		dataPoint := dataPoints.At(i)
		factor := math.Pow(10, float64(precision))
		switch {
		case dataPoint.DoubleValue() != 0.0:
			dataPoint.SetDoubleValue(math.Round(dataPoint.DoubleValue()*factor) / factor)
		case dataPoint.IntValue() != 0:
			panic(fmt.Sprintf("integers can not have float precision ignored: %v", dataPoints.At(i)))
		}
	}
}

// IgnoreExemplars is a CompareMetricsOption that clears exemplar fields on all data points.
func IgnoreExemplars() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		maskExemplars(expected)
		maskExemplars(actual)
	})
}

func maskExemplars(metrics pmetric.Metrics) {
	emptyExemplar := pmetric.NewExemplar()
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		for j := 0; j < metrics.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for g := 0; g < metrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); g++ {
				m := metrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(g)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					datapoints := m.Gauge().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						for f := 0; f < datapoints.At(k).Exemplars().Len(); f++ {
							emptyExemplar.CopyTo(datapoints.At(k).Exemplars().At(f))
						}
					}
				case pmetric.MetricTypeSum:
					datapoints := m.Sum().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						for f := 0; f < datapoints.At(k).Exemplars().Len(); f++ {
							emptyExemplar.CopyTo(datapoints.At(k).Exemplars().At(f))
						}
					}
				case pmetric.MetricTypeHistogram:
					datapoints := m.Histogram().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						for f := 0; f < datapoints.At(k).Exemplars().Len(); f++ {
							emptyExemplar.CopyTo(datapoints.At(k).Exemplars().At(f))
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					datapoints := m.ExponentialHistogram().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						for f := 0; f < datapoints.At(k).Exemplars().Len(); f++ {
							emptyExemplar.CopyTo(datapoints.At(k).Exemplars().At(f))
						}
					}
				}
			}
		}
	}
}

// IgnoreExemplarSlice is a CompareMetricsOption that clears exemplars slice on all data points.
func IgnoreExemplarSlice() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		maskExemplarSlice(expected)
		maskExemplarSlice(actual)
	})
}

func maskExemplarSlice(metrics pmetric.Metrics) {
	emptyExemplarSlice := pmetric.NewExemplarSlice()
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		for j := 0; j < metrics.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for g := 0; g < metrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); g++ {
				m := metrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(g)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					datapoints := m.Gauge().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						emptyExemplarSlice.CopyTo(datapoints.At(k).Exemplars())
					}
				case pmetric.MetricTypeSum:
					datapoints := m.Sum().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						emptyExemplarSlice.CopyTo(datapoints.At(k).Exemplars())
					}
				case pmetric.MetricTypeHistogram:
					datapoints := m.Histogram().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						emptyExemplarSlice.CopyTo(datapoints.At(k).Exemplars())
					}
				case pmetric.MetricTypeExponentialHistogram:
					datapoints := m.ExponentialHistogram().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						emptyExemplarSlice.CopyTo(datapoints.At(k).Exemplars())
					}
				}
			}
		}
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
				//exhaustive:enforce
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
				case pmetric.MetricTypeEmpty:
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
				//exhaustive:enforce
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
				case pmetric.MetricTypeEmpty:
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

// IgnoreDatapointAttributesOrder is a CompareMetricsOption that ignores the order of datapoint attributes.
func IgnoreDatapointAttributesOrder() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		orderDatapointAttributes(expected)
		orderDatapointAttributes(actual)
	})
}

func orderDatapointAttributes(metrics pmetric.Metrics) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			msl := ilms.At(j).Metrics()
			for g := 0; g < msl.Len(); g++ {
				msl.At(g)
				switch msl.At(g).Type() {
				case pmetric.MetricTypeGauge:
					for k := 0; k < msl.At(g).Gauge().DataPoints().Len(); k++ {
						rawOrdered := internal.OrderMapByKey(msl.At(g).Gauge().DataPoints().At(k).Attributes().AsRaw())
						_ = msl.At(g).Gauge().DataPoints().At(k).Attributes().FromRaw(rawOrdered)
					}
				case pmetric.MetricTypeSum:
					for k := 0; k < msl.At(g).Sum().DataPoints().Len(); k++ {
						rawOrdered := internal.OrderMapByKey(msl.At(g).Sum().DataPoints().At(k).Attributes().AsRaw())
						_ = msl.At(g).Sum().DataPoints().At(k).Attributes().FromRaw(rawOrdered)
					}
				case pmetric.MetricTypeHistogram:
					for k := 0; k < msl.At(g).Histogram().DataPoints().Len(); k++ {
						rawOrdered := internal.OrderMapByKey(msl.At(g).Histogram().DataPoints().At(k).Attributes().AsRaw())
						_ = msl.At(g).Histogram().DataPoints().At(k).Attributes().FromRaw(rawOrdered)
					}
				case pmetric.MetricTypeExponentialHistogram:
					for k := 0; k < msl.At(g).ExponentialHistogram().DataPoints().Len(); k++ {
						rawOrdered := internal.OrderMapByKey(msl.At(g).ExponentialHistogram().DataPoints().At(k).Attributes().AsRaw())
						_ = msl.At(g).ExponentialHistogram().DataPoints().At(k).Attributes().FromRaw(rawOrdered)
					}
				case pmetric.MetricTypeSummary:
					for k := 0; k < msl.At(g).Summary().DataPoints().Len(); k++ {
						rawOrdered := internal.OrderMapByKey(msl.At(g).Summary().DataPoints().At(k).Attributes().AsRaw())
						_ = msl.At(g).Summary().DataPoints().At(k).Attributes().FromRaw(rawOrdered)
					}
				case pmetric.MetricTypeEmpty:
				}
			}
		}
	}
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
			switch metrics.At(i).Type() {
			case pmetric.MetricTypeHistogram:
				dps := metrics.At(i).Histogram().DataPoints()
				maskHistogramSliceAttributeValues(dps, attributeName)

				// If attribute values are ignored, some data points may become
				// indistinguishable from each other, but sorting by value allows
				// for a reasonably thorough comparison and a deterministic outcome.
				dps.Sort(func(a, b pmetric.HistogramDataPoint) bool {
					if a.Sum() < b.Sum() {
						return true
					}
					if a.Min() < b.Min() {
						return true
					}
					if a.Max() < b.Max() {
						return true
					}
					if a.Count() < b.Count() {
						return true
					}
					if a.BucketCounts().Len() < b.BucketCounts().Len() {
						return true
					}
					if a.ExplicitBounds().Len() < b.ExplicitBounds().Len() {
						return true
					}
					return false
				})
			default:
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
			case pcommon.ValueTypeBool:
				attribute.SetBool(false)
			case pcommon.ValueTypeInt:
				attribute.SetInt(0)
			case pcommon.ValueTypeEmpty, pcommon.ValueTypeDouble, pcommon.ValueTypeMap, pcommon.ValueTypeSlice, pcommon.ValueTypeBytes:
				fallthrough
			default:
				panic(fmt.Sprintf("data type not supported: %s", attribute.Type()))
			}
		}
	}
}

// maskHistogramSliceAttributeValues sets the value of the specified attribute to
// the zero value associated with the attribute data type.
func maskHistogramSliceAttributeValues(dataPoints pmetric.HistogramDataPointSlice, attributeName string) {
	for i := 0; i < dataPoints.Len(); i++ {
		attributes := dataPoints.At(i).Attributes()
		attribute, ok := attributes.Get(attributeName)
		if ok {
			switch attribute.Type() {
			case pcommon.ValueTypeStr:
				attribute.SetStr("")
			case pcommon.ValueTypeBool:
				attribute.SetBool(false)
			case pcommon.ValueTypeInt:
				attribute.SetInt(0)
			case pcommon.ValueTypeEmpty, pcommon.ValueTypeDouble, pcommon.ValueTypeMap, pcommon.ValueTypeSlice, pcommon.ValueTypeBytes:
				fallthrough
			default:
				panic(fmt.Sprintf("data type not supported: %s", attribute.Type()))
			}
		}
	}
}

// MatchMetricAttributeValue is a CompareMetricsOption that transforms a metric attribute value based on a regular expression.
func MatchMetricAttributeValue(attributeName string, pattern string, metricNames ...string) CompareMetricsOption {
	re := regexp.MustCompile(pattern)
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		matchMetricAttributeValue(expected, attributeName, re, metricNames)
		matchMetricAttributeValue(actual, attributeName, re, metricNames)
	})
}

func matchMetricAttributeValue(metrics pmetric.Metrics, attributeName string, re *regexp.Regexp, metricNames []string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			matchMetricSliceAttributeValues(ilms.At(j).Metrics(), attributeName, re, metricNames)
		}
	}
}

func matchMetricSliceAttributeValues(metrics pmetric.MetricSlice, attributeName string, re *regexp.Regexp, metricNames []string) {
	metricNameSet := make(map[string]bool, len(metricNames))
	for _, metricName := range metricNames {
		metricNameSet[metricName] = true
	}

	for i := 0; i < metrics.Len(); i++ {
		if len(metricNames) == 0 || metricNameSet[metrics.At(i).Name()] {
			switch metrics.At(i).Type() {
			case pmetric.MetricTypeHistogram:
				dps := metrics.At(i).Histogram().DataPoints()
				matchHistogramDataPointSliceAttributeValues(dps, attributeName, re)

				// If attribute values are ignored, some data points may become
				// indistinguishable from each other, but sorting by value allows
				// for a reasonably thorough comparison and a deterministic outcome.
				dps.Sort(func(a, b pmetric.HistogramDataPoint) bool {
					if a.Sum() < b.Sum() {
						return true
					}
					if a.Min() < b.Min() {
						return true
					}
					if a.Max() < b.Max() {
						return true
					}
					if a.Count() < b.Count() {
						return true
					}
					if a.BucketCounts().Len() < b.BucketCounts().Len() {
						return true
					}
					if a.ExplicitBounds().Len() < b.ExplicitBounds().Len() {
						return true
					}
					return false
				})
			default:
				dps := getDataPointSlice(metrics.At(i))
				matchDataPointSliceAttributeValues(dps, attributeName, re)

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
}

func matchDataPointSliceAttributeValues(dataPoints pmetric.NumberDataPointSlice, attributeName string, re *regexp.Regexp) {
	for i := 0; i < dataPoints.Len(); i++ {
		attributes := dataPoints.At(i).Attributes()
		attribute, ok := attributes.Get(attributeName)
		if ok {
			results := re.FindStringSubmatch(attribute.Str())
			if len(results) > 0 {
				attribute.SetStr(results[0])
			}
		}
	}
}

func matchHistogramDataPointSliceAttributeValues(dataPoints pmetric.HistogramDataPointSlice, attributeName string, re *regexp.Regexp) {
	for i := 0; i < dataPoints.Len(); i++ {
		attributes := dataPoints.At(i).Attributes()
		attribute, ok := attributes.Get(attributeName)
		if ok {
			results := re.FindStringSubmatch(attribute.Str())
			if len(results) > 0 {
				attribute.SetStr(results[0])
			}
		}
	}
}

// MatchResourceAttributeValue is a CompareMetricsOption that transforms a resource attribute value based on a regular expression.
func MatchResourceAttributeValue(attributeName string, pattern string) CompareMetricsOption {
	re := regexp.MustCompile(pattern)
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		matchResourceAttributeValue(expected, attributeName, re)
		matchResourceAttributeValue(actual, attributeName, re)
	})
}

func matchResourceAttributeValue(metrics pmetric.Metrics, attributeName string, re *regexp.Regexp) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		internal.MatchResourceAttributeValue(rms.At(i).Resource(), attributeName, re)
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

func ChangeResourceAttributeValue(attributeName string, changeFn func(string) string) CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		changeMetricsResourceAttributeValue(expected, attributeName, changeFn)
		changeMetricsResourceAttributeValue(actual, attributeName, changeFn)
	})
}

func changeMetricsResourceAttributeValue(metrics pmetric.Metrics, attributeName string, changeFn func(string) string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		internal.ChangeResourceAttributeValue(rms.At(i).Resource(), attributeName, changeFn)
	}
}

// ChangeDatapointAttributeValue changes the metric datapoint value with the specified key
func ChangeDatapointAttributeValue(attributeName string, changeFn func(string) string) CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		changeMetricsDatapointAttributeValue(expected, attributeName, changeFn)
		changeMetricsDatapointAttributeValue(actual, attributeName, changeFn)
	})
}

func changeMetricsDatapointAttributeValue(metrics pmetric.Metrics, attributeName string, changeFn func(string) string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		internal.ChangeResourceAttributeValue(rms.At(i).Resource(), attributeName, changeFn)
	}

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		for j := 0; j < metrics.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for g := 0; g < metrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); g++ {
				m := metrics.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(g)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					datapoints := m.Gauge().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						if v, ok := datapoints.At(k).Attributes().Get(attributeName); ok {
							datapoints.At(k).Attributes().PutStr(attributeName, changeFn(v.Str()))
						}
					}
				case pmetric.MetricTypeSum:
					datapoints := m.Sum().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						if v, ok := datapoints.At(k).Attributes().Get(attributeName); ok {
							datapoints.At(k).Attributes().PutStr(attributeName, changeFn(v.Str()))
						}
					}
				case pmetric.MetricTypeHistogram:
					datapoints := m.Histogram().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						if v, ok := datapoints.At(k).Attributes().Get(attributeName); ok {
							datapoints.At(k).Attributes().PutStr(attributeName, changeFn(v.Str()))
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					datapoints := m.ExponentialHistogram().DataPoints()
					for k := 0; k < datapoints.Len(); k++ {
						if v, ok := datapoints.At(k).Attributes().Get(attributeName); ok {
							datapoints.At(k).Attributes().PutStr(attributeName, changeFn(v.Str()))
						}
					}
				}
			}
		}
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
					switch ms.At(k).Type() {
					case pmetric.MetricTypeHistogram:
						dps := ms.At(k).Histogram().DataPoints()
						n := 0
						dps.RemoveIf(func(pmetric.HistogramDataPoint) bool {
							n++
							return n > 1
						})
					default:
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

func IgnoreScopeVersion() CompareMetricsOption {
	return compareMetricsOptionFunc(func(expected, actual pmetric.Metrics) {
		maskScopeVersion(expected)
		maskScopeVersion(actual)
	})
}

func maskScopeVersion(metrics pmetric.Metrics) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			sm.Scope().SetVersion("")
		}
	}
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
				//exhaustive:enforce
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
				case pmetric.MetricTypeEmpty:
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
