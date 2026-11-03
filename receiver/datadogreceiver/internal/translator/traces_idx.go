// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace/idx"
)

// Datadog Agent 7.73+ sends traces on /api/v0.2/traces in AgentPayload.IdxTracerPayloads instead of
// AgentPayload.TracerPayloads. These payloads use a string-table ("idx") format where every string is
// a reference into TracerPayload.Strings, the 128-bit trace ID lives on the chunk, and a few span tags
// (env, version, component, span.kind) are promoted to dedicated span fields. tracerPayloadFromIdx
// converts them back to the legacy pb.TracerPayload so they share the existing translation to OTLP.

// idxStrings resolves string table references and keeps the first out-of-range reference as an error,
// so a malformed payload is rejected instead of silently producing empty strings.
type idxStrings struct {
	table []string
	err   error
}

func (s *idxStrings) get(ref uint32) string {
	if int(ref) < len(s.table) {
		return s.table[ref]
	}
	if s.err == nil {
		s.err = fmt.Errorf("string table reference %d out of range (table size %d)", ref, len(s.table))
	}
	return ""
}

func tracerPayloadFromIdx(payload *idx.TracerPayload) (*pb.TracerPayload, error) {
	if payload == nil {
		return nil, nil
	}
	strs := &idxStrings{table: payload.Strings}
	tp := &pb.TracerPayload{
		ContainerID:     strs.get(payload.ContainerIDRef),
		LanguageName:    strs.get(payload.LanguageNameRef),
		LanguageVersion: strs.get(payload.LanguageVersionRef),
		TracerVersion:   strs.get(payload.TracerVersionRef),
		RuntimeID:       strs.get(payload.RuntimeIDRef),
		Env:             strs.get(payload.EnvRef),
		Hostname:        strs.get(payload.HostnameRef),
		AppVersion:      strs.get(payload.AppVersionRef),
		Tags:            idxStringMap(strs, payload.Attributes),
		Chunks:          make([]*pb.TraceChunk, 0, len(payload.Chunks)),
	}
	for _, chunk := range payload.Chunks {
		if chunk == nil {
			continue
		}
		tp.Chunks = append(tp.Chunks, traceChunkFromIdx(strs, chunk))
	}
	if strs.err != nil {
		return nil, strs.err
	}
	return tp, nil
}

func traceChunkFromIdx(strs *idxStrings, chunk *idx.TraceChunk) *pb.TraceChunk {
	var traceIDHigh, traceIDLow uint64
	if len(chunk.TraceID) == 16 {
		traceIDHigh = binary.BigEndian.Uint64(chunk.TraceID[:8])
		traceIDLow = binary.BigEndian.Uint64(chunk.TraceID[8:])
	}

	spans := make([]*pb.Span, 0, len(chunk.Spans))
	for _, span := range chunk.Spans {
		if span == nil {
			continue
		}
		spans = append(spans, spanFromIdx(strs, span, traceIDLow))
	}
	if traceIDHigh != 0 {
		setTraceIDHighTag(spans, traceIDHigh)
	}

	return &pb.TraceChunk{
		Priority:     chunk.Priority,
		Origin:       strs.get(chunk.OriginRef),
		Spans:        spans,
		Tags:         idxStringMap(strs, chunk.Attributes),
		DroppedTrace: chunk.DroppedTrace,
	}
}

// setTraceIDHighTag restores the upper 64 bits of the chunk trace ID as the _dd.p.tid tag on the
// chunk-local root span, mirroring what Datadog tracers send in the legacy format, unless a span
// already carries it. ToTraces reconstructs the 128-bit trace ID of every span of the payload from it.
func setTraceIDHighTag(spans []*pb.Span, traceIDHigh uint64) {
	if len(spans) == 0 {
		return
	}
	spanIDs := make(map[uint64]struct{}, len(spans))
	for _, span := range spans {
		if _, ok := span.Meta["_dd.p.tid"]; ok {
			return
		}
		spanIDs[span.SpanID] = struct{}{}
	}
	root := spans[0]
	for _, span := range spans {
		if _, ok := spanIDs[span.ParentID]; !ok {
			root = span
			break
		}
	}
	root.Meta["_dd.p.tid"] = fmt.Sprintf("%016x", traceIDHigh)
}

