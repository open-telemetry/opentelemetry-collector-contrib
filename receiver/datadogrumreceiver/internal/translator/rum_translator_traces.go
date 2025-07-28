// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogrumreceiver/internal/translator"

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
	"go.uber.org/zap"
)

func ToTraces(logger *zap.Logger, payload map[string]any, req *http.Request, reqBytes []byte, traceparent string) ptrace.Traces {
	results := ptrace.NewTraces()
	rs := results.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl(semconv.SchemaURL)
	parseRUMRequestIntoResource(rs.Resource(), payload, req, reqBytes)

	in := rs.ScopeSpans().AppendEmpty()
	in.Scope().SetName(InstrumentationScopeName)

	traceID, spanID, err := parseW3CTraceContext(traceparent)
	logger.Info("W3C Trace ID", zap.String("traceID", traceID.String()))
	logger.Info("W3C Span ID", zap.String("spanID", spanID.String()))
	if err != nil {
		err = nil
		traceID, spanID, err = parseIDs(payload, req)
		if err != nil {
			fmt.Println(err)
			return results
		}
	}

	logger.Info("Trace ID", zap.String("traceID", traceID.String()))
	logger.Info("Span ID", zap.String("spanID", spanID.String()))
	newSpan := in.Spans().AppendEmpty()
	if eventType, ok := payload[AttrType].(string); ok {
		newSpan.SetName("datadog.rum." + eventType)
	} else {
		newSpan.SetName("datadog.rum.event")
	}
	newSpan.SetTraceID(traceID)
	newSpan.SetSpanID(spanID)
	newSpan.Attributes().PutStr("operation.name", "rum")

	flatPayload := flattenJSON(payload)

	setAttributes(flatPayload, newSpan.Attributes())

	return results
}
