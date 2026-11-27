// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func (*Serializer) SerializeExemplar(
	resource pcommon.Resource,
	resourceSchemaURL string,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	idx elasticsearch.Index,
	buf *bytes.Buffer,
	e pmetric.Exemplar,
	metric pmetric.Metric,
	temporality string,
	dpAttrs pcommon.Map,
) error {
	value, err := datapoints.ExemplarValue(e)
	if err != nil {
		return err
	}

	w := newJSONWriter(buf)
	w.StartObject()
	first := true
	first = w.writeTimestampEpochMillisField("@timestamp", e.Timestamp(), first)
	first = w.writeDataStream(idx, first)
	first = w.writeResource(resource, resourceSchemaURL, true, first)
	first = w.writeScope(scope, scopeSchemaURL, true, first)
	first = w.writeAttributes(dpAttrs, true, first)
	first = w.writeStringFieldSkipDefault("unit", metric.Unit(), first)
	first = w.writeStringFieldSkipDefault("temporality", temporality, first)
	first = w.writeStringFieldSkipDefault("metric_name", metric.Name(), first)
	if e.FilteredAttributes().Len() > 0 {
		first = w.Key("filtered_attributes", first)
		w.writeMap(e.FilteredAttributes())
	}
	first = w.writeTraceIDField(e.TraceID(), first)
	first = w.writeSpanIDField("span_id", e.SpanID(), first)
	w.Key("value", first)
	w.writeValue(value, false)
	w.EndObject()
	return nil
}
