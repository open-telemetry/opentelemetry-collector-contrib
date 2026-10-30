// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"strconv"

	"github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace/idx"
	ddsampler "github.com/DataDog/datadog-agent/pkg/trace/sampler"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"
)

// attributeConvertedV1 is set by the Datadog Agent on the spans it converts from the v0.x formats to the v1 format.
const attributeConvertedV1 = "_dd.convertedv1"

// ToTracesV1 converts a tracer payload in the v1 format to OTel traces. Attributes keep their native types and the
// 128-bit trace id is taken from the trace chunk as is. Span links and events that are in JSON tags are parsed, and
// spans that the agent converted from the v0.x formats get the span kind that the v0.x translation produces for them.
func ToTracesV1(logger *zap.Logger, payload *idx.InternalTracerPayload, req *http.Request) (traces ptrace.Traces, err error) {
	// Payloads that were not decoded from the wire (agent payloads) can reference strings that do not exist.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid v1 tracer payload: %v", r)
		}
	}()

	strs := payload.Strings
	sharedAttributes := pcommon.NewMap()
	for k, v := range map[string]string{
		string(conventions.ContainerIDKey):               payload.ContainerID(),
		string(conventions.TelemetrySDKLanguageKey):      payload.LanguageName(),
		string(conventions.ProcessRuntimeVersionKey):     payload.LanguageVersion(),
		string(conventions.DeploymentEnvironmentNameKey): payload.Env(),
		string(conventions.HostNameKey):                  payload.Hostname(),
		string(conventions.ServiceVersionKey):            payload.AppVersion(),
		string(conventions.TelemetrySDKNameKey):          "Datadog",
		string(conventions.TelemetrySDKVersionKey):       payload.TracerVersion(),
	} {
		if v != "" {
			sharedAttributes.PutStr(k, v)
		}
	}
	putV1Attributes(sharedAttributes, payload.Attributes, strs, true)
	upsertHeadersAttributes(req, sharedAttributes)

	// service.name is a resource attribute, so spans are grouped by service.
	groupByService := make(map[string]ptrace.SpanSlice)
	for _, chunk := range payload.Chunks {
		traceID := chunkTraceID(chunk)
		for _, span := range chunk.Spans {
			service := span.Service()
			// Restore base service name as the service name, like for the older payload formats.
			baseService, hasBaseService := span.GetAttributeAsString("_dd.base_service")

			if hasBaseService {
				service, baseService = baseService, service
			}
			slice, exist := groupByService[service]
			if !exist {
				slice = ptrace.NewSpanSlice()
				groupByService[service] = slice
			}
			newSpan := slice.AppendEmpty()

			newSpan.SetTraceID(traceID)
			newSpan.SetSpanID(uInt64ToSpanID(span.SpanID()))
			newSpan.SetParentSpanID(uInt64ToSpanID(span.ParentID()))
			newSpan.SetStartTimestamp(pcommon.Timestamp(span.Start()))
			newSpan.SetEndTimestamp(pcommon.Timestamp(span.Start() + span.Duration()))
			newSpan.SetName(span.Name())
			newSpan.Status().SetCode(ptrace.StatusCodeOk)
			if span.Error() {
				newSpan.Status().SetCode(ptrace.StatusCodeError)
			}

			attrs := newSpan.Attributes()
			attrs.PutStr("dd.span.Resource", span.Resource())
			if chunk.Priority != int32(ddsampler.PriorityNone) {
				attrs.PutStr("sampling.priority", fmt.Sprintf("%f", float64(chunk.Priority)))
			}
			attrs.PutStr(attributeDatadogSpanID, strconv.FormatUint(span.SpanID(), 10))
			attrs.PutStr(attributeDatadogTraceID, strconv.FormatUint(binary.BigEndian.Uint64(traceID[8:]), 10))
			if hasBaseService {
				// Preserve original per-span service name so the DD exporter can recover it via span-level
				// service.name precedence.
				attrs.PutStr(string(conventions.ServiceNameKey), baseService)
			}
			// env, version and component are fields of a v1 span.
			for k, v := range map[string]string{
				string(conventions.DeploymentEnvironmentNameKey): span.Env(),
				string(conventions.ServiceVersionKey):            span.Version(),
				"component":                                      span.Component(),
			} {
				if v != "" {
					attrs.PutStr(k, v)
				}
			}
			putV1Attributes(attrs, span.Attributes(), strs, true)

			_, converted := span.GetAttributeAsString(attributeConvertedV1)
			switch span.Kind() {
			case idx.SpanKind_SPAN_KIND_SERVER:
				newSpan.SetKind(ptrace.SpanKindServer)
			case idx.SpanKind_SPAN_KIND_CLIENT:
				newSpan.SetKind(ptrace.SpanKindClient)
			case idx.SpanKind_SPAN_KIND_PRODUCER:
				newSpan.SetKind(ptrace.SpanKindProducer)
			case idx.SpanKind_SPAN_KIND_CONSUMER:
				newSpan.SetKind(ptrace.SpanKindConsumer)
			case idx.SpanKind_SPAN_KIND_INTERNAL:
				newSpan.SetKind(ptrace.SpanKindInternal)
				// The agent sets internal on converted spans that have no span.kind, so the span type is more telling.
				if kind := kindFromType(span.Type()); converted && kind != ptrace.SpanKindUnspecified {
					newSpan.SetKind(kind)
				}
			default:
				newSpan.SetKind(kindFromType(span.Type()))
			}

			// For client/producer/consumer spans, if we have `peer.hostname`, and `server.address` is unset, set
			// `server.address` to `peer.hostname`.
			if newSpan.Kind() == ptrace.SpanKindClient ||
				newSpan.Kind() == ptrace.SpanKindProducer ||
				newSpan.Kind() == ptrace.SpanKindConsumer {
				if _, ok := attrs.Get("server.address"); !ok {
					if val, ok := span.GetAttributeAsString("peer.hostname"); ok {
						attrs.PutStr("server.address", val)
					}
				}
			}

			for _, l := range span.Links() {
				link := newSpan.Links().AppendEmpty()
				if id := l.TraceID(); len(id) == 16 {
					link.SetTraceID(pcommon.TraceID(id))
				}
				link.SetSpanID(uInt64ToSpanID(l.SpanID()))
				link.TraceState().FromRaw(l.Tracestate())
				link.SetFlags(l.Flags())
				putV1Attributes(link.Attributes(), l.Attributes(), strs, false)
			}
			for _, e := range span.Events() {
				event := newSpan.Events().AppendEmpty()
				event.SetName(e.Name())
				event.SetTimestamp(pcommon.Timestamp(e.Time()))
				putV1Attributes(event.Attributes(), e.Attributes(), strs, false)
			}

			// Tracers send links and events as JSON tags when the agent does not advertise native support in /info,
			// and the agent keeps them as tags when it converts spans to v1.
			parseLinksAndEventsTags(logger, newSpan, len(span.Links()) > 0, len(span.Events()) > 0)

			var errCode int32
			if span.Error() {
				errCode = 1
			}
			// Some spans need specific processing (http, db, grpc...)
			processSpanByName(spanView{name: span.Name(), resource: span.Resource(), err: errCode, meta: span.GetAttributeAsString}, &newSpan)
		}
	}

	results := ptrace.NewTraces()
	for service, spans := range groupByService {
		rs := results.ResourceSpans().AppendEmpty()
		rs.SetSchemaUrl(conventions.SchemaURL)
		sharedAttributes.CopyTo(rs.Resource().Attributes())
		rs.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), service)

		in := rs.ScopeSpans().AppendEmpty()
		in.Scope().SetName("Datadog")
		in.Scope().SetVersion(payload.TracerVersion())
		spans.CopyTo(in.Spans())
	}
	return results, nil
}

