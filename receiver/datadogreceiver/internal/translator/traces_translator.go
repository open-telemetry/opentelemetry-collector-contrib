// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	ddsampler "github.com/DataDog/datadog-agent/pkg/trace/sampler"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator/header"
)

const (
	datadogSpanKindKey = "span.kind"
	// The datadog trace id
	//
	// Type: string
	// Requirement Level: Optional
	// Examples: '6249785623524942554'
	attributeDatadogTraceID = "datadog.trace.id"
	// The datadog span id
	//
	// Type: string
	// Requirement Level: Optional
	// Examples: '228114450199004348'
	attributeDatadogSpanID = "datadog.span.id"
)

var spanProcessor = map[string]func(*pb.Span, *ptrace.Span){
	// HTTP
	"servlet.request": processHTTPSpan,
	"http.request":    processHTTPSpan,
	"web.request":     processHTTPSpan,

	// Internal
	"spring.handler": processInternalSpan,

	// Database
	"postgresql.query": processDBSpan,
	"redis.query":      processDBSpan,

	// GRPC
	"grpc.server": processGRPCSpan,
	"grpc.client": processGRPCSpan,

	// AWS
	"aws.request": processAWSSdkSpan,
	"aws.command": processAWSSdkSpan,
}

func upsertHeadersAttributes(req *http.Request, attrs pcommon.Map) {
	if ddTracerVersion := req.Header.Get(header.TracerVersion); ddTracerVersion != "" {
		attrs.PutStr(string(conventions.TelemetrySDKVersionKey), "Datadog-"+ddTracerVersion)
	}
	if ddTracerLang := req.Header.Get(header.Lang); ddTracerLang != "" {
		otelLang := ddTracerLang
		if ddTracerLang == ".NET" {
			otelLang = "dotnet"
		}
		attrs.PutStr(string(conventions.TelemetrySDKLanguageKey), otelLang)
	}
}

// traceID64to128 reconstructs the 128 bits TraceID, if available or cached.
//
// Datadog traces split a 128 bits trace id in two parts: TraceID and Tags._dd_p_tid. This happens if the
// instrumented service received a TraceContext from an OTel instrumented service. When it happens, we need
// to concatenate the two into newSpan.TraceID.
// The traceIDCache keeps track of the TraceIDs we process as only the first span has the upper 64 bits from the 128
// bits trace ID.
//
// Note: This may not be resilient to related spans being flushed separately in datadog's tracing libraries.
//
//	It might also not work if multiple datadog instrumented services are chained.
//
// This is currently gated by a feature gate (receiver.datadogreceiver.Enable128BitTraceID). If we don't get a cache
// in traceIDCache, we don't enable this behavior.
func traceID64to128(span *pb.Span, traceIDCache *lru.Cache[uint64, pcommon.TraceID]) (pcommon.TraceID, error) {
	if val, ok := traceIDCache.Get(span.TraceID); ok {
		return val, nil
	} else if val, ok := span.Meta["_dd.p.tid"]; ok {
		tid, err := strconv.ParseUint(val, 16, 64)
		if err != nil {
			return pcommon.TraceID{}, fmt.Errorf("error converting %s to uint64", val)
		}
		traceID := uInt64ToTraceID(tid, span.TraceID)
		// Child spans don't have _dd.p.tid, we cache it.
		traceIDCache.Add(span.TraceID, traceID)

		return traceID, nil
	}
	return pcommon.TraceID{}, nil
}

func processInternalSpan(span *pb.Span, newSpan *ptrace.Span) {
	newSpan.SetName(span.Resource)
	newSpan.SetKind(ptrace.SpanKindInternal)
}

func processHTTPSpan(span *pb.Span, newSpan *ptrace.Span) {
	// https://opentelemetry.io/docs/specs/semconv/http/http-spans/#name
	// We assume that http.route coming from datadog is low cardinality
	if val, ok := span.Meta["http.method"]; ok {
		if suffix, ok := span.Meta["http.route"]; ok {
			newSpan.SetName(val + " " + suffix)
		} else {
			newSpan.SetName(val)
		}
	}
}

