// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal"

import (
	"encoding/json"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

// GetServiceName extracts the service name from resource attributes.
func GetServiceName(resAttr pcommon.Map) string {
	if v, ok := resAttr.Get(string(conventions.ServiceNameKey)); ok {
		return v.AsString()
	}
	return ""
}

// AttributesToJSON converts pcommon.Map attributes to a JSON byte slice.
func AttributesToJSON(attributes pcommon.Map) []byte {
	if attributes.Len() == 0 {
		return nil
	}
	m := make(map[string]string, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
		m[k] = v.AsString()
		return true
	})
	b, _ := json.Marshal(m)
	return b
}
