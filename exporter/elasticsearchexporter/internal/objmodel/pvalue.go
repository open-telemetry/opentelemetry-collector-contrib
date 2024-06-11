// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// PValue processes a pcommon.Value into key value pairs and adds them to
// the Elasticsearch document. Only map values are processed recursively.
type PValue struct {
	pcommon.Value

	// Cache map to prevent recreation of a new map if value type is map
	m Map
}

// NewPValueProcessor creates a new processor for processing pcommon.Value.
func NewPValueProcessor(v pcommon.Value) PValue {
	pv := PValue{Value: v}
	if v.Type() == pcommon.ValueTypeMap {
		pv.m = NewMapProcessor(v.Map(), nil)
	}
	return pv
}

// Len gives the number of entries that will be added to the Elasticsearch document.
func (pv PValue) Len() int {
	switch pv.Type() {
	case pcommon.ValueTypeEmpty:
		return 0
	case pcommon.ValueTypeMap:
		return pv.m.Len()
	}
	return 1
}

// Process iterates over the value types and adds them to the provided document
// against a given key.
func (pv PValue) Process(doc *Document, key string) {
	// Map is flattened, everything else is encoded as a single field
	switch pv.Type() {
	case pcommon.ValueTypeEmpty:
		return
	case pcommon.ValueTypeMap:
		pv.m.Process(doc, key)
		return
	}
	doc.Add(key, ValueFromAttribute(pv.Value))
}