func processDBSpan(span *pb.Span, newSpan *ptrace.Span) {
	// references:
	// https://github.com/DataDog/documentation/blob/master/content/en/tracing/guide/ignoring_apm_resources.md#database
	// https://opentelemetry.io/docs/specs/semconv/database/database-spans/#name
	if val, ok := span.Meta["db.query.summary"]; ok {
		newSpan.SetName(val)
	} else {
		if val, ok = span.Meta["db.operation"]; ok {
			newSpan.SetName(val)
			suffix := cmp.Or(span.Meta["db.instance"], span.Meta["db.namespace"], span.Meta["peer.hostname"])
			if suffix != "" {
				newSpan.SetName(val + " " + suffix)
			}
		} else if val, ok = span.Meta["db.type"]; ok {
			newSpan.SetName(val)
		}
	}
}

func processGRPCSpan(span *pb.Span, newSpan *ptrace.Span) {
	// references:
	// https://github.com/DataDog/documentation/blob/master/content/en/tracing/guide/ignoring_apm_resources.md#remote-procedure-calls
	// https://opentelemetry.io/docs/specs/semconv/rpc/rpc-spans/

	// ddSpan.Attributes["grpc.status.code"] contains the gRPC status code name (eg "OK")
	// not the numeric value (eg "0")
	// it's ddSpan.error that indicates holds the gRPC status code numeric value
	newSpan.Attributes().PutStr(string(conventions.RPCSystemNameKey), conventions.RPCSystemNameGRPC.Value.AsString())
	newSpan.Attributes().PutStr(string(conventions.RPCResponseStatusCodeKey), strconv.Itoa(int(span.GetError())))

	method := ""
	service := ""
	if rpcMethod, ok := span.Meta[("rpc.method")]; ok {
		// "rpc.method" is used by dd-trace-rb, check dd-trace-php
		method = rpcMethod
		if rpcService, ok := span.Meta[("rpc.service")]; ok {
			service = rpcService
		}
	} else if grpcFullMethod, ok := span.Meta[("rpc.grpc.full_method")]; ok {
		// "rpc.grpc.full_method" is used by dd-trace-go & dd-trace-rb, they also set span.Resource to the full method name
		// format: /$package.$service/$method
		grpcFullMethodElements := strings.SplitN(strings.TrimPrefix(grpcFullMethod, "/"), "/", 2)
		if len(grpcFullMethodElements) == 2 {
			method = grpcFullMethodElements[1]
			service = grpcFullMethodElements[0]
		}
	} else if grpcMethod, ok := span.Meta[("grpc.method.name")]; ok {
		// format: /$package.$service/$method
		// "grpc.method.name" is used by dd-trace-dotnet dd-trace-python & dd-trace-go
		grpcMethodElements := strings.SplitN(strings.TrimPrefix(grpcMethod, "/"), "/", 2)
		if len(grpcMethodElements) == 2 {
			method = grpcMethodElements[1]
			service = grpcMethodElements[0]
		} else if len(grpcMethodElements) == 1 {
			// unexpected format
			method = ""
			service = grpcMethodElements[0]
		}
	} else {
		// resource is used by dd-trace-java
		ddResource := strings.SplitN(strings.TrimPrefix(span.Resource, "/"), "/", 2)
		if len(ddResource) == 2 {
			method = ddResource[1]
			service = ddResource[0]
		}
	}

	spanName := ""
	if method != "" {
		newSpan.Attributes().PutStr(string(conventions.RPCMethodKey), method)
		if !metadata.ReceiverDatadogreceiverDontEmitDeprecatedRPCServiceAttrFeatureGate.IsEnabled() {
			newSpan.Attributes().PutStr("rpc.service", service)
		}
		spanName = service + "/" + method
	} else if service != "" {
		if !metadata.ReceiverDatadogreceiverDontEmitDeprecatedRPCServiceAttrFeatureGate.IsEnabled() {
			newSpan.Attributes().PutStr("rpc.service", service)
		}
		spanName = service
	}
	if spanName != "" {
		newSpan.SetName(spanName)
	}
}

