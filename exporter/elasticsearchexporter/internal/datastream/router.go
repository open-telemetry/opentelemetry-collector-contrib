// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datastream // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/ecs/internal/datastream"

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

var receiverRegex = regexp.MustCompile(`/receiver/(\w*receiver)`)

const (
	maxDataStreamBytes       = 100
	disallowedNamespaceRunes = "\\/*?\"<>| ,#:"
	disallowedDatasetRunes   = "-\\/*?\"<>| ,#:"

	IndexPrefix = "elasticsearch.index.prefix"
	IndexSuffix = "elasticsearch.index.suffix"
)

var (
	// RouteLogRecord returns the name of the index to send the log record to according to data stream routing related attributes.
	// This function may mutate record attributes.
	RouteLogRecord = routeWithDefaults(DefaultDataStreamTypeLogs)

	// RouteDataPoint returns the name of the index to send the data point to according to data stream routing related attributes.
	// This function may mutate record attributes.
	RouteDataPoint = routeWithDefaults(DefaultDataStreamTypeMetrics)

	// RouteSpan returns the name of the index to send the span to according to data stream routing related attributes.
	// This function may mutate record attributes.
	RouteSpan = routeWithDefaults(DefaultDataStreamTypeTraces)

	// RouteSpanEvent returns the name of the index to send the span event to according to data stream routing related attributes.
	// This function may mutate record attributes.
	RouteSpanEvent = routeWithDefaults(DefaultDataStreamTypeLogs)
)

// Sanitize the datastream fields (dataset, namespace) to apply restrictions
// as outlined in https://www.elastic.co/guide/en/ecs/current/ecs-data_stream.html
// The suffix will be appended after truncation of max bytes.
func sanitizeDataStreamField(field, disallowed, appendSuffix string) string {
	field = strings.Map(func(r rune) rune {
		if strings.ContainsRune(disallowed, r) {
			return '_'
		}
		return unicode.ToLower(r)
	}, field)

	if len(field) > maxDataStreamBytes-len(appendSuffix) {
		field = field[:maxDataStreamBytes-len(appendSuffix)]
	}
	field += appendSuffix

	return field
}

func routeWithDefaults(defaultDSType string) func(
	pcommon.Map,
	pcommon.Map,
	pcommon.Map,
	string,
	bool,
	string,
) string {
	return func(
		recordAttr pcommon.Map,
		scopeAttr pcommon.Map,
		resourceAttr pcommon.Map,
		fIndex string,
		otel bool,
		scopeName string,
	) string {
		// Order:
		// 1. read data_stream.* from attributes
		// 2. read elasticsearch.index.* from attributes
		// 3. receiver-based routing
		// 4. use default hardcoded data_stream.*
		dataset, datasetExists := getFromAttributes(DataStreamDataset, DefaultDataStreamDataset, recordAttr, scopeAttr, resourceAttr)
		namespace, namespaceExists := getFromAttributes(DataStreamNamespace, DefaultDataStreamNamespace, recordAttr, scopeAttr, resourceAttr)
		dataStreamMode := datasetExists || namespaceExists
		if !dataStreamMode {
			prefix, prefixExists := getFromAttributes(IndexPrefix, "", resourceAttr, scopeAttr, recordAttr)
			suffix, suffixExists := getFromAttributes(IndexSuffix, "", resourceAttr, scopeAttr, recordAttr)
			if prefixExists || suffixExists {
				return fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
			}
		}

		// Receiver-based routing
		// For example, hostmetricsreceiver (or hostmetricsreceiver.otel in the OTel output mode)
		// for the scope name
		// github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper
		if submatch := receiverRegex.FindStringSubmatch(scopeName); len(submatch) > 0 {
			receiverName := submatch[1]
			dataset = receiverName
		}

		// For dataset, the naming convention for datastream is expected to be "logs-[dataset].otel-[namespace]".
		// This is in order to match the built-in logs-*.otel-* index template.
		var datasetSuffix string
		if otel {
			datasetSuffix += ".otel"
		}

		dataset = sanitizeDataStreamField(dataset, disallowedDatasetRunes, datasetSuffix)
		namespace = sanitizeDataStreamField(namespace, disallowedNamespaceRunes, "")

		recordAttr.PutStr(DataStreamDataset, dataset)
		recordAttr.PutStr(DataStreamNamespace, namespace)
		recordAttr.PutStr(DataStreamType, defaultDSType)

		return fmt.Sprintf("%s-%s-%s", defaultDSType, dataset, namespace)
	}
}

func getFromAttributes(name string, defaultValue string, attributeMaps ...pcommon.Map) (string, bool) {
	for _, attributeMap := range attributeMaps {
		if value, exists := attributeMap.Get(name); exists {
			return value.AsString(), true
		}
	}
	return defaultValue, false
}