func spanFromIdx(strs *idxStrings, span *idx.Span, traceID uint64) *pb.Span {
	s := &pb.Span{
		Service:    strs.get(span.ServiceRef),
		Name:       strs.get(span.NameRef),
		Resource:   strs.get(span.ResourceRef),
		TraceID:    traceID,
		SpanID:     span.SpanID,
		ParentID:   span.ParentID,
		Start:      int64(span.Start),
		Duration:   int64(span.Duration),
		Type:       strs.get(span.TypeRef),
		Meta:       make(map[string]string, len(span.Attributes)+4),
		Metrics:    make(map[string]float64),
		SpanLinks:  make([]*pb.SpanLink, 0, len(span.Links)),
		SpanEvents: make([]*pb.SpanEvent, 0, len(span.Events)),
	}
	if span.Error {
		s.Error = 1
	}

	for k, v := range span.Attributes {
		if v == nil {
			continue
		}
		key := strs.get(k)
		switch val := v.Value.(type) {
		case *idx.AnyValue_DoubleValue:
			s.Metrics[key] = val.DoubleValue
		case *idx.AnyValue_IntValue:
			s.Metrics[key] = float64(val.IntValue)
		case *idx.AnyValue_BytesValue:
			if s.MetaStruct == nil {
				s.MetaStruct = make(map[string][]byte)
			}
			s.MetaStruct[key] = val.BytesValue
		default:
			s.Meta[key] = idxAnyValueString(strs, v)
		}
	}

	// Restore the tags the Datadog Agent promotes to dedicated span fields.
	setMetaIfMissing(s.Meta, "env", strs.get(span.EnvRef))
	setMetaIfMissing(s.Meta, "version", strs.get(span.VersionRef))
	setMetaIfMissing(s.Meta, "component", strs.get(span.ComponentRef))
	// SPAN_KIND_INTERNAL is also the Agent's default for spans without a span.kind tag, so it is not
	// restored: this keeps the span.type based kind inference of the legacy format for those spans.
	switch span.Kind {
	case idx.SpanKind_SPAN_KIND_SERVER:
		setMetaIfMissing(s.Meta, datadogSpanKindKey, "server")
	case idx.SpanKind_SPAN_KIND_CLIENT:
		setMetaIfMissing(s.Meta, datadogSpanKindKey, "client")
	case idx.SpanKind_SPAN_KIND_PRODUCER:
		setMetaIfMissing(s.Meta, datadogSpanKindKey, "producer")
	case idx.SpanKind_SPAN_KIND_CONSUMER:
		setMetaIfMissing(s.Meta, datadogSpanKindKey, "consumer")
	}

	for _, link := range span.Links {
		if link == nil {
			continue
		}
		l := &pb.SpanLink{
			SpanID:     link.SpanID,
			Tracestate: strs.get(link.TracestateRef),
			Flags:      link.Flags,
			Attributes: idxStringMap(strs, link.Attributes),
		}
		if len(link.TraceID) == 16 {
			l.TraceIDHigh = binary.BigEndian.Uint64(link.TraceID[:8])
			l.TraceID = binary.BigEndian.Uint64(link.TraceID[8:])
		}
		s.SpanLinks = append(s.SpanLinks, l)
	}

	for _, event := range span.Events {
		if event == nil {
			continue
		}
		e := &pb.SpanEvent{
			TimeUnixNano: event.Time,
			Name:         strs.get(event.NameRef),
			Attributes:   make(map[string]*pb.AttributeAnyValue, len(event.Attributes)),
		}
		for k, v := range event.Attributes {
			if attr := idxAttributeAnyValue(strs, v); attr != nil {
				e.Attributes[strs.get(k)] = attr
			}
		}
		s.SpanEvents = append(s.SpanEvents, e)
	}

	return s
}

func setMetaIfMissing(meta map[string]string, key, value string) {
	if value == "" {
		return
	}
	if _, ok := meta[key]; !ok {
		meta[key] = value
	}
}

func idxStringMap(strs *idxStrings, attrs map[uint32]*idx.AnyValue) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	m := make(map[string]string, len(attrs))
	for k, v := range attrs {
		if v == nil {
			continue
		}
		m[strs.get(k)] = idxAnyValueString(strs, v)
	}
	return m
}

