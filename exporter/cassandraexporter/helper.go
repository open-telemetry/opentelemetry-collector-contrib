// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import "encoding/json"

func attributesToMap(attributes map[string]any) map[string]string {
	newAttrMap := make(map[string]string)
	for k, v := range attributes {
		jsonV, err := json.Marshal(v)
		if err == nil {
			newAttrMap[k] = string(jsonV)
		}
	}
	return newAttrMap
}
