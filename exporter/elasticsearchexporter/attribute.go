// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import "go.opentelemetry.io/collector/pdata/pcommon"

// dynamic index attribute key constants
const (
	indexPrefix                  = "elasticsearch.index.prefix"
	indexSuffix                  = "elasticsearch.index.suffix"
	dataStreamDataset            = "data_stream.dataset"
	dataStreamNamespace          = "data_stream.namespace"
	dataStreamType               = "data_stream.type"
	defaultDataStreamDataset     = "generic"
	defaultDataStreamNamespace   = "default"
	defaultDataStreamTypeLogs    = "logs"
	defaultDataStreamTypeMetrics = "metrics"
	defaultDataStreamTypeTraces  = "traces"
)

func getFromAttributes(name string, defaultValue string, attributeMaps ...pcommon.Map) (string, bool) {
	for _, attributeMap := range attributeMaps {
		if value, exists := attributeMap.Get(name); exists {
			return value.AsString(), true
		}
	}
	return defaultValue, false
}
