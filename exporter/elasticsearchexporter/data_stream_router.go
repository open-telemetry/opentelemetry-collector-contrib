// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

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
) esIndex {
	return func(
		recordAttr pcommon.Map,
		scopeAttr pcommon.Map,
		resourceAttr pcommon.Map,
		fIndex string,
		otel bool,
		scopeName string,
	) esIndex {
		// Order:
		// 1. read data_stream.* from attributes
		// 2. read elasticsearch.index.* from attributes
		// 3. receiver-based routing
		// 4. use default hardcoded data_stream.*
		dataset, datasetExists := getFromAttributes(dataStreamDataset, defaultDataStreamDataset, recordAttr, scopeAttr, resourceAttr)
		namespace, namespaceExists := getFromAttributes(dataStreamNamespace, defaultDataStreamNamespace, recordAttr, scopeAttr, resourceAttr)
		dataStreamMode := datasetExists || namespaceExists
		if !dataStreamMode {
			prefix, prefixExists := getFromAttributes(indexPrefix, "", resourceAttr, scopeAttr, recordAttr)
			suffix, suffixExists := getFromAttributes(indexSuffix, "", resourceAttr, scopeAttr, recordAttr)
			if prefixExists || suffixExists {
				return esIndex{Index: fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)}
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
		return newDataStream(defaultDSType, dataset, namespace)
	}
}

type esIndex struct {
	Index     string
	Type      string
	Dataset   string
	Namespace string
}

func newDataStream(typ, dataset, namespace string) esIndex {
	return esIndex{
		Index:     fmt.Sprintf("%s-%s-%s", typ, dataset, namespace),
		Type:      typ,
		Dataset:   dataset,
		Namespace: namespace,
	}
}

func (i esIndex) isDataStream() bool {
	return i.Type != "" && i.Dataset != "" && i.Namespace != ""
}

var (
	// routeLogRecord returns the name of the index to send the log record to according to data stream routing related attributes.
	// This function may mutate record attributes.
	routeLogRecord = routeWithDefaults(defaultDataStreamTypeLogs)

	// routeDataPoint returns the name of the index to send the data point to according to data stream routing related attributes.
	// This function may mutate record attributes.
	routeDataPoint = routeWithDefaults(defaultDataStreamTypeMetrics)

	// routeSpan returns the name of the index to send the span to according to data stream routing related attributes.
	// This function may mutate record attributes.
	routeSpan = routeWithDefaults(defaultDataStreamTypeTraces)

	// routeSpanEvent returns the name of the index to send the span event to according to data stream routing related attributes.
	// This function may mutate record attributes.
	routeSpanEvent = routeWithDefaults(defaultDataStreamTypeLogs)
)
