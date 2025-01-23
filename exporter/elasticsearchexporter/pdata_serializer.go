// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

const tsLayout = "2006-01-02T15:04:05.000000000Z"

func serializeMetrics(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, dataPoints []dataPoint, validationErrors *[]error, idx esIndex, buf *bytes.Buffer) (map[string]string, error) {
	if len(dataPoints) == 0 {
		return nil, nil
	}
	dp0 := dataPoints[0]

	v := json.NewVisitor(buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeTimestampField(v, "@timestamp", dp0.Timestamp())
	if dp0.StartTimestamp() != 0 {
		writeTimestampField(v, "start_timestamp", dp0.StartTimestamp())
	}
	writeStringFieldSkipDefault(v, "unit", dp0.Metric().Unit())
	writeDataStream(v, idx)
	writeAttributes(v, dp0.Attributes(), true)
	writeResource(v, resource, resourceSchemaURL, true)
	writeScope(v, scope, scopeSchemaURL, true)
	dynamicTemplates := serializeDataPoints(v, dataPoints, validationErrors)
	_ = v.OnObjectFinished()
	return dynamicTemplates, nil
}

func serializeDataPoints(v *json.Visitor, dataPoints []dataPoint, validationErrors *[]error) map[string]string {
	_ = v.OnKey("metrics")
	_ = v.OnObjectStart(-1, structform.AnyType)

	dynamicTemplates := make(map[string]string, len(dataPoints))
	var docCount uint64
	metricNames := make(map[string]bool, len(dataPoints))
	for _, dp := range dataPoints {
		metric := dp.Metric()
		if _, present := metricNames[metric.Name()]; present {
			*validationErrors = append(
				*validationErrors,
				fmt.Errorf(
					"metric with name '%s' has already been serialized in document with timestamp %s",
					metric.Name(),
					dp.Timestamp().AsTime().UTC().Format(tsLayout),
				),
			)
			continue
		}
		metricNames[metric.Name()] = true
		// TODO here's potential for more optimization by directly serializing the value instead of allocating a pcommon.Value
		//  the tradeoff is that this would imply a duplicated logic for the ECS mode
		value, err := dp.Value()
		if dp.HasMappingHint(elasticsearch.HintDocCount) {
			docCount = dp.DocCount()
		}
		if err != nil {
			*validationErrors = append(*validationErrors, err)
			continue
		}
		_ = v.OnKey(metric.Name())
		// TODO: support quantiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34561
		writeValue(v, value, false)
		// DynamicTemplate returns the name of dynamic template that applies to the metric and data point,
		// so that the field is indexed into Elasticsearch with the correct mapping. The name should correspond to a
		// dynamic template that is defined in ES mapping, e.g.
		// https://github.com/elastic/elasticsearch/blob/8.15/x-pack/plugin/core/template-resources/src/main/resources/metrics%40mappings.json
		dynamicTemplates["metrics."+metric.Name()] = dp.DynamicTemplate(metric)
	}
	_ = v.OnObjectFinished()
	if docCount != 0 {
		writeUIntField(v, "_doc_count", docCount)
	}
	return dynamicTemplates
}

func serializeSpanEvent(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span, spanEvent ptrace.SpanEvent, idx esIndex, buf *bytes.Buffer) {
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

func serializeSpan(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span, idx esIndex, buf *bytes.Buffer) error {
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

func serializeMap(m pcommon.Map, buf *bytes.Buffer) {
	v := json.NewVisitor(buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	writeMap(v, m, false)
}

func serializeLog(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, record plog.LogRecord, idx esIndex, buf *bytes.Buffer) error {
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
	isEvent := false
	if record.EventName() != "" {
		isEvent = true
		writeStringFieldSkipDefault(v, "event_name", record.EventName())
	} else if eventNameAttr, ok := record.Attributes().Get("event.name"); ok && eventNameAttr.Str() != "" {
		isEvent = true
		writeStringFieldSkipDefault(v, "event_name", eventNameAttr.Str())
	}
	writeResource(v, resource, resourceSchemaURL, false)
	writeScope(v, scope, scopeSchemaURL, false)
	writeLogBody(v, record, isEvent)
	_ = v.OnObjectFinished()
	return nil
}

func writeDataStream(v *json.Visitor, idx esIndex) {
	if !idx.isDataStream() {
		return
	}
	_ = v.OnKey("data_stream")
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeStringFieldSkipDefault(v, "type", idx.Type)
	writeStringFieldSkipDefault(v, "dataset", idx.Dataset)
	writeStringFieldSkipDefault(v, "namespace", idx.Namespace)
	_ = v.OnObjectFinished()
}

func writeLogBody(v *json.Visitor, record plog.LogRecord, isEvent bool) {
	if record.Body().Type() == pcommon.ValueTypeEmpty {
		return
	}
	_ = v.OnKey("body")
	_ = v.OnObjectStart(-1, structform.AnyType)

	// Determine if this log record is an event, as they are mapped differently
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/general/events.md
	var bodyType string
	if isEvent {
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
	_ = v.OnKey(bodyType)
	writeValue(v, body, false)
	_ = v.OnObjectFinished()
}

func writeResource(v *json.Visitor, resource pcommon.Resource, resourceSchemaURL string, stringifyMapAttributes bool) {
	_ = v.OnKey("resource")
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeStringFieldSkipDefault(v, "schema_url", resourceSchemaURL)
	writeAttributes(v, resource.Attributes(), stringifyMapAttributes)
	writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(resource.DroppedAttributesCount()))
	_ = v.OnObjectFinished()
}

func writeScope(v *json.Visitor, scope pcommon.InstrumentationScope, scopeSchemaURL string, stringifyMapAttributes bool) {
	_ = v.OnKey("scope")
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeStringFieldSkipDefault(v, "schema_url", scopeSchemaURL)
	writeStringFieldSkipDefault(v, "name", scope.Name())
	writeStringFieldSkipDefault(v, "version", scope.Version())
	writeAttributes(v, scope.Attributes(), stringifyMapAttributes)
	writeIntFieldSkipDefault(v, "dropped_attributes_count", int64(scope.DroppedAttributesCount()))
	_ = v.OnObjectFinished()
}

func writeAttributes(v *json.Visitor, attributes pcommon.Map, stringifyMapValues bool) {
	if attributes.Len() == 0 {
		return
	}

	_ = v.OnKey("attributes")
	_ = v.OnObjectStart(-1, structform.AnyType)
	attributes.Range(func(k string, val pcommon.Value) bool {
		switch k {
		case dataStreamType, dataStreamDataset, dataStreamNamespace, elasticsearch.MappingHintsAttrKey, documentIDAttributeName:
			return true
		}
		if isGeoAttribute(k, val) {
			return true
		}
		_ = v.OnKey(k)
		writeValue(v, val, stringifyMapValues)
		return true
	})
	writeGeolocationAttributes(v, attributes)
	_ = v.OnObjectFinished()
}

func isGeoAttribute(k string, val pcommon.Value) bool {
	if val.Type() != pcommon.ValueTypeDouble {
		return false
	}
	switch k {
	case "geo.location.lat", "geo.location.lon":
		return true
	}
	return strings.HasSuffix(k, ".geo.location.lat") || strings.HasSuffix(k, ".geo.location.lon")
}

func writeGeolocationAttributes(v *json.Visitor, attributes pcommon.Map) {
	const (
		lonKey    = "geo.location.lon"
		latKey    = "geo.location.lat"
		mergedKey = "geo.location"
	)
	// Prefix is the attribute name without lonKey or latKey suffix
	// e.g. prefix of "foo.bar.geo.location.lon" is "foo.bar.", prefix of "geo.location.lon" is "".
	prefixToGeo := make(map[string]struct {
		lon, lat       float64
		lonSet, latSet bool
	})
	setLon := func(prefix string, v float64) {
		g := prefixToGeo[prefix]
		g.lon = v
		g.lonSet = true
		prefixToGeo[prefix] = g
	}
	setLat := func(prefix string, v float64) {
		g := prefixToGeo[prefix]
		g.lat = v
		g.latSet = true
		prefixToGeo[prefix] = g
	}
	attributes.Range(func(key string, val pcommon.Value) bool {
		if val.Type() != pcommon.ValueTypeDouble {
			return true
		}

		if key == lonKey {
			setLon("", val.Double())
			return true
		} else if key == latKey {
			setLat("", val.Double())
			return true
		} else if namespace, found := strings.CutSuffix(key, "."+lonKey); found {
			prefix := namespace + "."
			setLon(prefix, val.Double())
			return true
		} else if namespace, found := strings.CutSuffix(key, "."+latKey); found {
			prefix := namespace + "."
			setLat(prefix, val.Double())
			return true
		}
		return true
	})

	for prefix, geo := range prefixToGeo {
		if geo.lonSet && geo.latSet {
			key := prefix + mergedKey
			// Geopoint expressed as an array with the format: [lon, lat]
			_ = v.OnKey(key)
			_ = v.OnArrayStart(-1, structform.AnyType)
			_ = v.OnFloat64(geo.lon)
			_ = v.OnFloat64(geo.lat)
			_ = v.OnArrayFinished()
			continue
		}
		// Place the attributes back if lon and lat are not present together
		if geo.lonSet {
			_ = v.OnKey(prefix + lonKey)
			_ = v.OnFloat64(geo.lon)
		}
		if geo.latSet {
			_ = v.OnKey(prefix + latKey)
			_ = v.OnFloat64(geo.lat)
		}
	}
}

func writeMap(v *json.Visitor, m pcommon.Map, stringifyMapValues bool) {
	_ = v.OnObjectStart(-1, structform.AnyType)
	m.Range(func(k string, val pcommon.Value) bool {
		_ = v.OnKey(k)
		writeValue(v, val, stringifyMapValues)
		return true
	})
	_ = v.OnObjectFinished()
}

func writeValue(v *json.Visitor, val pcommon.Value, stringifyMaps bool) {
	switch val.Type() {
	case pcommon.ValueTypeEmpty:
		_ = v.OnNil()
	case pcommon.ValueTypeStr:
		_ = v.OnString(val.Str())
	case pcommon.ValueTypeBool:
		_ = v.OnBool(val.Bool())
	case pcommon.ValueTypeDouble:
		_ = v.OnFloat64(val.Double())
	case pcommon.ValueTypeInt:
		_ = v.OnInt64(val.Int())
	case pcommon.ValueTypeBytes:
		_ = v.OnString(hex.EncodeToString(val.Bytes().AsRaw()))
	case pcommon.ValueTypeMap:
		if stringifyMaps {
			_ = v.OnString(val.AsString())
		} else {
			writeMap(v, val.Map(), false)
		}
	case pcommon.ValueTypeSlice:
		_ = v.OnArrayStart(-1, structform.AnyType)
		slice := val.Slice()
		for i := 0; i < slice.Len(); i++ {
			writeValue(v, slice.At(i), stringifyMaps)
		}
		_ = v.OnArrayFinished()
	}
}

func writeTimestampField(v *json.Visitor, key string, timestamp pcommon.Timestamp) {
	_ = v.OnKey(key)
	nsec := uint64(timestamp)
	msec := nsec / 1e6
	nsec -= msec * 1e6
	_ = v.OnString(strconv.FormatUint(msec, 10) + "." + strconv.FormatUint(nsec, 10))
}

func writeUIntField(v *json.Visitor, key string, i uint64) {
	_ = v.OnKey(key)
	_ = v.OnUint64(i)
}

func writeIntFieldSkipDefault(v *json.Visitor, key string, i int64) {
	if i == 0 {
		return
	}
	_ = v.OnKey(key)
	_ = v.OnInt64(i)
}

func writeStringFieldSkipDefault(v *json.Visitor, key, value string) {
	if value == "" {
		return
	}
	_ = v.OnKey(key)
	_ = v.OnString(value)
}

func writeTraceIDField(v *json.Visitor, id pcommon.TraceID) {
	if id.IsEmpty() {
		return
	}
	_ = v.OnKey("trace_id")
	_ = v.OnString(hex.EncodeToString(id[:]))
}

func writeSpanIDField(v *json.Visitor, key string, id pcommon.SpanID) {
	if id.IsEmpty() {
		return
	}
	_ = v.OnKey(key)
	_ = v.OnString(hex.EncodeToString(id[:]))
}
