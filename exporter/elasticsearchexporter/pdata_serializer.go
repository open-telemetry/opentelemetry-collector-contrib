// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"encoding/hex"
	"strings"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const tsLayout = "2006-01-02T15:04:05.000000000Z"

func serializeMetrics(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, dataPoints []dataPoint, validationErrors *[]error) ([]byte, map[string]string, error) {
	if len(dataPoints) == 0 {
		return nil, nil, nil
	}
	dp0 := dataPoints[0]
	var buf bytes.Buffer

	v := json.NewVisitor(&buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return nil, nil, err
	}
	if err := writeTimestampField(v, "@timestamp", dp0.Timestamp()); err != nil {
		return nil, nil, err
	}
	if dp0.StartTimestamp() != 0 {
		if err := writeTimestampField(v, "start_timestamp", dp0.StartTimestamp()); err != nil {
			return nil, nil, err
		}
	}
	if err := writeStringFieldSkipDefault(v, "unit", dp0.Metric().Unit()); err != nil {
		return nil, nil, err
	}
	if err := writeDataStream(v, dp0.Attributes()); err != nil {
		return nil, nil, err
	}
	if err := writeAttributes(v, dp0.Attributes(), true); err != nil {
		return nil, nil, err
	}
	if err := writeResource(v, resource, resourceSchemaURL, true); err != nil {
		return nil, nil, err
	}
	if err := writeScope(v, scope, scopeSchemaURL, true); err != nil {
		return nil, nil, err
	}
	dynamicTemplates, serr := serializeDataPoints(v, dataPoints, validationErrors)
	if serr != nil {
		return nil, nil, serr
	}
	if err := v.OnObjectFinished(); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), dynamicTemplates, nil
}

func serializeDataPoints(v *json.Visitor, dataPoints []dataPoint, validationErrors *[]error) (map[string]string, error) {
	if err := v.OnKey("metrics"); err != nil {
		return nil, err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return nil, err
	}

	dynamicTemplates := make(map[string]string, len(dataPoints))
	var docCount uint64
	for _, dp := range dataPoints {
		metric := dp.Metric()
		value, err := dp.Value()
		if dp.HasMappingHint(hintDocCount) {
			docCount = dp.DocCount()
		}
		if err != nil {
			*validationErrors = append(*validationErrors, err)
			continue
		}
		if err = v.OnKey(metric.Name()); err != nil {
			return nil, err
		}
		// TODO: support quantiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34561
		if err := writeValue(v, value, false); err != nil {
			return nil, err
		}
		// DynamicTemplate returns the name of dynamic template that applies to the metric and data point,
		// so that the field is indexed into Elasticsearch with the correct mapping. The name should correspond to a
		// dynamic template that is defined in ES mapping, e.g.
		// https://github.com/elastic/elasticsearch/blob/8.15/x-pack/plugin/core/template-resources/src/main/resources/metrics%40mappings.json
		dynamicTemplates["metrics."+metric.Name()] = dp.DynamicTemplate(metric)
	}
	if err := v.OnObjectFinished(); err != nil {
		return nil, err
	}
	if docCount != 0 {
		if err := writeUIntField(v, "_doc_count", docCount); err != nil {
			return nil, err
		}
	}
	return dynamicTemplates, nil
}

