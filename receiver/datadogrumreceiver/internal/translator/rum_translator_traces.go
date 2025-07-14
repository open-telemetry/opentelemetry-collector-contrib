package translator

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

func ToTraces(payload map[string]any, req *http.Request, reqBytes []byte, traceparent string) ptrace.Traces {
	results := ptrace.NewTraces()
	rs := results.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl(semconv.SchemaURL)
	parseRUMRequestIntoResource(rs.Resource(), payload, req, reqBytes)

	in := rs.ScopeSpans().AppendEmpty()
	in.Scope().SetName(InstrumentationScopeName)

	traceID, spanID, err := parseW3CTraceContext(traceparent)
	fmt.Println("%%%%%%%%%%%%%% W3C TRACE ID: ", traceID)
	fmt.Println("%%%%%%%%%%%%%% W3C SPAN ID: ", spanID)
	if err != nil {
		err = nil
		traceID, spanID, err = parseIDs(payload, req)
		if err != nil {
			fmt.Println(err)
			return results
		}
	}

	fmt.Println("%%%%%%%%%%%%%% TRACE ID: ", traceID)
	fmt.Println("%%%%%%%%%%%%%% SPAN ID: ", spanID)
	newSpan := in.Spans().AppendEmpty()
	if eventType, ok := payload[AttrType].(string); ok {
		newSpan.SetName("datadog.rum." + eventType)
	} else {
		newSpan.SetName("datadog.rum.event")
	}
	newSpan.SetTraceID(traceID)
	newSpan.SetSpanID(spanID)

	flatPayload := flattenJSON(payload)

    setAttributes(flatPayload, newSpan.Attributes())

	return results
}
