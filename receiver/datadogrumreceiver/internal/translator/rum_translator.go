package translator

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

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

type RUMPayload struct {
	Type string
}

func parseIDs(payload map[string]any) (traceID uint64, spanID uint64, err error) {
	ddMetadata, ok := payload["_dd"].(map[string]any)
	if !ok {
		return 0, 0, fmt.Errorf("failed to find _dd metadata in payload")
	}

	traceIDString, ok := ddMetadata["trace_id"].(string)
	if !ok {
		return 0, 0, fmt.Errorf("failed to retrieve traceID from payload")
	}
	traceID, err = strconv.ParseUint(traceIDString, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse traceID: %w", err)
	}

	spanIDString, ok := ddMetadata["span_id"].(string)
	if !ok {
		return 0, 0, fmt.Errorf("failed to retrieve spanID from payload")
	}
	spanID, err = strconv.ParseUint(spanIDString, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse spanID: %w", err)
	}

	return traceID, spanID, nil
}

func parseTimestamp(payload map[string]any) (pcommon.Timestamp, float64, error) {
	date, ok := payload["date"]
	if !ok {
		return 0, 0, fmt.Errorf("date field not found in payload")
	}

	dateFloat, ok := date.(float64)
	if !ok {
		return 0, 0, fmt.Errorf("failed to retrieve date from payload")
	}

	dateNanoseconds := uint64(dateFloat) * 1000000

	duration, ok := payload["resource"].(map[string]any)["duration"].(float64)
	if !ok {
		return 0, 0, fmt.Errorf("failed to parse duration from payload")
	}

	return pcommon.Timestamp(dateNanoseconds), duration, nil
}

func ToTraces(payload map[string]any, req *http.Request, rawRequestBody []byte) ptrace.Traces {
	r := pcommon.NewResource()
	rl := plog.ResourceLogs().SetResource(r)
	rt := ptrace.ResourceSpans().SetResource(r)
	results := ptrace.NewTraces()
	resource := results.ResourceSpans().AppendEmpty()
	ParseRUMRequestIntoResource(resource.Resource(), payload, req, rawRequestBody)

	instrumentationScope := results.ScopeSpans().AppendEmpty()
	instrumentationScope.Scope().SetName("Datadog")

	traceID, spanID, err := parseIDs(payload)
	if err != nil {
		fmt.Println(err)
		return results
	}

	newSpan := instrumentationScope.Spans().AppendEmpty()
	newSpan.SetName("RUMResource")
	newSpan.SetTraceID(uInt64ToTraceID(0, traceID))
	newSpan.SetSpanID(uInt64ToSpanID(spanID))
	newSpan.Attributes().PutBool("datadog.is_rum", true)

	startTime, duration, err := parseTimestamp(payload)
	if err != nil {
		fmt.Println(err)
		return results
	}
	newSpan.SetStartTimestamp(startTime)
	resource.Resource().Attributes().PutInt("datadog.rum.duration", int64(duration))
	newSpan.SetEndTimestamp(pcommon.Timestamp(uint64(startTime) + uint64(duration)))

	return results
}

func ParseRUMRequestIntoResource(resource pcommon.Resource, payload map[string]any, req *http.Request, rawRequestBody []byte) {
	resource.Attributes().PutStr(semconv.AttributeServiceName, "browser-rum-sdk")
	resource.Attributes().PutBool("datadog.is_rum", true)

	prettyPayload, _ := json.MarshalIndent(payload, "", "\t")
	resource.Attributes().PutStr("pretty_payload", string(prettyPayload))

	bodyDump := resource.Attributes().PutEmptyBytes("request_body_dump")
	bodyDump.FromRaw(rawRequestBody)

	// Store HTTP headers as attributes
	headerAttrs := resource.Attributes().PutEmptyMap("request_headers")
	for headerName, headerValues := range req.Header {
		headerValueList := headerAttrs.PutEmptySlice(headerName)
		for _, headerValue := range headerValues {
			headerValueList.AppendEmpty().SetStr(headerValue)
		}
	}

	// Store URL query parameters as attributes
	queryAttrs := resource.Attributes().PutEmptyMap("request_query")
	for paramName, paramValues := range req.URL.Query() {
		paramValueList := queryAttrs.PutEmptySlice(paramName)
		for _, paramValue := range paramValues {
			paramValueList.AppendEmpty().SetStr(paramValue)
		}
	}

	resource.Attributes().PutStr("request_ddforward", req.URL.Query().Get("ddforward"))
}

func uInt64ToTraceID(high, low uint64) pcommon.TraceID {
	traceID := [16]byte{}
	binary.BigEndian.PutUint64(traceID[0:8], high)
	binary.BigEndian.PutUint64(traceID[8:16], low)
	return pcommon.TraceID(traceID)
}

func uInt64ToSpanID(id uint64) pcommon.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint64(spanID[:], id)
	return pcommon.SpanID(spanID)
}