func serializeSpanEvent(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span, spanEvent ptrace.SpanEvent) ([]byte, error) {
	var buf bytes.Buffer

	v := json.NewVisitor(&buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return nil, err
	}
	if err := writeTimestampField(v, "@timestamp", spanEvent.Timestamp()); err != nil {
		return nil, err
	}
	if err := writeDataStream(v, spanEvent.Attributes()); err != nil {
		return nil, err
	}
	if err := writeTraceIDField(v, span.TraceID()); err != nil {
		return nil, err
	}
	if err := writeSpanIDField(v, "span_id", span.SpanID()); err != nil {
		return nil, err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(spanEvent.DroppedAttributesCount())); err != nil {
		return nil, err
	}
	if err := writeStringFieldSkipDefault(v, "event_name", spanEvent.Name()); err != nil {
		return nil, err
	}

	var attributes pcommon.Map
	if spanEvent.Name() != "" {
		attributes = pcommon.NewMap()
		spanEvent.Attributes().CopyTo(attributes)
		attributes.PutStr("event.name", spanEvent.Name())
	} else {
		attributes = spanEvent.Attributes()
	}
	if err := writeAttributes(v, attributes, false); err != nil {
		return nil, err
	}
	if err := writeResource(v, resource, resourceSchemaURL, false); err != nil {
		return nil, err
	}
	if err := writeScope(v, scope, scopeSchemaURL, false); err != nil {
		return nil, err
	}
	if err := v.OnObjectFinished(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func serializeSpan(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span) ([]byte, error) {
	var buf bytes.Buffer

	v := json.NewVisitor(&buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return nil, err
	}
	if err := writeTimestampField(v, "@timestamp", span.StartTimestamp()); err != nil {
		return nil, err
	}
	if err := writeDataStream(v, span.Attributes()); err != nil {
		return nil, err
	}
	if err := writeTraceIDField(v, span.TraceID()); err != nil {
		return nil, err
	}
	if err := writeSpanIDField(v, "span_id", span.SpanID()); err != nil {
		return nil, err
	}
	if err := writeStringFieldSkipDefault(v, "trace_state", span.TraceState().AsRaw()); err != nil {
		return nil, err
	}
	if err := writeSpanIDField(v, "parent_span_id", span.ParentSpanID()); err != nil {
		return nil, err
	}
	if err := writeStringFieldSkipDefault(v, "name", span.Name()); err != nil {
		return nil, err
	}
	if err := writeStringFieldSkipDefault(v, "kind", span.Kind().String()); err != nil {
		return nil, err
	}
	if err := writeUIntField(v, "duration", uint64(span.EndTimestamp()-span.StartTimestamp())); err != nil {
		return nil, err
	}
	if err := writeAttributes(v, span.Attributes(), false); err != nil {
		return nil, err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(span.DroppedAttributesCount())); err != nil {
		return nil, err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_events_count", int64(span.DroppedEventsCount())); err != nil {
		return nil, err
	}
	if err := writeSpanLinks(v, span); err != nil {
		return nil, err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_links_count", int64(span.DroppedLinksCount())); err != nil {
		return nil, err
	}
	if err := writeStatus(v, span.Status()); err != nil {
		return nil, err
	}
	if err := writeResource(v, resource, resourceSchemaURL, false); err != nil {
		return nil, err
	}
	if err := writeScope(v, scope, scopeSchemaURL, false); err != nil {
		return nil, err
	}
	if err := v.OnObjectFinished(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeStatus(v *json.Visitor, status ptrace.Status) error {
	if err := v.OnKey("status"); err != nil {
		return err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "message", status.Message()); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "code", status.Code().String()); err != nil {
		return err
	}
	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeSpanLinks(v *json.Visitor, span ptrace.Span) error {
	if err := v.OnKey("links"); err != nil {
		return err
	}
	if err := v.OnArrayStart(-1, structform.AnyType); err != nil {
		return err
	}
	spanLinks := span.Links()
	for i := 0; i < spanLinks.Len(); i++ {
		spanLink := spanLinks.At(i)
		if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
			return err
		}
		if err := writeStringFieldSkipDefault(v, "trace_id", spanLink.TraceID().String()); err != nil {
			return err
		}
		if err := writeStringFieldSkipDefault(v, "span_id", spanLink.SpanID().String()); err != nil {
			return err
		}
		if err := writeStringFieldSkipDefault(v, "trace_state", spanLink.TraceState().AsRaw()); err != nil {
			return err
		}
		if err := writeAttributes(v, spanLink.Attributes(), false); err != nil {
			return err
		}
		if err := writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(spanLink.DroppedAttributesCount())); err != nil {
			return err
		}
		if err := v.OnObjectFinished(); err != nil {
			return err
		}
	}
	if err := v.OnArrayFinished(); err != nil {
		return err
	}
	return nil
}

func serializeLog(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, record plog.LogRecord) ([]byte, error) {
	var buf bytes.Buffer

	v := json.NewVisitor(&buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return nil, err
	}
	docTimeStamp := record.Timestamp()
	if docTimeStamp.AsTime().UnixNano() == 0 {
		docTimeStamp = record.ObservedTimestamp()
	}
	if err := writeTimestampField(v, "@timestamp", docTimeStamp); err != nil {
		return nil, err
	}
	if err := writeTimestampField(v, "observed_timestamp", record.ObservedTimestamp()); err != nil {
		return nil, err
	}
	if err := writeDataStream(v, record.Attributes()); err != nil {
		return nil, err
	}
	if err := writeStringFieldSkipDefault(v, "severity_text", record.SeverityText()); err != nil {
		return nil, err
	}
	if err := writeIntFieldSkipDefault(v, "severity_number", int64(record.SeverityNumber())); err != nil {
		return nil, err
	}
	if err := writeTraceIDField(v, record.TraceID()); err != nil {
		return nil, err
	}
	if err := writeSpanIDField(v, "span_id", record.SpanID()); err != nil {
		return nil, err
	}
	if err := writeAttributes(v, record.Attributes(), false); err != nil {
		return nil, err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(record.DroppedAttributesCount())); err != nil {
		return nil, err
	}
	if record.EventName() != "" {
		if err := writeStringFieldSkipDefault(v, "event_name", record.EventName()); err != nil {
			return nil, err
		}
	} else if eventNameAttr, ok := record.Attributes().Get("event.name"); ok && eventNameAttr.Str() != "" {
		if err := writeStringFieldSkipDefault(v, "event_name", eventNameAttr.Str()); err != nil {
			return nil, err
		}
	}
	if err := writeResource(v, resource, resourceSchemaURL, false); err != nil {
		return nil, err
	}
	if err := writeScope(v, scope, scopeSchemaURL, false); err != nil {
		return nil, err
	}
	if err := writeLogBody(v, record); err != nil {
		return nil, err
	}
	if err := v.OnObjectFinished(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeDataStream(v *json.Visitor, attributes pcommon.Map) error {
	if err := v.OnKey("data_stream"); err != nil {
		return err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	var err error
	attributes.Range(func(k string, val pcommon.Value) bool {
		if strings.HasPrefix(k, "data_stream.") && val.Type() == pcommon.ValueTypeStr {
			if err = writeStringFieldSkipDefault(v, k[12:], val.Str()); err != nil {
				return false
			}
		}
		return true
	})
	if err != nil {
		return err
	}

	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeLogBody(v *json.Visitor, record plog.LogRecord) error {
	if record.Body().Type() == pcommon.ValueTypeEmpty {
		return nil
	}
	if err := v.OnKey("body"); err != nil {
		return err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}

	// Determine if this log record is an event, as they are mapped differently
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/general/events.md
	var bodyType string
	if _, hasEventNameAttribute := record.Attributes().Get("event.name"); hasEventNameAttribute || record.EventName() != "" {
		bodyType = "structured"
	} else {
		bodyType = "flattened"
	}
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
	if err := v.OnKey(bodyType); err != nil {
		return err
	}
	if err := writeValue(v, body, false); err != nil {
		return err
	}
	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeResource(v *json.Visitor, resource pcommon.Resource, resourceSchemaURL string, stringifyMapAttributes bool) error {
	if err := v.OnKey("resource"); err != nil {
		return err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "schema_url", resourceSchemaURL); err != nil {
		return err
	}
	if err := writeAttributes(v, resource.Attributes(), stringifyMapAttributes); err != nil {
		return err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(resource.DroppedAttributesCount())); err != nil {
		return err
	}
	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeScope(v *json.Visitor, scope pcommon.InstrumentationScope, scopeSchemaURL string, stringifyMapAttributes bool) error {
	if err := v.OnKey("scope"); err != nil {
		return err
	}
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "schema_url", scopeSchemaURL); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "name", scope.Name()); err != nil {
		return err
	}
	if err := writeStringFieldSkipDefault(v, "version", scope.Version()); err != nil {
		return err
	}
	if err := writeAttributes(v, scope.Attributes(), stringifyMapAttributes); err != nil {
		return err
	}
	if err := writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(scope.DroppedAttributesCount())); err != nil {
		return err
	}
	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeAttributes(v *json.Visitor, attributes pcommon.Map, stringifyMapValues bool) error {
	if attributes.Len() == 0 {
		return nil
	}
	attrCopy := pcommon.NewMap()
	attributes.CopyTo(attrCopy)
	attrCopy.RemoveIf(func(key string, _ pcommon.Value) bool {
		switch key {
		case dataStreamType, dataStreamDataset, dataStreamNamespace, mappingHintsAttrKey:
			return true
		}
		return false
	})
	mergeGeolocation(attrCopy)
	if attrCopy.Len() == 0 {
		return nil
	}
	if err := v.OnKey("attributes"); err != nil {
		return err
	}
	if err := writeMap(v, attrCopy, stringifyMapValues); err != nil {
		return err
	}
	return nil
}

func writeMap(v *json.Visitor, attributes pcommon.Map, stringifyMapValues bool) error {
	if err := v.OnObjectStart(-1, structform.AnyType); err != nil {
		return err
	}
	var err error
	attributes.Range(func(k string, val pcommon.Value) bool {
		if err = v.OnKey(k); err != nil {
			return false
		}
		err = writeValue(v, val, stringifyMapValues)
		return err == nil
	})
	if err != nil {
		return err
	}
	if err := v.OnObjectFinished(); err != nil {
		return err
	}
	return nil
}

func writeValue(v *json.Visitor, val pcommon.Value, stringifyMaps bool) error {
	switch val.Type() {
	case pcommon.ValueTypeEmpty:
		if err := v.OnNil(); err != nil {
			return err
		}
	case pcommon.ValueTypeStr:
		if err := v.OnString(val.Str()); err != nil {
			return err
		}
	case pcommon.ValueTypeBool:
		if err := v.OnBool(val.Bool()); err != nil {
			return err
		}
	case pcommon.ValueTypeDouble:
		if err := v.OnFloat64(val.Double()); err != nil {
			return err
		}
	case pcommon.ValueTypeInt:
		if err := v.OnInt64(val.Int()); err != nil {
			return err
		}
	case pcommon.ValueTypeBytes:
		if err := v.OnString(hex.EncodeToString(val.Bytes().AsRaw())); err != nil {
			return err
		}
	case pcommon.ValueTypeMap:
		if stringifyMaps {
			if err := v.OnString(val.AsString()); err != nil {
				return err
			}
		} else {
			if err := writeMap(v, val.Map(), false); err != nil {
				return err
			}
		}
	case pcommon.ValueTypeSlice:
		if err := v.OnArrayStart(-1, structform.AnyType); err != nil {
			return err
		}
		slice := val.Slice()
		for i := 0; i < slice.Len(); i++ {
			if err := writeValue(v, slice.At(i), stringifyMaps); err != nil {
				return err
			}
		}
		if err := v.OnArrayFinished(); err != nil {
			return err
		}
	}
	return nil
}

func writeTimestampField(v *json.Visitor, key string, timestamp pcommon.Timestamp) error {
	if err := v.OnKey(key); err != nil {
		return err
	}
	if err := v.OnString(timestamp.AsTime().UTC().Format(tsLayout)); err != nil {
		return err
	}
	return nil
}

func writeUIntField(v *json.Visitor, key string, i uint64) error {
	if err := v.OnKey(key); err != nil {
		return err
	}
	if err := v.OnUint64(i); err != nil {
		return err
	}
	return nil
}

func writeIntFieldSkipDefault(v *json.Visitor, key string, i int64) error {
	if i == 0 {
		return nil
	}
	if err := v.OnKey(key); err != nil {
		return err
	}
	if err := v.OnInt64(i); err != nil {
		return err
	}
	return nil
}

func writeStringFieldSkipDefault(v *json.Visitor, key, value string) error {
	if value == "" {
		return nil
	}
	if err := v.OnKey(key); err != nil {
		return err
	}
	if err := v.OnString(value); err != nil {
		return err
	}
	return nil
}

func writeTraceIDField(v *json.Visitor, id pcommon.TraceID) error {
	if id.IsEmpty() {
		return nil
	}
	if err := v.OnKey("trace_id"); err != nil {
		return err
	}
	if err := v.OnString(hex.EncodeToString(id[:])); err != nil {
		return err
	}
	return nil
}

func writeSpanIDField(v *json.Visitor, key string, id pcommon.SpanID) error {
	if id.IsEmpty() {
		return nil
	}
	if err := v.OnKey(key); err != nil {
		return err
	}
	if err := v.OnString(hex.EncodeToString(id[:])); err != nil {
		return err
	}
	return nil
}
