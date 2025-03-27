// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"
)

func (*Serializer) SerializeLog(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, record plog.LogRecord, idx elasticsearch.Index, buf *bytes.Buffer) error {
	v := json.NewVisitor(buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	_ = v.OnObjectStart(-1, structform.AnyType)
	docTimeStamp := record.Timestamp()
	if docTimeStamp.AsTime().UnixNano() == 0 {
		docTimeStamp = record.ObservedTimestamp()
	}
	writeTimestampField(v, "@timestamp", docTimeStamp)
	writeTimestampField(v, "observed_timestamp", record.ObservedTimestamp())
	writeDataStream(v, idx)
	writeStringFieldSkipDefault(v, "severity_text", record.SeverityText())
	writeIntFieldSkipDefault(v, "severity_number", int64(record.SeverityNumber()))
	writeTraceIDField(v, record.TraceID())
	writeSpanIDField(v, "span_id", record.SpanID())
	writeAttributes(v, record.Attributes(), false)
	writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(record.DroppedAttributesCount()))
	if record.EventName() != "" {
		writeStringFieldSkipDefault(v, "event_name", record.EventName())
	} else if eventNameAttr, ok := record.Attributes().Get("event.name"); ok && eventNameAttr.Str() != "" {
		writeStringFieldSkipDefault(v, "event_name", eventNameAttr.Str())
	}
	writeResource(v, resource, resourceSchemaURL, false)
	writeScope(v, scope, scopeSchemaURL, false)
	writeLogBody(v, record)
	_ = v.OnObjectFinished()
	return nil
}

func writeLogBody(v *json.Visitor, record plog.LogRecord) {
	if record.Body().Type() == pcommon.ValueTypeEmpty {
		return
	}
	_ = v.OnKey("body")
	_ = v.OnObjectStart(-1, structform.AnyType)

	bodyType := "structured"
	body := record.Body()
	switch body.Type() {
	case pcommon.ValueTypeMap:
	case pcommon.ValueTypeSlice:
		// output must be an array of objects due to ES limitations
		// otherwise, wrap the array in an object
		s := body.Slice()
		allMaps := true
		for i := 0; i < s.Len(); i++ {
			if s.At(i).Type() != pcommon.ValueTypeMap {
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
	_ = v.OnKey(bodyType)
	serializer.WriteValue(v, body, false)
	_ = v.OnObjectFinished()
}
