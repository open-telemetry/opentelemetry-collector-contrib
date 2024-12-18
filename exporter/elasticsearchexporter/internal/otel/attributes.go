// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otel // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otel"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

func EncodeAttributes(document *objmodel.Document, attributeMap pcommon.Map) {
	attrsCopy := pcommon.NewMap() // Copy to avoid mutating original map
	attributeMap.CopyTo(attrsCopy)
	attrsCopy.RemoveIf(func(key string, val pcommon.Value) bool {
		switch key {
		case dataStreamType, dataStreamDataset, dataStreamNamespace:
			// At this point the data_stream attributes are expected to be in the record attributes,
			// updated by the router.
			// Move them to the top of the document and remove them from the record
			document.AddAttribute(key, val)
			return true
		case mappingHintsAttrKey:
			return true
		}
		return false
	})
	mergeGeolocation(attrsCopy)
	document.AddAttributes("attributes", attrsCopy)
}
