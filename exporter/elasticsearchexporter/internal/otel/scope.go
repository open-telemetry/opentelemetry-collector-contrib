// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otel // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otel"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

func EncodeScope(document *objmodel.Document, scope pcommon.InstrumentationScope, scopeSchemaURL string) {
	scopeMapVal := pcommon.NewValueMap()
	scopeMap := scopeMapVal.Map()
	if scope.Name() != "" {
		scopeMap.PutStr("name", scope.Name())
	}
	if scope.Version() != "" {
		scopeMap.PutStr("version", scope.Version())
	}
	if scopeSchemaURL != "" {
		scopeMap.PutStr("schema_url", scopeSchemaURL)
	}
	scopeMap.PutInt("dropped_attributes_count", int64(scope.DroppedAttributesCount()))
	scopeAttrMap := scopeMap.PutEmptyMap("attributes")
	scope.Attributes().CopyTo(scopeAttrMap)
	scopeAttrMap.RemoveIf(func(key string, _ pcommon.Value) bool {
		switch key {
		case dataStreamType, dataStreamDataset, dataStreamNamespace:
			return true
		}
		return false
	})
	mergeGeolocation(scopeAttrMap)
	document.Add("scope", objmodel.ValueFromAttribute(scopeMapVal))
}
