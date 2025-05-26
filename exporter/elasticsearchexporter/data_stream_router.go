// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"errors"
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

// newDocumentRouter returns a router that routes document based on configured mode, static index config, and config.
func newDocumentRouter(mode MappingMode, staticIndex string, cfg *Config) documentRouter {
	var router documentRouter
	if staticIndex == "" {
		router = dynamicDocumentRouter{
			mode: mode,
		}
	} else {
		router = staticDocumentRouter{
			index: elasticsearch.Index{Index: staticIndex},
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
	mode MappingMode
}

func (r dynamicDocumentRouter) routeLogRecord(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return routeRecord(resource, scope, recordAttrs, r.mode, defaultDataStreamTypeLogs)
}

func (r dynamicDocumentRouter) routeDataPoint(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return routeRecord(resource, scope, recordAttrs, r.mode, defaultDataStreamTypeMetrics)
}

func (r dynamicDocumentRouter) routeSpan(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return routeRecord(resource, scope, recordAttrs, r.mode, defaultDataStreamTypeTraces)
}

func (r dynamicDocumentRouter) routeSpanEvent(resource pcommon.Resource, scope pcommon.InstrumentationScope, recordAttrs pcommon.Map) (elasticsearch.Index, error) {
	return routeRecord(resource, scope, recordAttrs, r.mode, defaultDataStreamTypeLogs)
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
	mode MappingMode,
	defaultDSType string,
) (elasticsearch.Index, error) {
	resourceAttr := resource.Attributes()
	scopeAttr := scope.Attributes()

	// Order:
	// 1. elasticsearch.index from attributes
	// 2. read data_stream.* from attributes
	// 3. receiver-based routing
	// 4. use default hardcoded data_stream.*
	if esIndex, esIndexExists := getFromAttributes(elasticsearch.IndexAttributeName, "", recordAttr, scopeAttr, resourceAttr); esIndexExists {
		// Advanced users can route documents by setting IndexAttributeName in a processor earlier in the pipeline.
		// If `data_stream.*` needs to be set in the document, users should use `data_stream.*` attributes.
		return elasticsearch.Index{Index: esIndex}, nil
	}

	dataset, datasetExists := getFromAttributes(elasticsearch.DataStreamDataset, defaultDataStreamDataset, recordAttr, scopeAttr, resourceAttr)
	namespace, _ := getFromAttributes(elasticsearch.DataStreamNamespace, defaultDataStreamNamespace, recordAttr, scopeAttr, resourceAttr)

	dsType := defaultDSType
	// if mapping mode is bodymap, allow overriding data_stream.type
	if mode == MappingBodyMap {
		dsType, _ = getFromAttributes(elasticsearch.DataStreamType, defaultDSType, recordAttr, scopeAttr, resourceAttr)
		if dsType != "logs" && dsType != "metrics" {
			return elasticsearch.Index{}, errors.New("data_stream.type cannot be other than logs or metrics")
		}
	}

	// Only use receiver-based routing if dataset is not specified.
	if !datasetExists {
		// Receiver-based routing
		// For example, hostmetricsreceiver (or hostmetricsreceiver.otel in the OTel output mode)
		// for the scope name
		// github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper
		if submatch := receiverRegex.FindStringSubmatch(scope.Name()); len(submatch) > 0 {
			receiverName := submatch[1]
			dataset = receiverName
		}
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
