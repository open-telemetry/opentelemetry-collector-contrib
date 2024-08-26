// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type otlpJSONTracesUnmarshaler struct {
    encoding string
}

func (o otlpJSONTracesUnmarshaler) Unmarshal(buf []byte) (ptrace.Traces, error) {
    var jsonData map[string]interface{}
    if err := json.Unmarshal(buf, &jsonData); err != nil {
        return ptrace.NewTraces(), fmt.Errorf("failed to unmarshal JSON: %w", err)
    }

    traces := ptrace.NewTraces()
    resourceSpans := traces.ResourceSpans()

    // Parse resourceSpans
    if resourceSpansArr, ok := jsonData["resourceSpans"].([]interface{}); ok {
        for _, rs := range resourceSpansArr {
            if rsMap, ok := rs.(map[string]interface{}); ok {
                parseResourceSpan(rsMap, resourceSpans.AppendEmpty())
            }
        }
    }

    return traces, nil
}

func (o otlpJSONTracesUnmarshaler) Encoding() string {
    return o.encoding
}

func newOTLPJSONTracesUnmarshaler() TracesUnmarshaler {
    return &otlpJSONTracesUnmarshaler{
        encoding: "otlp_json",
    }
}

func parseResourceSpan(rsMap map[string]interface{}, rs ptrace.ResourceSpans) {
    // Parse resource
    if resource, ok := rsMap["resource"].(map[string]interface{}); ok {
        parseResource(resource, rs.Resource())
    }

    // Parse scopeSpans
    if scopeSpans, ok := rsMap["scopeSpans"].([]interface{}); ok {
        for _, ss := range scopeSpans {
            if ssMap, ok := ss.(map[string]interface{}); ok {
                parseScopeSpan(ssMap, rs.ScopeSpans().AppendEmpty())
            }
        }
    }
}

func parseResource(resource map[string]interface{}, r pcommon.Resource) {
    if attrs, ok := resource["attributes"].([]interface{}); ok {
        parseAttributes(attrs, r.Attributes())
    }
}

func parseScopeSpan(ssMap map[string]interface{}, ss ptrace.ScopeSpans) {
    if scope, ok := ssMap["scope"].(map[string]interface{}); ok {
        parseScope(scope, ss.Scope())
    }

    if spans, ok := ssMap["spans"].([]interface{}); ok {
        for _, span := range spans {
            if spanMap, ok := span.(map[string]interface{}); ok {
                parseSpan(spanMap, ss.Spans().AppendEmpty())
            }
        }
    }
}

func parseScope(scope map[string]interface{}, s pcommon.InstrumentationScope) {
    if name, ok := scope["name"].(string); ok {
        s.SetName(name)
    }
    if version, ok := scope["version"].(string); ok {
        s.SetVersion(version)
    }
    if attrs, ok := scope["attributes"].([]interface{}); ok {
        parseAttributes(attrs, s.Attributes())
    }
}

func parseStatus(status map[string]interface{}, spanStatus ptrace.Status) {
    if code, ok := status["code"].(float64); ok {
        spanStatus.SetCode(ptrace.StatusCode(int32(code)))
    }
    if message, ok := status["message"].(string); ok {
        spanStatus.SetMessage(message)
    }
}

func parseSpan(spanMap map[string]interface{}, span ptrace.Span) {
    if name, ok := spanMap["name"].(string); ok {
        span.SetName(name)
    }
    if traceIDStr, ok := spanMap["traceId"].(string); ok {
        if traceID, err := hex.DecodeString(traceIDStr); err == nil && len(traceID) == 16 {
            span.SetTraceID(pcommon.TraceID(traceID))
        }
    }
    if spanIDStr, ok := spanMap["spanId"].(string); ok {
        if spanID, err := hex.DecodeString(spanIDStr); err == nil && len(spanID) == 8 {
            span.SetSpanID(pcommon.SpanID(spanID))
        }
    }
    if parentSpanIDStr, ok := spanMap["parentSpanId"].(string); ok {
        if parentSpanID, err := hex.DecodeString(parentSpanIDStr); err == nil && len(parentSpanID) == 8 {
            span.SetParentSpanID(pcommon.SpanID(parentSpanID))
        }
    }
    if kindVal, ok := spanMap["kind"].(float64); ok {
        span.SetKind(ptrace.SpanKind(int32(kindVal)))
    }
    if startTime, ok := spanMap["startTimeUnixNano"].(string); ok {
        if startTimeInt, err := strconv.ParseUint(startTime, 10, 64); err == nil {
            span.SetStartTimestamp(pcommon.Timestamp(startTimeInt))
        }
    }
    if endTime, ok := spanMap["endTimeUnixNano"].(string); ok {
        if endTimeInt, err := strconv.ParseUint(endTime, 10, 64); err == nil {
            span.SetEndTimestamp(pcommon.Timestamp(endTimeInt))
        }
    }
    if status, ok := spanMap["status"].(map[string]interface{}); ok {
        parseStatus(status, span.Status())
    }
    if attrs, ok := spanMap["attributes"].([]interface{}); ok {
        parseAttributes(attrs, span.Attributes())
    }
}

func parseAttributes(attrs []interface{}, dest pcommon.Map) {
    for _, attr := range attrs {
        if attrMap, ok := attr.(map[string]interface{}); ok {
            key, _ := attrMap["key"].(string)
            value, _ := attrMap["value"].(map[string]interface{})
            if strVal, ok := value["stringValue"].(string); ok {
                dest.PutStr(key, strVal)
            }
            //TODO: WIll be adding more value types as needed
        }
    }
}