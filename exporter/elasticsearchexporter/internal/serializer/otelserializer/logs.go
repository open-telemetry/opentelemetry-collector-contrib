// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func (*Serializer) SerializeLog(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, record plog.LogRecord, idx elasticsearch.Index, buf *bytes.Buffer) error {
	w := newJSONWriter(buf)
	w.startObject()
	docTimeStamp := record.Timestamp()
	if docTimeStamp.AsTime().UnixNano() == 0 {
		docTimeStamp = record.ObservedTimestamp()
	}
	first := true
	first = w.writeTimestampField("@timestamp", docTimeStamp, first)
	first = w.writeTimestampField("observed_timestamp", record.ObservedTimestamp(), first)
	first = w.writeDataStream(idx, first)
	first = w.writeStringFieldSkipDefault("severity_text", record.SeverityText(), first)
	first = w.writeIntFieldSkipDefault("severity_number", int64(record.SeverityNumber()), first)
	first = w.writeTraceIDField(record.TraceID(), first)
	first = w.writeSpanIDField("span_id", record.SpanID(), first)
	first = w.writeAttributes(record.Attributes(), false, first)
	first = w.writeIntFieldSkipDefault("dropped_attributes_count", int64(record.DroppedAttributesCount()), first)
	if record.EventName() != "" {
		first = w.writeStringFieldSkipDefault("event_name", record.EventName(), first)
	} else if eventNameAttr, ok := record.Attributes().Get("event.name"); ok && eventNameAttr.Str() != "" {
		first = w.writeStringFieldSkipDefault("event_name", eventNameAttr.Str(), first)
	}
	first = w.writeResource(resource, resourceSchemaURL, false, first)
	first = w.writeScope(scope, scopeSchemaURL, false, first)
	writeLogBody(&w, record, first)
	w.endObject()
	return nil
}

func writeLogBody(w *jsonWriter, record plog.LogRecord, first bool) bool {
	if record.Body().Type() == pcommon.ValueTypeEmpty {
		return first
	}
	first = w.key("body", first)
	w.startObject()

	bodyType := "structured"
	body := record.Body()
	switch body.Type() {
	case pcommon.ValueTypeMap:
	case pcommon.ValueTypeSlice:
		// output must be an array of objects due to ES limitations
		// otherwise, wrap the array in an object
		allMaps := true
		for _, o := range body.Slice().All() {
			if o.Type() != pcommon.ValueTypeMap {
				allMaps = false
			}
		}

		if !allMaps {
			body = pcommon.NewValueMap()
			m := body.SetEmptyMap()
			record.Body().Slice().CopyTo(m.PutEmptySlice("value"))
		}
	default:
		bodyType = "text"
	}
	firstField := true
	firstField = w.key(bodyType, firstField)
	_ = firstField
	w.writeValue(body, false)
	w.endObject()
	return first
}
