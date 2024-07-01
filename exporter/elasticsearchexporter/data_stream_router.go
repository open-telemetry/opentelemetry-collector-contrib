// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func routeWithDefaults(defaultDSType, defaultDSDataset, defaultDSNamespace string) func(
	pcommon.Map,
	pcommon.Map,
	pcommon.Map,
	string,
) string {
	return func(
		recordAttr pcommon.Map,
		scopeAttr pcommon.Map,
		resourceAttr pcommon.Map,
		fIndex string,
	) string {
		// Order:
		// 1. read data_stream.* from attributes
		// 2. read elasticsearch.index.* from attributes
		// 3. use default hardcoded data_stream.*
		dataset, datasetExists := getFromAttributes(dataStreamDataset, defaultDSDataset, recordAttr, scopeAttr, resourceAttr)
		namespace, namespaceExists := getFromAttributes(dataStreamNamespace, defaultDSNamespace, recordAttr, scopeAttr, resourceAttr)
		dataStreamMode := datasetExists || namespaceExists
		if !dataStreamMode {
			prefix, prefixExists := getFromAttributes(indexPrefix, "", resourceAttr, scopeAttr, recordAttr)
			suffix, suffixExists := getFromAttributes(indexSuffix, "", resourceAttr, scopeAttr, recordAttr)
			if prefixExists || suffixExists {
				return fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
			}
		}
		recordAttr.PutStr(dataStreamDataset, dataset)
		recordAttr.PutStr(dataStreamNamespace, namespace)
		recordAttr.PutStr(dataStreamType, defaultDSType)
		return fmt.Sprintf("%s-%s-%s", defaultDSType, dataset, namespace)
	}
}

// routeLogRecord returns the name of the index to send the log record to according to data stream routing attributes and prefix/suffix attributes.
// This function may mutate record attributes.
func routeLogRecord(
	record plog.LogRecord,
	scope pcommon.InstrumentationScope,
	resource pcommon.Resource,
	fIndex string,
) string {
	route := routeWithDefaults(defaultDataStreamTypeLogs, defaultDataStreamDataset, defaultDataStreamNamespace)
	return route(record.Attributes(), scope.Attributes(), resource.Attributes(), fIndex)
}

// routeDataPoint returns the name of the index to send the data point to according to data stream routing attributes.
// This function may mutate record attributes.
func routeDataPoint(
	dataPoint pmetric.NumberDataPoint,
	scope pcommon.InstrumentationScope,
	resource pcommon.Resource,
	fIndex string,
) string {
	route := routeWithDefaults(defaultDataStreamTypeMetrics, defaultDataStreamDataset, defaultDataStreamNamespace)
	return route(dataPoint.Attributes(), scope.Attributes(), resource.Attributes(), fIndex)
}

// routeSpan returns the name of the index to send the span to according to data stream routing attributes.
// This function may mutate record attributes.
func routeSpan(
	span ptrace.Span,
	scope pcommon.InstrumentationScope,
	resource pcommon.Resource,
	fIndex string,
) string {
	route := routeWithDefaults(defaultDataStreamTypeTraces, defaultDataStreamDataset, defaultDataStreamNamespace)
	return route(span.Attributes(), scope.Attributes(), resource.Attributes(), fIndex)
}
