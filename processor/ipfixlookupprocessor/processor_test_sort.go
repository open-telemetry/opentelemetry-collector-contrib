// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor

import (
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Question: Is there a better way of doing this ?
func sortTraceAttributes(traces ptrace.Traces) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		resourceSpan := traces.ResourceSpans().At(i)
		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)
			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				attributes := span.Attributes()
				sortedAttributes := sortAttributeMap(attributes)
				sortedAttributes.CopyTo(attributes)
			}
		}
	}
}

// derviced from https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/pkg/golden/v0.89.0/pkg/golden/sort_metrics.go
// sortAttributeMap sorts the attributes of a pcommon.Map according to the alphanumeric ordering of the keys
func sortAttributeMap(mp pcommon.Map) pcommon.Map {
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
			case pcommon.ValueTypeMap:
				sortedMap := sortAttributeMap(value.Map())
				sortedMap.CopyTo(tempMap.PutEmptyMap(k))
			case pcommon.ValueTypeSlice:
				sortedSlice := sortAttributeSlice(value.Slice())
				sortedSlice.CopyTo(tempMap.PutEmptySlice(k))
			default:
				value.CopyTo(tempMap.PutEmpty(k))
			}
		}
	}
	return tempMap
}

func sortAttributeSlice(slice pcommon.Slice) pcommon.Slice {
	tempSlice := pcommon.NewSlice()
	for i := 0; i < slice.Len(); i++ {
		value := slice.At(i)
		switch value.Type() {
		case pcommon.ValueTypeMap:
			sortedMap := sortAttributeMap(value.Map())
			sortedMap.CopyTo(tempSlice.AppendEmpty().SetEmptyMap())
		case pcommon.ValueTypeSlice:
			sortedSlice := sortAttributeSlice(value.Slice())
			sortedSlice.CopyTo(tempSlice.AppendEmpty().SetEmptySlice())
		default:
			value.CopyTo(tempSlice.AppendEmpty())
		}
	}
	return tempSlice
}
