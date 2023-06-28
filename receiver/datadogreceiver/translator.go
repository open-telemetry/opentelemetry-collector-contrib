// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	"go.uber.org/multierr"
)

const (
	datadogSpanKindKey = "span.kind"
)

func upsertHeadersAttributes(req *http.Request, attrs pcommon.Map) {
	if ddTracerVersion := req.Header.Get("Datadog-Meta-Tracer-Version"); ddTracerVersion != "" {
		attrs.PutStr(semconv.AttributeTelemetrySDKVersion, "Datadog-"+ddTracerVersion)
	}
	if ddTracerLang := req.Header.Get("Datadog-Meta-Lang"); ddTracerLang != "" {
		attrs.PutStr(semconv.AttributeTelemetrySDKLanguage, ddTracerLang)
	}
}

func toTraces(payload *pb.TracerPayload, req *http.Request) ptrace.Traces {
	var traces pb.Traces
	for _, p := range payload.GetChunks() {
		traces = append(traces, p.GetSpans())
	}
	sharedAttributes := pcommon.NewMap()
	for k, v := range map[string]string{
		semconv.AttributeContainerID:           payload.ContainerID,
		semconv.AttributeTelemetrySDKLanguage:  payload.LanguageName,
		semconv.AttributeProcessRuntimeVersion: payload.LanguageVersion,
		semconv.AttributeDeploymentEnvironment: payload.Env,
		semconv.AttributeHostName:              payload.Hostname,
		semconv.AttributeServiceVersion:        payload.AppVersion,
		semconv.AttributeTelemetrySDKName:      "Datadog",
		semconv.AttributeTelemetrySDKVersion:   payload.TracerVersion,
	} {
		if v != "" {
			sharedAttributes.PutStr(k, v)
		}
	}

	for k, v := range payload.Tags {
		if k = translateDataDogKeyToOtel(k); v != "" {
			sharedAttributes.PutStr(k, v)
		}
	}

	upsertHeadersAttributes(req, sharedAttributes)

	// Creating a map of service spans to slices
	// since the expectation is that `service.name`
	// is added as a resource attribute in most systems
	// now instead of being a span level attribute.
	groupByService := make(map[string]ptrace.SpanSlice)

	for _, trace := range traces {
		for _, span := range trace {
			slice, exist := groupByService[span.Service]
			if !exist {
				slice = ptrace.NewSpanSlice()
				groupByService[span.Service] = slice
			}
			newSpan := slice.AppendEmpty()

			newSpan.SetTraceID(uInt64ToTraceID(0, span.TraceID))
			newSpan.SetSpanID(uInt64ToSpanID(span.SpanID))
			newSpan.SetStartTimestamp(pcommon.Timestamp(span.Start))
			newSpan.SetEndTimestamp(pcommon.Timestamp(span.Start + span.Duration))
			newSpan.SetParentSpanID(uInt64ToSpanID(span.ParentID))
			newSpan.SetName(span.Name)
			newSpan.Status().SetCode(ptrace.StatusCodeOk)
			newSpan.Attributes().PutStr("span.Resource", span.Resource)

			if span.Error > 0 {
				newSpan.Status().SetCode(ptrace.StatusCodeError)
			}

			for k, v := range span.GetMeta() {
				if k = translateDataDogKeyToOtel(k); len(k) > 0 {
					newSpan.Attributes().PutStr(k, v)
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
		}
	}

	results := ptrace.NewTraces()
	for service, spans := range groupByService {
		rs := results.ResourceSpans().AppendEmpty()
		rs.SetSchemaUrl(semconv.SchemaURL)
		sharedAttributes.CopyTo(rs.Resource().Attributes())
		rs.Resource().Attributes().PutStr(semconv.AttributeServiceName, service)

		in := rs.ScopeSpans().AppendEmpty()
		in.Scope().SetName("Datadog")
		in.Scope().SetVersion(payload.TracerVersion)
		spans.CopyTo(in.Spans())
	}

	return results
}

func translateDataDogKeyToOtel(k string) string {
	switch strings.ToLower(k) {
	case "env":
		return semconv.AttributeDeploymentEnvironment
	case "version":
		return semconv.AttributeServiceVersion
	case "container_id":
		return semconv.AttributeContainerID
	case "container_name":
		return semconv.AttributeContainerName
	case "image_name":
		return semconv.AttributeContainerImageName
	case "image_tag":
		return semconv.AttributeContainerImageTag
	case "process_id":
		return semconv.AttributeProcessPID
	case "error.stacktrace":
		return semconv.AttributeExceptionStacktrace
	case "error.msg":
		return semconv.AttributeExceptionMessage
	default:
		return k
	}

}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

func putBuffer(buffer *bytes.Buffer) {
	bufferPool.Put(buffer)
}

func handlePayload(req *http.Request) (tp *pb.TracerPayload, err error) {
	defer func() {
		_, errs := io.Copy(io.Discard, req.Body)
		err = multierr.Combine(err, errs, req.Body.Close())
	}()

	switch {
	case strings.HasPrefix(req.URL.Path, "/v0.7"):
		buf := getBuffer()
		defer putBuffer(buf)
		if _, err = io.Copy(buf, req.Body); err != nil {
			return nil, err
		}
		var tracerPayload pb.TracerPayload
		_, err = tracerPayload.UnmarshalMsg(buf.Bytes())
		return &tracerPayload, err
	case strings.HasPrefix(req.URL.Path, "/v0.5"):
		buf := getBuffer()
		defer putBuffer(buf)
		if _, err = io.Copy(buf, req.Body); err != nil {
			return nil, err
		}
		var traces pb.Traces
		err = traces.UnmarshalMsgDictionary(buf.Bytes())
		return &pb.TracerPayload{
			LanguageName:    req.Header.Get("Datadog-Meta-Lang"),
			LanguageVersion: req.Header.Get("Datadog-Meta-Lang-Version"),
			TracerVersion:   req.Header.Get("Datadog-Meta-Tracer-Version"),
			Chunks:          traceChunksFromTraces(traces),
		}, err
	case strings.HasPrefix(req.URL.Path, "/v0.1"):
		var spans []pb.Span
		if err = json.NewDecoder(req.Body).Decode(&spans); err != nil {
			return nil, err
		}
		return &pb.TracerPayload{
			LanguageName:    req.Header.Get("Datadog-Meta-Lang"),
			LanguageVersion: req.Header.Get("Datadog-Meta-Lang-Version"),
			TracerVersion:   req.Header.Get("Datadog-Meta-Tracer-Version"),
			Chunks:          traceChunksFromSpans(spans),
		}, nil

	default:
		var traces pb.Traces
		if err = decodeRequest(req, &traces); err != nil {
			return nil, err
		}
		return &pb.TracerPayload{
			LanguageName:    req.Header.Get("Datadog-Meta-Lang"),
			LanguageVersion: req.Header.Get("Datadog-Meta-Lang-Version"),
			TracerVersion:   req.Header.Get("Datadog-Meta-Tracer-Version"),
			Chunks:          traceChunksFromTraces(traces),
		}, err
	}
}

func decodeRequest(req *http.Request, dest *pb.Traces) (err error) {
	switch mediaType := getMediaType(req); mediaType {
	case "application/msgpack":
		buf := getBuffer()
		defer putBuffer(buf)
		_, err = io.Copy(buf, req.Body)
		if err != nil {
			return err
		}
		_, err = dest.UnmarshalMsg(buf.Bytes())
		return err
	case "application/json":
		fallthrough
	case "text/json":
		fallthrough
	case "":
		err = json.NewDecoder(req.Body).Decode(&dest)
		return err
	default:
		// do our best
		if err1 := json.NewDecoder(req.Body).Decode(&dest); err1 != nil {
			buf := getBuffer()
			defer putBuffer(buf)
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
			Priority: int32(0),
			Spans:    t,
		})
	}
	return traceChunks
}

func traceChunksFromTraces(traces pb.Traces) []*pb.TraceChunk {
	traceChunks := make([]*pb.TraceChunk, 0, len(traces))
	for _, trace := range traces {
		traceChunks = append(traceChunks, &pb.TraceChunk{
			Priority: int32(0),
			Spans:    trace,
		})
	}

	return traceChunks
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
