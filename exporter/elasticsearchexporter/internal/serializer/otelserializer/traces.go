// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func (*Serializer) SerializeSpanEvent(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span, spanEvent ptrace.SpanEvent, idx elasticsearch.Index, buf *bytes.Buffer) {
	v := json.NewVisitor(buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeTimestampField(v, "@timestamp", spanEvent.Timestamp())
	writeDataStream(v, idx)
	writeTraceIDField(v, span.TraceID())
	writeSpanIDField(v, "span_id", span.SpanID())
	writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(spanEvent.DroppedAttributesCount()))
	writeStringFieldSkipDefault(v, "event_name", spanEvent.Name())

	var attributes pcommon.Map
	if spanEvent.Name() != "" {
		attributes = pcommon.NewMap()
		spanEvent.Attributes().CopyTo(attributes)
		attributes.PutStr("event.name", spanEvent.Name())
	} else {
		attributes = spanEvent.Attributes()
	}
	writeAttributes(v, attributes, false)
	writeResource(v, resource, resourceSchemaURL, false)
	writeScope(v, scope, scopeSchemaURL, false)
	_ = v.OnObjectFinished()
}

func (*Serializer) SerializeSpan(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span, idx elasticsearch.Index, buf *bytes.Buffer) error {
	v := json.NewVisitor(buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeTimestampField(v, "@timestamp", span.StartTimestamp())
	writeDataStream(v, idx)
	writeTraceIDField(v, span.TraceID())
	writeSpanIDField(v, "span_id", span.SpanID())
	writeStringFieldSkipDefault(v, "trace_state", span.TraceState().AsRaw())
	writeSpanIDField(v, "parent_span_id", span.ParentSpanID())
	writeStringFieldSkipDefault(v, "name", span.Name())
	writeStringFieldSkipDefault(v, "kind", span.Kind().String())
	writeUIntField(v, "duration", uint64(span.EndTimestamp()-span.StartTimestamp()))
	writeAttributes(v, span.Attributes(), false)
	writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(span.DroppedAttributesCount()))
	writeIntFieldSkipDefault(v, "dropped_events_count", int64(span.DroppedEventsCount()))
	writeSpanLinks(v, span)
	writeIntFieldSkipDefault(v, "dropped_links_count", int64(span.DroppedLinksCount()))
	writeStatus(v, span.Status())
	writeResource(v, resource, resourceSchemaURL, false)
	writeScope(v, scope, scopeSchemaURL, false)
	_ = v.OnObjectFinished()
	return nil
}

func writeStatus(v *json.Visitor, status ptrace.Status) {
	_ = v.OnKey("status")
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeStringFieldSkipDefault(v, "message", status.Message())
	writeStringFieldSkipDefault(v, "code", status.Code().String())
	_ = v.OnObjectFinished()
}

func writeSpanLinks(v *json.Visitor, span ptrace.Span) {
	_ = v.OnKey("links")
	_ = v.OnArrayStart(-1, structform.AnyType)
	spanLinks := span.Links()
	for i := 0; i < spanLinks.Len(); i++ {
		spanLink := spanLinks.At(i)
		_ = v.OnObjectStart(-1, structform.AnyType)
		writeStringFieldSkipDefault(v, "trace_id", spanLink.TraceID().String())
		writeStringFieldSkipDefault(v, "span_id", spanLink.SpanID().String())
		writeStringFieldSkipDefault(v, "trace_state", spanLink.TraceState().AsRaw())
		writeAttributes(v, spanLink.Attributes(), false)
		writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(spanLink.DroppedAttributesCount()))
		_ = v.OnObjectFinished()
	}
	_ = v.OnArrayFinished()
}
