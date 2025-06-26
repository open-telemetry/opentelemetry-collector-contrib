package translator

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
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

func parseW3CTraceContext(traceparent string) (traceID pcommon.TraceID, spanID pcommon.SpanID, err error) {
	// W3C traceparent format: version-traceID-spanID-flags
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("invalid traceparent format: %s", traceparent)
	}

	// Parse trace ID (32 hex characters)
	traceIDBytes, err := hex.DecodeString(parts[1])
	if err != nil || len(traceIDBytes) != 16 {
		return pcommon.NewTraceIDEmpty(), pcommon.SpanID{}, fmt.Errorf("invalid trace ID: %s", parts[1])
	}
	copy(traceID[:], traceIDBytes)

	// Parse span ID
	spanIDBytes, err := hex.DecodeString(parts[2])
	if err != nil || len(spanIDBytes) != 8 {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("invalid parent ID: %s", parts[2])
	}
	copy(spanID[:], spanIDBytes)

	return traceID, spanID, nil
}

func parseIDs(payload map[string]any, req *http.Request) (pcommon.TraceID, pcommon.SpanID, error) {
	ddMetadata, ok := payload["_dd"].(map[string]any)
	if !ok {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to find _dd metadata in payload")
	}

	traceIDString, ok := ddMetadata["trace_id"].(string)
	if !ok {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to retrieve traceID from payload")
	}
	traceID, err := strconv.ParseUint(traceIDString, 10, 64)
	if err != nil {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to parse traceID: %w", err)
	}

	spanIDString, ok := ddMetadata["span_id"].(string)
	if !ok {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to retrieve spanID from payload")
	}
	spanID, err := strconv.ParseUint(spanIDString, 10, 64)
	if err != nil {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to parse spanID: %w", err)
	}

	return uInt64ToTraceID(0, traceID), uInt64ToSpanID(spanID), nil
}

func parseRUMRequestIntoResource(resource pcommon.Resource, payload map[string]any, req *http.Request, rawRequestBody []byte) {
	resource.Attributes().PutStr(semconv.AttributeServiceName, "browser-rum-sdk")

	prettyPayload, _ := json.MarshalIndent(payload, "", "\t")
	resource.Attributes().PutStr("pretty_payload", string(prettyPayload))

	bodyDump := resource.Attributes().PutEmptyBytes("request_body_dump")
	bodyDump.FromRaw(rawRequestBody)

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
