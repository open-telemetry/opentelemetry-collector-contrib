// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otel // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otel"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datastream"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

func EncodeResource(document *objmodel.Document, resource pcommon.Resource, resourceSchemaURL string) {
	resourceMapVal := pcommon.NewValueMap()
	resourceMap := resourceMapVal.Map()
	if resourceSchemaURL != "" {
		resourceMap.PutStr("schema_url", resourceSchemaURL)
	}
	resourceMap.PutInt("dropped_attributes_count", int64(resource.DroppedAttributesCount()))
	resourceAttrMap := resourceMap.PutEmptyMap("attributes")
	resource.Attributes().CopyTo(resourceAttrMap)
	resourceAttrMap.RemoveIf(func(key string, _ pcommon.Value) bool {
		switch key {
		case datastream.DataStreamType, datastream.DataStreamDataset, datastream.DataStreamNamespace:
			return true
		}
		return false
	})
	mergeGeolocation(resourceAttrMap)
	document.Add("resource", objmodel.ValueFromAttribute(resourceMapVal))
}