func processAWSSdkSpan(span *pb.Span, newSpan *ptrace.Span) {
	// https://opentelemetry.io/docs/specs/semconv/cloud-providers/aws-sdk/
	newSpan.Attributes().PutStr(string(conventions.RPCSystemNameKey), "aws-api")
	if service, ok := span.Meta[("aws.service")]; ok {
		if operation, ok := span.Meta[("aws.operation")]; ok {
			newSpan.SetName(service + "/" + operation)
		} else {
			newSpan.SetName(service)
		}
	}
}

func processSpanByName(span *pb.Span, newSpan *ptrace.Span) {
	if processor, ok := spanProcessor[span.Name]; ok {
		processor(span, newSpan)
	}
}

func traceChunkSamplingPriority(traceChunk *pb.TraceChunk) (float64, bool) {
	if traceChunk == nil {
		return 0, false
	}
	if traceChunk.Priority != int32(ddsampler.PriorityNone) {
		return float64(traceChunk.Priority), true
	}
	for _, span := range traceChunk.GetSpans() {
		if samplingPriority, ok := span.Metrics["_sampling_priority_v1"]; ok {
			return samplingPriority, true
		}
	}
	return 0, false
}

func ToTraces(logger *zap.Logger, payload *pb.TracerPayload, req *http.Request, traceIDCache *lru.Cache[uint64, pcommon.TraceID]) (ptrace.Traces, error) {
	sharedAttributes := pcommon.NewMap()
	for k, v := range map[string]string{
		string(conventions.ContainerIDKey):               payload.ContainerID,
		string(conventions.TelemetrySDKLanguageKey):      payload.LanguageName,
		string(conventions.ProcessRuntimeVersionKey):     payload.LanguageVersion,
		string(conventions.DeploymentEnvironmentNameKey): payload.Env,
		string(conventions.HostNameKey):                  payload.Hostname,
		string(conventions.ServiceVersionKey):            payload.AppVersion,
		string(conventions.TelemetrySDKNameKey):          "Datadog",
		string(conventions.TelemetrySDKVersionKey):       payload.TracerVersion,
	} {
		if v != "" {
			sharedAttributes.PutStr(k, v)
		}
	}

	for k, v := range payload.Tags {
		if k = translateDatadogKeyToOTel(k); v != "" {
			sharedAttributes.PutStr(k, v)
		}
	}

	upsertHeadersAttributes(req, sharedAttributes)

	// Creating a map of service spans to slices
	// since the expectation is that `service.name`
	// is added as a resource attribute in most systems
	// now instead of being a span level attribute.
	groupByService := make(map[string]ptrace.SpanSlice)

	for _, traceChunk := range payload.GetChunks() {
		samplingPriority, hasSamplingPriority := traceChunkSamplingPriority(traceChunk)
		for _, span := range traceChunk.GetSpans() {
			// Restore base service name as the service name.
			// Without this, internal spans such as postgresql queries have a service.name set to postgresql
			if val, ok := span.Meta["_dd.base_service"]; ok {
				// Preserve original per-span service name so the DD exporter
				// can recover it via span-level service.name precedence
				span.Meta["service.name"] = span.Service
				span.Service = val
			}
			slice, exist := groupByService[span.Service]
			if !exist {
				slice = ptrace.NewSpanSlice()
				groupByService[span.Service] = slice
			}
			newSpan := slice.AppendEmpty()

			setSpanLinks(span, newSpan.Links(), logger)
			setSpanEvents(span, newSpan.Events(), logger)

			newSpan.SetTraceID(uInt64ToTraceID(0, span.TraceID))
			// Try to get the 128-bit traceID, if available.
			if traceIDCache != nil {
				traceID, err := traceID64to128(span, traceIDCache)
				if err != nil {
					logger.Error("error converting trace ID to 128", zap.Error(err))
				}
				if !traceID.IsEmpty() {
					newSpan.SetTraceID(traceID)
				}
			}

			newSpan.SetSpanID(uInt64ToSpanID(span.SpanID))
			newSpan.SetStartTimestamp(pcommon.Timestamp(span.Start))
			newSpan.SetEndTimestamp(pcommon.Timestamp(span.Start + span.Duration))
			newSpan.SetParentSpanID(uInt64ToSpanID(span.ParentID))
			newSpan.SetName(span.Name)
			newSpan.Status().SetCode(ptrace.StatusCodeOk)
			newSpan.Attributes().PutStr("dd.span.Resource", span.Resource)
			if hasSamplingPriority {
				newSpan.Attributes().PutStr("sampling.priority", fmt.Sprintf("%f", samplingPriority))
			}
			if span.Error > 0 {
				newSpan.Status().SetCode(ptrace.StatusCodeError)
			}
			newSpan.Attributes().PutStr(attributeDatadogSpanID, strconv.FormatUint(span.SpanID, 10))
			newSpan.Attributes().PutStr(attributeDatadogTraceID, strconv.FormatUint(span.TraceID, 10))
			for k, v := range span.GetMeta() {
				if k = translateDatadogKeyToOTel(k); k != "" {
					newSpan.Attributes().PutStr(k, v)
				}
			}
			for k, v := range span.GetMetrics() {
				if k = translateDatadogKeyToOTel(k); k != "" {
					newSpan.Attributes().PutDouble(k, v)
				}
			}

			switch span.Meta[datadogSpanKindKey] {
			case "server":
				newSpan.SetKind(ptrace.SpanKindServer)
			case "client":
				newSpan.SetKind(ptrace.SpanKindClient)
			case "producer":
				newSpan.SetKind(ptrace.SpanKindProducer)
			case "consumer":
				newSpan.SetKind(ptrace.SpanKindConsumer)
			case "internal":
				newSpan.SetKind(ptrace.SpanKindInternal)
			default:
				switch span.Type {
				case "web":
					newSpan.SetKind(ptrace.SpanKindServer)
				case "http":
					newSpan.SetKind(ptrace.SpanKindClient)
				default:
					newSpan.SetKind(ptrace.SpanKindUnspecified)
				}
			}

			// For client/producer/consumer spans, if we have `peer.hostname`, and `server.address` is unset, set
			// `server.address` to `peer.hostname`.
			if newSpan.Kind() == ptrace.SpanKindClient ||
				newSpan.Kind() == ptrace.SpanKindProducer ||
				newSpan.Kind() == ptrace.SpanKindConsumer {
				if _, ok := newSpan.Attributes().Get("server.address"); !ok {
					if val, ok := span.Meta["peer.hostname"]; ok {
						newSpan.Attributes().PutStr("server.address", val)
					}
				}
			}

			// Some spans need specific processing (http, db, grpc...)
			processSpanByName(span, &newSpan)
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
		in.Scope().SetVersion(payload.TracerVersion)
		spans.CopyTo(in.Spans())
	}

	return results, nil
}

// setSpanLinks populates the OTel span links from a Datadog span. Datadog tracers send links in the
// native span_links field, which carries the full 128-bit trace id (trace_id + trace_id_high) and
// the W3C trace flags; spans converted from OTLP by the Datadog agent instead carry a _dd.span_links
// meta JSON string. Native links take precedence. The meta key is always removed so it does not also
// surface as a raw span attribute.
func setSpanLinks(span *pb.Span, dest ptrace.SpanLinkSlice, logger *zap.Logger) {
	raw, hasMeta := span.Meta["_dd.span_links"]
	delete(span.Meta, "_dd.span_links")

	if len(span.SpanLinks) > 0 {
		for _, l := range span.SpanLinks {
			link := dest.AppendEmpty()
			link.SetTraceID(uInt64ToTraceID(l.TraceIDHigh, l.TraceID))
			link.SetSpanID(uInt64ToSpanID(l.SpanID))
			link.TraceState().FromRaw(l.Tracestate)
			link.SetFlags(l.Flags)
			for k, v := range l.Attributes {
				link.Attributes().PutStr(k, v)
			}
		}
		return
	}

	if hasMeta {
		if err := metaSpanLinks(raw, dest); err != nil {
			logger.Error("error parsing _dd.span_links", zap.Error(err))
		}
	}
}

// ddSpanLink mirrors an entry of the _dd.span_links meta JSON array. Datadog encodes the ids either
// as decimal numbers (dd-trace tracers: trace_id + optional trace_id_high + span_id) or as hex
// strings (OTLP spans converted by the Datadog agent: 32-char trace_id, 16-char span_id), so the ids
// are kept raw and decoded to accept both encodings.
type ddSpanLink struct {
	TraceID     json.RawMessage `json:"trace_id"`
	TraceIDHigh json.RawMessage `json:"trace_id_high"`
	SpanID      json.RawMessage `json:"span_id"`
	Tracestate  string          `json:"tracestate"`
	Flags       uint32          `json:"flags"`
	Attributes  map[string]any  `json:"attributes"`
}

func metaSpanLinks(raw string, dest ptrace.SpanLinkSlice) error {
	var links []ddSpanLink
	if err := json.Unmarshal([]byte(raw), &links); err != nil {
		return err
	}

	for _, l := range links {
		link := dest.AppendEmpty()

		traceID, err := decodeLinkTraceID(l.TraceID, l.TraceIDHigh)
		if err != nil {
			return err
		}
		link.SetTraceID(traceID)

		spanID, err := decodeLinkSpanID(l.SpanID)
		if err != nil {
			return err
		}
		link.SetSpanID(spanID)

		link.TraceState().FromRaw(l.Tracestate)
		link.SetFlags(l.Flags)
		if err := link.Attributes().FromRaw(l.Attributes); err != nil {
			return err
		}
	}

	return nil
}

func decodeLinkTraceID(traceID, traceIDHigh json.RawMessage) (pcommon.TraceID, error) {
	if isJSONString(traceID) {
		var s string
		if err := json.Unmarshal(traceID, &s); err != nil {
			return pcommon.TraceID{}, err
		}
		raw, err := oteltrace.TraceIDFromHex(s)
		if err != nil {
			return pcommon.TraceID{}, fmt.Errorf("error converting trace id (%s) from hex: %w", s, err)
		}
		return pcommon.TraceID(raw), nil
	}

	low, err := rawUint64(traceID)
	if err != nil {
		return pcommon.TraceID{}, err
	}
	var high uint64
	if len(traceIDHigh) > 0 {
		if high, err = rawUint64(traceIDHigh); err != nil {
			return pcommon.TraceID{}, err
		}
	}
	return uInt64ToTraceID(high, low), nil
}

func decodeLinkSpanID(spanID json.RawMessage) (pcommon.SpanID, error) {
	if isJSONString(spanID) {
		var s string
		if err := json.Unmarshal(spanID, &s); err != nil {
			return pcommon.SpanID{}, err
		}
		raw, err := oteltrace.SpanIDFromHex(s)
		if err != nil {
			return pcommon.SpanID{}, fmt.Errorf("error converting span id (%s) from hex: %w", s, err)
		}
		return pcommon.SpanID(raw), nil
	}

	id, err := rawUint64(spanID)
	if err != nil {
		return pcommon.SpanID{}, err
	}
	return uInt64ToSpanID(id), nil
}

func isJSONString(raw json.RawMessage) bool {
	return len(raw) > 0 && raw[0] == '"'
}

func rawUint64(raw json.RawMessage) (uint64, error) {
	var n uint64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, err
	}
	return n, nil
}