func kindFromType(spanType string) ptrace.SpanKind {
	switch spanType {
	case "web":
		return ptrace.SpanKindServer
	case "http":
		return ptrace.SpanKindClient
	default:
		return ptrace.SpanKindUnspecified
	}
}

// parseLinksAndEventsTags turns the span links and events that are in JSON tags into OTel span links and events, like
// the v0.x translation does. The tags are removed from the span. Native links and events take precedence.
func parseLinksAndEventsTags(logger *zap.Logger, span ptrace.Span, hasNativeLinks, hasNativeEvents bool) {
	attrs := span.Attributes()
	if raw, ok := attrs.Get("_dd.span_links"); ok {
		if !hasNativeLinks {
			if err := metaSpanLinks(raw.AsString(), span.Links()); err != nil {
				logger.Error("error parsing _dd.span_links", zap.Error(err))
			}
		}
		attrs.Remove("_dd.span_links")
	}
	if raw, ok := attrs.Get("events"); ok {
		if !hasNativeEvents {
			if err := metaSpanEvents(raw.AsString(), span.Events()); err != nil {
				logger.Error("error parsing span events", zap.Error(err))
			}
		}
		attrs.Remove("events")
	}
	attrs.Remove("_dd.span_events.has_exception")
}

// chunkTraceID returns the 128-bit trace id of a chunk. A shorter id is treated as if the missing high-order bytes were zero.
func chunkTraceID(chunk *idx.InternalTraceChunk) pcommon.TraceID {
	var traceID pcommon.TraceID
	copy(traceID[len(traceID)-min(len(chunk.TraceID), len(traceID)):], chunk.TraceID)
	return traceID
}

// putV1Attributes adds v1 attributes to dst with their native types. If translateKeys is set, the Datadog keys are
// renamed to their OTel equivalent.
func putV1Attributes(dst pcommon.Map, attrs map[uint32]*idx.AnyValue, strs *idx.StringTable, translateKeys bool) {
	for k, v := range attrs {
		key := strs.Get(k)
		if translateKeys {
			key = translateDatadogKeyToOTel(key)
		}
		if key != "" && v != nil {
			putV1Value(dst.PutEmpty(key), v, strs)
		}
	}
}

// putV1Value sets dst to the OTel equivalent of a v1 attribute value.
func putV1Value(dst pcommon.Value, in *idx.AnyValue, strs *idx.StringTable) {
	switch v := in.Value.(type) {
	case *idx.AnyValue_StringValueRef:
		dst.SetStr(strs.Get(v.StringValueRef))
	case *idx.AnyValue_BoolValue:
		dst.SetBool(v.BoolValue)
	case *idx.AnyValue_IntValue:
		dst.SetInt(v.IntValue)
	case *idx.AnyValue_DoubleValue:
		dst.SetDouble(v.DoubleValue)
	case *idx.AnyValue_BytesValue:
		dst.SetEmptyBytes().FromRaw(v.BytesValue)
	case *idx.AnyValue_ArrayValue:
		slice := dst.SetEmptySlice()
		for _, e := range v.ArrayValue.GetValues() {
			putV1Value(slice.AppendEmpty(), e, strs)
		}
	case *idx.AnyValue_KeyValueList:
		m := dst.SetEmptyMap()
		for _, kv := range v.KeyValueList.GetKeyValues() {
			putV1Value(m.PutEmpty(strs.Get(kv.Key)), kv.Value, strs)
		}
	}
}
