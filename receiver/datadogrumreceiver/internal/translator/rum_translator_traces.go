package translator

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

func setTraceAttributes(flatPayload map[string]any, span ptrace.Span) {
    for rumKey, meta := range attributeMetaMap {
        val, exists := flatPayload[rumKey]
        if !exists {
            continue
        }

        switch meta.Type {
        case StringAttribute:
            if s, ok := val.(string); ok {
                span.Attributes().PutStr(meta.OTLPName, s)
            }
        case BoolAttribute:
            if b, ok := val.(bool); ok {
                span.Attributes().PutBool(meta.OTLPName, b)
            }
        case NumberAttribute:
            if f, ok := val.(float64); ok {
                span.Attributes().PutDouble(meta.OTLPName, f)
            }
		case IntegerAttribute:
			if i, ok := val.(int); ok {
				span.Attributes().PutInt(meta.OTLPName, int64(i))
			}
        case ObjectAttribute:
            if o, ok := val.(map[string]any); ok {
				objVal := span.Attributes().PutEmptyMap(meta.OTLPName)
                for k, v := range o {
                    objVal.PutStr(meta.OTLPName + k, v.(string))
                }
            }
        case ArrayAttribute:
            if a, ok := val.([]any); ok {
				arrVal := span.Attributes().PutEmptySlice(meta.OTLPName)
                for _, v := range a {
					arrVal.AppendEmpty().SetStr(v.(string))
				}
            }
        case StringOrArrayAttribute:
            if s, ok := val.(string); ok {
                span.Attributes().PutStr(meta.OTLPName, s)
            } else if a, ok := val.([]any); ok {
				arrVal := span.Attributes().PutEmptySlice(meta.OTLPName)
                for _, v := range a {
					arrVal.AppendEmpty().SetStr(v.(string))
				}
            }
        }
    }
}

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

    setTraceAttributes(flatPayload, newSpan)

	return results
}
