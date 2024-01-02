// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"

import (
	"encoding/hex"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func convertTraceID(traceID pcommon.TraceID) string {
	return hex.EncodeToString(traceID[:])
}

func convertSpanID(spanID pcommon.SpanID) string {
	return hex.EncodeToString(spanID[:])
}

func otelKindToInstanaKind(otelKind ptrace.SpanKind) (string, bool) {
	switch otelKind {
	case ptrace.SpanKindServer:
		return InstanaSpanKindServer, true
	case ptrace.SpanKindClient:
		return InstanaSpanKindClient, false
	case ptrace.SpanKindProducer:
		return InstanaSpanKindProducer, false
	case ptrace.SpanKindConsumer:
		return InstanaSpanKindConsumer, true
	case ptrace.SpanKindInternal:
		return InstanaSpanKindInternal, false
	default:
		return "unknown", false
	}
}