// setSpanEvents populates OTel span events from a Datadog span. Datadog tracers send events in the
// native span_events field when the endpoint advertises support; otherwise (and for older tracers)
// they arrive as an "events" meta JSON string. Native events take precedence. The events meta keys
// are removed so they do not also surface as raw span attributes.
func setSpanEvents(span *pb.Span, dest ptrace.SpanEventSlice, logger *zap.Logger) {
	raw, hasMeta := span.Meta["events"]
	delete(span.Meta, "events")
	delete(span.Meta, "_dd.span_events.has_exception")

	if len(span.SpanEvents) > 0 {
		for _, e := range span.SpanEvents {
			event := dest.AppendEmpty()
			event.SetName(e.Name)
			event.SetTimestamp(pcommon.Timestamp(e.TimeUnixNano))
			for k, v := range e.Attributes {
				putAttributeAnyValue(event.Attributes(), k, v)
			}
		}
		return
	}

	if hasMeta {
		if err := metaSpanEvents(raw, dest); err != nil {
			logger.Error("error parsing span events", zap.Error(err))
		}
	}
}

// ddSpanEvent mirrors an entry of the "events" meta JSON array emitted by tracers that do not use the
// native span_events field.
type ddSpanEvent struct {
	TimeUnixNano uint64         `json:"time_unix_nano"`
	Name         string         `json:"name"`
	Attributes   map[string]any `json:"attributes"`
}

