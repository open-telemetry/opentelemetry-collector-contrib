// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import "go.opentelemetry.io/collector/pdata/pcommon"

// dynamic index attribute key constants
const (
	defaultDataStreamDataset      = "generic"
	defaultDataStreamNamespace    = "default"
	defaultDataStreamTypeLogs     = "logs"
	defaultDataStreamTypeMetrics  = "metrics"
	defaultDataStreamTypeTraces   = "traces"
	defaultDataStreamTypeProfiles = "profiles"
)

func getFromAttributes(name string, defaultValue string, attributeMaps ...pcommon.Map) (string, bool) {
	for _, attributeMap := range attributeMaps {
		if value, exists := attributeMap.Get(name); exists {
			return value.AsString(), true
		}
	}
	return defaultValue, false
}
