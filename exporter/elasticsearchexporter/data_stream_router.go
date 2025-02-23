// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
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

// documentRouter is an interface for routing records to the appropriate
// index or data stream. The router may mutate record attributes.
type documentRouter interface {
	routeLogRecord(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error)
	routeDataPoint(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error)
	routeSpan(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error)
	routeSpanEvent(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error)
}

func newDocumentRouter(mode MappingMode, dynamicIndex bool, defaultIndex string, cfg *Config) documentRouter {
	var router documentRouter
	if dynamicIndex {
		router = dynamicDocumentRouter{
			index: elasticsearch.Index{Index: defaultIndex},
			mode:  mode,
		}
	} else {
		router = staticDocumentRouter{
			index: elasticsearch.Index{Index: defaultIndex},
		}
	}
	if cfg.LogstashFormat.Enabled {
		router = logstashDocumentRouter{inner: router, logstashFormat: cfg.LogstashFormat}
	}
	return router
}

type staticDocumentRouter struct {
	index elasticsearch.Index
}

func (r staticDocumentRouter) routeLogRecord(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return r.route(resource, scope, recordAttrs)
}

func (r staticDocumentRouter) routeDataPoint(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return r.route(resource, scope, recordAttrs)
}

func (r staticDocumentRouter) routeSpan(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return r.route(resource, scope, recordAttrs)
}

func (r staticDocumentRouter) routeSpanEvent(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return r.route(resource, scope, recordAttrs)
}

func (r staticDocumentRouter) route(_ pcommon.Resource, _ pcommon.InstrumentationScope, _ pcommon.Map) (elasticsearch.Index, error) {
	return r.index, nil
}

type dynamicDocumentRouter struct {
	index elasticsearch.Index
	mode  MappingMode
}

func (r dynamicDocumentRouter) routeLogRecord(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return routeRecord(resource, scope, recordAttrs, r.index.Index, r.mode, defaultDataStreamTypeLogs)
}

func (r dynamicDocumentRouter) routeDataPoint(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return routeRecord(resource, scope, recordAttrs, r.index.Index, r.mode, defaultDataStreamTypeMetrics)
}

func (r dynamicDocumentRouter) routeSpan(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return routeRecord(resource, scope, recordAttrs, r.index.Index, r.mode, defaultDataStreamTypeTraces)
}

func (r dynamicDocumentRouter) routeSpanEvent(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return routeRecord(resource, scope, recordAttrs, r.index.Index, r.mode, defaultDataStreamTypeLogs)
}

type logstashDocumentRouter struct {
	inner          documentRouter
	logstashFormat LogstashFormatSettings
}

func (r logstashDocumentRouter) routeLogRecord(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return r.route(r.inner.routeLogRecord(resource, scope, recordAttrs))
}

func (r logstashDocumentRouter) routeDataPoint(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return r.route(r.inner.routeDataPoint(resource, scope, recordAttrs))
}

func (r logstashDocumentRouter) routeSpan(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return r.route(r.inner.routeSpan(resource, scope, recordAttrs))
}

func (r logstashDocumentRouter) routeSpanEvent(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return r.route(r.inner.routeSpanEvent(resource, scope, recordAttrs))
}

func (r logstashDocumentRouter) route(index elasticsearch.Index, err error) (elasticsearch.Index, error) {
	if err != nil {
		return elasticsearch.Index{}, err
	}
	formattedIndex, err := generateIndexWithLogstashFormat(index.Index, &r.logstashFormat, time.Now())
	if err != nil {
		return elasticsearch.Index{}, err
	}
	return elasticsearch.Index{Index: formattedIndex}, nil
}

func routeRecord(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	recordAttr pcommon.Map,
	index string,
	mode MappingMode,
	defaultDSType string,
) (elasticsearch.Index, error) {
	resourceAttr := resource.Attributes()
	scopeAttr := scope.Attributes()

	// Order:
	// 1. read data_stream.* from attributes
	// 2. read elasticsearch.index.* from attributes
	// 3. receiver-based routing
	// 4. use default hardcoded data_stream.*
	dataset, datasetExists := getFromAttributes(elasticsearch.DataStreamDataset, defaultDataStreamDataset, recordAttr, scopeAttr, resourceAttr)
	namespace, namespaceExists := getFromAttributes(elasticsearch.DataStreamNamespace, defaultDataStreamNamespace, recordAttr, scopeAttr, resourceAttr)

	dsType := defaultDSType
	// if mapping mode is bodymap, allow overriding data_stream.type
	if mode == MappingBodyMap {
		dsType, _ = getFromAttributes(elasticsearch.DataStreamType, defaultDSType, recordAttr, scopeAttr, resourceAttr)
		if dsType != "logs" && dsType != "metrics" {
			return elasticsearch.Index{}, fmt.Errorf("data_stream.type cannot be other than logs or metrics")
		}
	}

	dataStreamMode := datasetExists || namespaceExists
	if !dataStreamMode {
		prefix, prefixExists := getFromAttributes(indexPrefix, "", resourceAttr, scopeAttr, recordAttr)
		suffix, suffixExists := getFromAttributes(indexSuffix, "", resourceAttr, scopeAttr, recordAttr)
		if prefixExists || suffixExists {
			return elasticsearch.Index{Index: fmt.Sprintf("%s%s%s", prefix, index, suffix)}, nil
		}
	}

	// Receiver-based routing
	// For example, hostmetricsreceiver (or hostmetricsreceiver.otel in the OTel output mode)
	// for the scope name
	// github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper
	if submatch := receiverRegex.FindStringSubmatch(scope.Name()); len(submatch) > 0 {
		receiverName := submatch[1]
		dataset = receiverName
	}

	// For dataset, the naming convention for datastream is expected to be "logs-[dataset].otel-[namespace]".
	// This is in order to match the built-in logs-*.otel-* index template.
	var datasetSuffix string
	if mode == MappingOTel {
		datasetSuffix += ".otel"
	}

	dataset = sanitizeDataStreamField(dataset, disallowedDatasetRunes, datasetSuffix)
	namespace = sanitizeDataStreamField(namespace, disallowedNamespaceRunes, "")
	return elasticsearch.NewDataStreamIndex(dsType, dataset, namespace), nil
}