func metaSpanEvents(raw string, dest ptrace.SpanEventSlice) error {
	var events []ddSpanEvent
	if err := json.Unmarshal([]byte(raw), &events); err != nil {
		return err
	}

	for _, e := range events {
		event := dest.AppendEmpty()
		event.SetName(e.Name)
		event.SetTimestamp(pcommon.Timestamp(e.TimeUnixNano))
		if err := event.Attributes().FromRaw(e.Attributes); err != nil {
			return err
		}
	}

	return nil
}

func putAttributeAnyValue(attrs pcommon.Map, key string, v *pb.AttributeAnyValue) {
	if v == nil {
		return
	}
	switch v.Type {
	case pb.AttributeAnyValue_STRING_VALUE:
		attrs.PutStr(key, v.StringValue)
	case pb.AttributeAnyValue_BOOL_VALUE:
		attrs.PutBool(key, v.BoolValue)
	case pb.AttributeAnyValue_INT_VALUE:
		attrs.PutInt(key, v.IntValue)
	case pb.AttributeAnyValue_DOUBLE_VALUE:
		attrs.PutDouble(key, v.DoubleValue)
	case pb.AttributeAnyValue_ARRAY_VALUE:
		if v.ArrayValue == nil {
			return
		}
		slice := attrs.PutEmptySlice(key)
		for _, av := range v.ArrayValue.Values {
			if av == nil {
				continue
			}
			switch av.Type {
			case pb.AttributeArrayValue_STRING_VALUE:
				slice.AppendEmpty().SetStr(av.StringValue)
			case pb.AttributeArrayValue_BOOL_VALUE:
				slice.AppendEmpty().SetBool(av.BoolValue)
			case pb.AttributeArrayValue_INT_VALUE:
				slice.AppendEmpty().SetInt(av.IntValue)
			case pb.AttributeArrayValue_DOUBLE_VALUE:
				slice.AppendEmpty().SetDouble(av.DoubleValue)
			}
		}
	}
}

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func GetBuffer() *bytes.Buffer {
	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

func PutBuffer(buffer *bytes.Buffer) {
	bufferPool.Put(buffer)
}

func HandleTracesPayload(req *http.Request) (tp []*pb.TracerPayload, err error) {
	var tracerPayloads []*pb.TracerPayload

	defer func() {
		_, errs := io.Copy(io.Discard, req.Body)
		err = errors.Join(err, errs, req.Body.Close())
	}()

	switch {
	case strings.HasPrefix(req.URL.Path, "/v0.7"):
		buf := GetBuffer()
		defer PutBuffer(buf)
		if _, err = io.Copy(buf, req.Body); err != nil {
			return nil, err
		}
		var tracerPayload pb.TracerPayload
		if _, err = tracerPayload.UnmarshalMsg(buf.Bytes()); err != nil {
			return nil, err
		}

		tracerPayloads = append(tracerPayloads, &tracerPayload)
	case strings.HasPrefix(req.URL.Path, "/v0.5"):
		buf := GetBuffer()
		defer PutBuffer(buf)
		if _, err = io.Copy(buf, req.Body); err != nil {
			return nil, err
		}
		var traces pb.Traces

		err = traces.UnmarshalMsgDictionary(buf.Bytes())
		if err != nil {
			return nil, err
		}

		traceChunks := traceChunksFromTraces(traces)
		appVersion := appVersionFromTraceChunks(traceChunks)

		tracerPayload := &pb.TracerPayload{
			LanguageName:    req.Header.Get(header.Lang),
			LanguageVersion: req.Header.Get(header.LangVersion),
			TracerVersion:   req.Header.Get(header.TracerVersion),
			ContainerID:     req.Header.Get(header.ContainerID),
			Chunks:          traceChunks,
			AppVersion:      appVersion,
		}
		tracerPayloads = append(tracerPayloads, tracerPayload)

	case strings.HasPrefix(req.URL.Path, "/v0.1"):
		var spans []pb.Span
		err = json.NewDecoder(req.Body).Decode(&spans)
		if err != nil {
			return nil, err
		}
		tracerPayload := &pb.TracerPayload{
			LanguageName:    req.Header.Get(header.Lang),
			LanguageVersion: req.Header.Get(header.LangVersion),
			TracerVersion:   req.Header.Get(header.TracerVersion),
			Chunks:          traceChunksFromSpans(spans),
		}
		tracerPayloads = append(tracerPayloads, tracerPayload)
	case strings.HasPrefix(req.URL.Path, "/api/v0.2"):
		buf := GetBuffer()
		defer PutBuffer(buf)
		if _, err = io.Copy(buf, req.Body); err != nil {
			return nil, err
		}

		var agentPayload pb.AgentPayload
		err = proto.Unmarshal(buf.Bytes(), &agentPayload)
		if err != nil {
			return nil, err
		}

		return agentPayload.TracerPayloads, err

	default:
		var traces pb.Traces
		err = decodeRequest(req, &traces)
		if err != nil {
			return nil, err
		}
		traceChunks := traceChunksFromTraces(traces)
		appVersion := appVersionFromTraceChunks(traceChunks)
		tracerPayload := &pb.TracerPayload{
			LanguageName:    req.Header.Get(header.Lang),
			LanguageVersion: req.Header.Get(header.LangVersion),
			TracerVersion:   req.Header.Get(header.TracerVersion),
			Chunks:          traceChunks,
			AppVersion:      appVersion,
		}
		tracerPayloads = append(tracerPayloads, tracerPayload)
	}

	return tracerPayloads, nil
}

func decodeRequest(req *http.Request, dest *pb.Traces) (err error) {
	switch mediaType := getMediaType(req); mediaType {
	case "application/msgpack":
		buf := GetBuffer()
		defer PutBuffer(buf)
		_, err = io.Copy(buf, req.Body)
		if err != nil {
			return err
		}
		_, err = dest.UnmarshalMsg(buf.Bytes())
		return err
	case "application/json", "text/json", "":
		err = json.NewDecoder(req.Body).Decode(&dest)
		return err
	default:
		// do our best
		if err1 := json.NewDecoder(req.Body).Decode(&dest); err1 != nil {
			buf := GetBuffer()
			defer PutBuffer(buf)
			_, err2 := io.Copy(buf, req.Body)
			if err2 != nil {
				return err2
			}
			_, err2 = dest.UnmarshalMsg(buf.Bytes())
			return err2
		}
		return nil
	}
}

func traceChunksFromSpans(spans []pb.Span) []*pb.TraceChunk {
	traceChunks := []*pb.TraceChunk{}
	byID := make(map[uint64][]*pb.Span)
	for i := range spans {
		byID[spans[i].TraceID] = append(byID[spans[i].TraceID], &spans[i])
	}
	for _, t := range byID {
		traceChunks = append(traceChunks, &pb.TraceChunk{
			Priority: int32(ddsampler.PriorityNone),
			Spans:    t,
		})
	}
	return traceChunks
}

func traceChunksFromTraces(traces pb.Traces) []*pb.TraceChunk {
	traceChunks := make([]*pb.TraceChunk, 0, len(traces))
	for _, trace := range traces {
		traceChunks = append(traceChunks, &pb.TraceChunk{
			Priority: int32(ddsampler.PriorityNone),
			Spans:    trace,
		})
	}

	return traceChunks
}

func appVersionFromTraceChunks(traces []*pb.TraceChunk) string {
	appVersion := ""
	for _, trace := range traces {
		for _, span := range trace.Spans {
			if span != nil && span.Meta["version"] != "" {
				appVersion = span.Meta["version"]
				return appVersion
			}
		}
	}
	return appVersion
}

func getMediaType(req *http.Request) string {
	mt, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		return "application/json"
	}
	return mt
}

func uInt64ToTraceID(high, low uint64) pcommon.TraceID {
	traceID := [16]byte{}
	binary.BigEndian.PutUint64(traceID[:8], high)
	binary.BigEndian.PutUint64(traceID[8:], low)
	return traceID
}

func uInt64ToSpanID(id uint64) pcommon.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint64(spanID[:], id)
	return spanID
}
