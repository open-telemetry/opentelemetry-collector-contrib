// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"

import (
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// sorts all Resource Metrics attributes and Datapoint Slice metric attributes and all Resource, Scope, and Datapoint Slices
func sortMetrics(ms pmetric.Metrics) {
	rms := ms.ResourceMetrics()
	for i := range rms.Len() {
		sortAttributeMap(rms.At(i).Resource().Attributes())
		ilms := rms.At(i).ScopeMetrics()
		for j := range ilms.Len() {
			sortAttributeMap(ilms.At(j).Scope().Attributes())
			metricsList := ilms.At(j).Metrics()
			for k := range metricsList.Len() {
				metric := metricsList.At(k)
				//exhaustive:enforce
				switch metricsList.At(k).Type() {
				case pmetric.MetricTypeGauge:
					ds := metric.Gauge().DataPoints()
					for l := range ds.Len() {
						sortAttributeMap(ds.At(l).Attributes())
					}
				case pmetric.MetricTypeSum:
					ds := metric.Sum().DataPoints()
					for l := range ds.Len() {
						sortAttributeMap(ds.At(l).Attributes())
					}
				case pmetric.MetricTypeHistogram:
					ds := metric.Histogram().DataPoints()
					for l := range ds.Len() {
						sortAttributeMap(ds.At(l).Attributes())
					}
				case pmetric.MetricTypeExponentialHistogram:
					ds := metric.ExponentialHistogram().DataPoints()
					for l := range ds.Len() {
						sortAttributeMap(ds.At(l).Attributes())
					}
				case pmetric.MetricTypeSummary:
					ds := metric.Summary().DataPoints()
					for l := range ds.Len() {
						sortAttributeMap(ds.At(l).Attributes())
					}
				}
			}
		}
	}

	sortResources(ms)
	sortScopes(ms)
	sortMetricDataPointSlices(ms)
}

// sortAttributeMap sorts the attributes of a pcommon.Map according to the alphanumeric ordering of the keys
func sortAttributeMap(mp pcommon.Map) {
	tempMap := pcommon.NewMap()
	keys := []string{}
	mp.Range(func(key string, _ pcommon.Value) bool {
		keys = append(keys, key)
		return true
	})
	sort.Strings(keys)
	for _, k := range keys {
		value, exists := mp.Get(k)
		if exists {
			switch value.Type() {
			case pcommon.ValueTypeStr:
				tempMap.PutStr(k, value.Str())
			case pcommon.ValueTypeBool:
				tempMap.PutBool(k, value.Bool())
			case pcommon.ValueTypeInt:
				tempMap.PutInt(k, value.Int())
			case pcommon.ValueTypeDouble:
				tempMap.PutDouble(k, value.Double())
			case pcommon.ValueTypeMap:
				value.Map().CopyTo(tempMap.PutEmptyMap(k))
			case pcommon.ValueTypeSlice:
				value.Slice().CopyTo(tempMap.PutEmptySlice(k))
			case pcommon.ValueTypeBytes:
				value.Bytes().CopyTo(tempMap.PutEmptyBytes(k))
			}
		}
	}
	tempMap.CopyTo(mp)
}

// sortMetricDataPointSlices sorts the datapoint slice of a pmetric.Metrics according to the alphanumeric ordering of map key
func sortMetricDataPointSlices(ms pmetric.Metrics) {
	for i := range ms.ResourceMetrics().Len() {
		for j := range ms.ResourceMetrics().At(i).ScopeMetrics().Len() {
			for k := range ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len() {
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
				}
			}
		}
	}
}

func sortResources(ms pmetric.Metrics) {
	ms.ResourceMetrics().Sort(func(a, b pmetric.ResourceMetrics) bool {
		return compareMaps(a.Resource().Attributes(), b.Resource().Attributes()) < 0
	})
}

func sortScopes(ms pmetric.Metrics) {
	for i := range ms.ResourceMetrics().Len() {
		rm := ms.ResourceMetrics().At(i)
		rm.ScopeMetrics().Sort(func(a, b pmetric.ScopeMetrics) bool {
			return compareMaps(a.Scope().Attributes(), b.Scope().Attributes()) < 0
		})
	}
}

func sortNumberDataPointSlice(ndps pmetric.NumberDataPointSlice) {
	ndps.Sort(func(a, b pmetric.NumberDataPoint) bool {
		return compareMaps(a.Attributes(), b.Attributes()) < 0
	})
}

func sortSummaryDataPointSlice(sdps pmetric.SummaryDataPointSlice) {
	sdps.Sort(func(a, b pmetric.SummaryDataPoint) bool {
		return compareMaps(a.Attributes(), b.Attributes()) < 0
	})
}

func sortHistogramDataPointSlice(hdps pmetric.HistogramDataPointSlice) {
	hdps.Sort(func(a, b pmetric.HistogramDataPoint) bool {
		return compareMaps(a.Attributes(), b.Attributes()) < 0
	})
}

func sortExponentialHistogramDataPointSlice(ehdps pmetric.ExponentialHistogramDataPointSlice) {
	ehdps.Sort(func(a, b pmetric.ExponentialHistogramDataPoint) bool {
		return compareMaps(a.Attributes(), b.Attributes()) < 0
	})
}

func compareMaps(a, b pcommon.Map) int {
	sortAttributeMap(a)
	sortAttributeMap(b)

	if a.Len() != b.Len() {
		return a.Len() - b.Len()
	}

	var aKeys, bKeys []string
	a.Range(func(k string, v pcommon.Value) bool {
		aKeys = append(aKeys, fmt.Sprintf("%s: %v", k, v.AsString()))
		return true
	})
	b.Range(func(k string, v pcommon.Value) bool {
		bKeys = append(bKeys, fmt.Sprintf("%s: %v", k, v.AsString()))
		return true
	})

	for i := range aKeys {
		if aKeys[i] != bKeys[i] {
			return strings.Compare(aKeys[i], bKeys[i])
		}
	}

	return 0
}
