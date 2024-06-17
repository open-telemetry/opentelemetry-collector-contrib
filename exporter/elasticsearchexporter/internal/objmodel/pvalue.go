// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// pValue processes a pcommon.Value into key value pairs and adds them to
// the Elasticsearch document. Only map values are processed recursively.
type pValue struct {
	pcommon.Value

	// Cache map to prevent recreation of a new map if value type is map
	m pMap
}

// NewPValueProcessorValue is a utility function to create a processor value
// from pValue processor. If the value is empty it returns NilValue.
func NewPValueProcessorValue(v pcommon.Value) Value {
	if v.Type() == pcommon.ValueTypeEmpty {
		return NilValue
	}
	return ProcessorValue(newPValueProcessor(v))
}

// newPValueProcessor creates a new processor for processing pcommon.Value.
func newPValueProcessor(v pcommon.Value) pValue {
	pv := pValue{Value: v}
	if v.Type() == pcommon.ValueTypeMap {
		pv.m = newMapProcessor(v.Map(), nil)
	}
	return pv
}

// Len gives the number of entries that will be added to the Elasticsearch document.
func (pv pValue) Len() int {
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
func (pv pValue) Process(doc *Document, key string) {
	// Map is flattened, everything else is encoded as a single field
	switch pv.Type() {
	case pcommon.ValueTypeEmpty:
		return
	case pcommon.ValueTypeMap:
		pv.m.Process(doc, key)
		return
	}
	doc.fields = append(doc.fields, NewKV(key, ValueFromAttribute(pv.Value)))
}