func idxAnyValueString(strs *idxStrings, v *idx.AnyValue) string {
	switch val := v.Value.(type) {
	case *idx.AnyValue_StringValueRef:
		return strs.get(val.StringValueRef)
	case *idx.AnyValue_BoolValue:
		return strconv.FormatBool(val.BoolValue)
	case *idx.AnyValue_DoubleValue:
		return strconv.FormatFloat(val.DoubleValue, 'g', -1, 64)
	case *idx.AnyValue_IntValue:
		return strconv.FormatInt(val.IntValue, 10)
	case *idx.AnyValue_BytesValue:
		return base64.StdEncoding.EncodeToString(val.BytesValue)
	case *idx.AnyValue_ArrayValue, *idx.AnyValue_KeyValueList:
		b, err := json.Marshal(idxAnyValueRaw(strs, v))
		if err != nil {
			return ""
		}
		return string(b)
	}
	return ""
}

// idxAnyValueRaw converts an AnyValue to a plain Go value, used to encode composite values as JSON.
func idxAnyValueRaw(strs *idxStrings, v *idx.AnyValue) any {
	if v == nil {
		return nil
	}
	switch val := v.Value.(type) {
	case *idx.AnyValue_StringValueRef:
		return strs.get(val.StringValueRef)
	case *idx.AnyValue_BoolValue:
		return val.BoolValue
	case *idx.AnyValue_DoubleValue:
		return val.DoubleValue
	case *idx.AnyValue_IntValue:
		return val.IntValue
	case *idx.AnyValue_BytesValue:
		return val.BytesValue
	case *idx.AnyValue_ArrayValue:
		if val.ArrayValue == nil {
			return []any{}
		}
		values := make([]any, 0, len(val.ArrayValue.Values))
		for _, av := range val.ArrayValue.Values {
			values = append(values, idxAnyValueRaw(strs, av))
		}
		return values
	case *idx.AnyValue_KeyValueList:
		kvs := make(map[string]any)
		if val.KeyValueList != nil {
			for _, kv := range val.KeyValueList.KeyValues {
				if kv != nil {
					kvs[strs.get(kv.Key)] = idxAnyValueRaw(strs, kv.Value)
				}
			}
		}
		return kvs
	}
	return nil
}

// idxAttributeAnyValue converts an AnyValue to the legacy span event attribute type. Key-value lists,
// which have no legacy equivalent, are encoded as a JSON string.
func idxAttributeAnyValue(strs *idxStrings, v *idx.AnyValue) *pb.AttributeAnyValue {
	if v == nil {
		return nil
	}
	switch val := v.Value.(type) {
	case *idx.AnyValue_BoolValue:
		return &pb.AttributeAnyValue{Type: pb.AttributeAnyValue_BOOL_VALUE, BoolValue: val.BoolValue}
	case *idx.AnyValue_DoubleValue:
		return &pb.AttributeAnyValue{Type: pb.AttributeAnyValue_DOUBLE_VALUE, DoubleValue: val.DoubleValue}
	case *idx.AnyValue_IntValue:
		return &pb.AttributeAnyValue{Type: pb.AttributeAnyValue_INT_VALUE, IntValue: val.IntValue}
	case *idx.AnyValue_ArrayValue:
		arr := &pb.AttributeArray{}
		if val.ArrayValue != nil {
			for _, av := range val.ArrayValue.Values {
				if elem := idxAttributeArrayValue(strs, av); elem != nil {
					arr.Values = append(arr.Values, elem)
				}
			}
		}
		return &pb.AttributeAnyValue{Type: pb.AttributeAnyValue_ARRAY_VALUE, ArrayValue: arr}
	default:
		return &pb.AttributeAnyValue{Type: pb.AttributeAnyValue_STRING_VALUE, StringValue: idxAnyValueString(strs, v)}
	}
}

func idxAttributeArrayValue(strs *idxStrings, v *idx.AnyValue) *pb.AttributeArrayValue {
	if v == nil {
		return nil
	}
	switch val := v.Value.(type) {
	case *idx.AnyValue_BoolValue:
		return &pb.AttributeArrayValue{Type: pb.AttributeArrayValue_BOOL_VALUE, BoolValue: val.BoolValue}
	case *idx.AnyValue_DoubleValue:
		return &pb.AttributeArrayValue{Type: pb.AttributeArrayValue_DOUBLE_VALUE, DoubleValue: val.DoubleValue}
	case *idx.AnyValue_IntValue:
		return &pb.AttributeArrayValue{Type: pb.AttributeArrayValue_INT_VALUE, IntValue: val.IntValue}
	default:
		return &pb.AttributeArrayValue{Type: pb.AttributeArrayValue_STRING_VALUE, StringValue: idxAnyValueString(strs, v)}
	}
}
