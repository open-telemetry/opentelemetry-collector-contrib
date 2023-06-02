// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// sorts all Resource Metrics attributes and Datapoint Slice metric attributes
func sortMetrics(ms pmetric.Metrics) {
	rms := ms.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sortAttributeMap(rms.At(i).Resource().Attributes())
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			sortAttributeMap(ilms.At(j).Scope().Attributes())
			metricsList := ilms.At(j).Metrics()
			for k := 0; k < metricsList.Len(); k++ {
				metric := metricsList.At(k)
				switch metricsList.At(k).Type() {
				case pmetric.MetricTypeGauge:
					ds := metric.Gauge().DataPoints()
					for l := 0; l < ds.Len(); l++ {
						sortAttributeMap(ds.At(l).Attributes())
					}
				case pmetric.MetricTypeSum:
					ds := metric.Sum().DataPoints()
					for l := 0; l < ds.Len(); l++ {
						sortAttributeMap(ds.At(l).Attributes())
					}
				case pmetric.MetricTypeHistogram:
					ds := metric.Histogram().DataPoints()
					for l := 0; l < ds.Len(); l++ {
						sortAttributeMap(ds.At(l).Attributes())
					}
				case pmetric.MetricTypeExponentialHistogram:
					ds := metric.ExponentialHistogram().DataPoints()
					for l := 0; l < ds.Len(); l++ {
						sortAttributeMap(ds.At(l).Attributes())
					}
				case pmetric.MetricTypeSummary:
					ds := metric.Summary().DataPoints()
					for l := 0; l < ds.Len(); l++ {
						sortAttributeMap(ds.At(l).Attributes())
					}
				}
			}
		}
	}
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
	fmt.Println("wrote it aahahahahahahahha")
}

func sortNumberDataPointSlice(ndps pmetric.NumberDataPointSlice) {
	ndps.Sort(func(a, b pmetric.NumberDataPoint) bool {
		aTime := a.Timestamp().String()
		bTime := b.Timestamp().String()
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		if bytes.Compare(aAttrs[:], bAttrs[:]) == 0 {
			return compareTimestamp(aTime, bTime)
		}
		return compareTimestamp(aTime, bTime)
	})
}

func sortSummaryDataPointSlice(sdps pmetric.SummaryDataPointSlice) {
	sdps.Sort(func(a, b pmetric.SummaryDataPoint) bool {
		aTime := a.Timestamp().String()
		bTime := b.Timestamp().String()
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		if bytes.Compare(aAttrs[:], bAttrs[:]) == 0 {
			return compareTimestamp(aTime, bTime)
		}
		return compareTimestamp(aTime, bTime)
	})
}

func sortHistogramDataPointSlice(hdps pmetric.HistogramDataPointSlice) {
	hdps.Sort(func(a, b pmetric.HistogramDataPoint) bool {
		aTime := a.Timestamp().String()
		bTime := b.Timestamp().String()
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		if bytes.Compare(aAttrs[:], bAttrs[:]) == 0 {
			return compareTimestamp(aTime, bTime)
		}
		return compareTimestamp(aTime, bTime)
	})
}

func sortExponentialHistogramDataPointSlice(ehdps pmetric.ExponentialHistogramDataPointSlice) {
	ehdps.Sort(func(a, b pmetric.ExponentialHistogramDataPoint) bool {
		aTime := a.Timestamp().String()
		bTime := b.Timestamp().String()
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		if bytes.Compare(aAttrs[:], bAttrs[:]) == 0 {
			return compareTimestamp(aTime, bTime)
		}
		return compareTimestamp(aTime, bTime)
	})
}

func compareTimestamp(a, b string) bool {
	lenA, lenB := len(a), len(b)

	if lenA != lenB {
		return lenA < lenB
	}

	for i := 0; i < lenA; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}

	return false
}
