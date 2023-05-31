// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"

import (
	"sort"

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
