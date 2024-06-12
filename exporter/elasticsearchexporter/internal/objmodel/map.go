// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Map processes a pcommon.Map into key value pairs and adds them to Elasticsearch
// document. Only map types are recursively processed. Map also allows remapping
// keys by passing in a key remapper. Any key remapped via the key remapper to
// an empty string is not added to the resulting document.
type Map struct {
	pcommon.Map

	keyRemapper func(string) string
}

var emptyRemapper = func(k string) string {
	return k
}

// NewMapProcessorValue is a utility function to create a processor value from
// map processor. If the map is empty then it returns a NilValue.
func NewMapProcessorValue(m pcommon.Map, remapper func(string) string) Value {
	if m.Len() == 0 {
		return NilValue
	}
	return ProcessorValue(NewMapProcessor(m, remapper))
}

// NewMapProcessor creates a new processor of processing pcommon.Map.
func NewMapProcessor(m pcommon.Map, remapper func(string) string) Map {
	if remapper == nil {
		remapper = emptyRemapper
	}
	return Map{Map: m, keyRemapper: remapper}
}

// Len gives the number of entries that will be added to the Document. This
// is an approximate figure as it doesn't count for entries removed via remapper.
func (m Map) Len() int {
	return lenMap(m.Map)
}

// Process iterates over the map and adds the required fields into the document.
// The keys could be remapped to another key as per the remapper function.
func (m Map) Process(doc *Document, key string) {
	processMap(m.Map, m.keyRemapper, doc, key)
}

func lenMap(m pcommon.Map) int {
	var count int
	m.Range(func(_ string, v pcommon.Value) bool {
		switch v.Type() {
		case pcommon.ValueTypeEmpty:
		// Only maps are expanded in the document
		case pcommon.ValueTypeMap:
			count += lenMap(v.Map())
		default:
			count += 1
		}
		return true
	})
	return count
}

func processMap(
	m pcommon.Map,
	keyRemapper func(string) string,
	doc *Document,
	key string,
) {
	m.Range(func(k string, v pcommon.Value) bool {
		k = keyRemapper(flattenKey(key, k))
		if k == "" {
			// any empty value for a remapped metric key
			// will be skipped
			return true
		}

		switch v.Type() {
		case pcommon.ValueTypeMap:
			processMap(v.Map(), keyRemapper, doc, k)
		default:
			doc.Add(k, ValueFromAttribute(v))
		}
		return true
	})
}
