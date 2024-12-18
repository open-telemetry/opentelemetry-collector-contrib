// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/ecs/internal/log"

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/otel"
)

type otelEncoder struct {
	dedot bool
}

func (e *otelEncoder) EncodeLog(resource pcommon.Resource, resourceSchemaURL string, record plog.LogRecord, scope pcommon.InstrumentationScope, scopeSchemaURL string) ([]byte, error) {
	var document objmodel.Document

	docTimeStamp := record.Timestamp()
	if docTimeStamp.AsTime().UnixNano() == 0 {
		docTimeStamp = record.ObservedTimestamp()
	}

	document.AddTimestamp("@timestamp", docTimeStamp)
	document.AddTimestamp("observed_timestamp", record.ObservedTimestamp())

	document.AddTraceID("trace_id", record.TraceID())
	document.AddSpanID("span_id", record.SpanID())
	document.AddString("severity_text", record.SeverityText())
	document.AddInt("severity_number", int64(record.SeverityNumber()))
	document.AddInt("dropped_attributes_count", int64(record.DroppedAttributesCount()))

	otel.EncodeAttributes(&document, record.Attributes())
	otel.EncodeResource(&document, resource, resourceSchemaURL)
	otel.EncodeScope(&document, scope, scopeSchemaURL)

	// Body
	setOTelLogBody(&document, record.Body(), record.Attributes())

	document.Dedup(false)
	var buf bytes.Buffer
	err := document.Serialize(&buf, e.dedot, true)
	return buf.Bytes(), err
}

func setOTelLogBody(doc *objmodel.Document, body pcommon.Value, attributes pcommon.Map) {
	// Determine if this log record is an event, as they are mapped differently
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/general/events.md
	_, isEvent := attributes.Get("event.name")

	switch body.Type() {
	case pcommon.ValueTypeMap:
		if isEvent {
			doc.AddAttribute("body.structured", body)
		} else {
			doc.AddAttribute("body.flattened", body)
		}
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

		var outVal pcommon.Value
		if allMaps {
			outVal = body
		} else {
			vm := pcommon.NewValueMap()
			m := vm.SetEmptyMap()
			body.Slice().CopyTo(m.PutEmptySlice("value"))
			outVal = vm
		}

		if isEvent {
			doc.AddAttribute("body.structured", outVal)
		} else {
			doc.AddAttribute("body.flattened", outVal)
		}
	case pcommon.ValueTypeStr:
		doc.AddString("body.text", body.Str())
	default:
		doc.AddString("body.text", body.AsString())
	}
}
