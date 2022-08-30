// Copyright 2022, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"

import (
	"encoding/hex"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func convertTraceID(traceID pcommon.TraceID) string {
	const byteLength = 16

	bytes := traceID.Bytes()
	traceBytes := make([]byte, 0)

	for (len(traceBytes) + len(bytes)) < byteLength {
		traceBytes = append(traceBytes, 0)
	}

	for _, byte := range bytes {
		traceBytes = append(traceBytes, byte)
	}

	return hex.EncodeToString(traceBytes)
}

func convertSpanID(spanID pcommon.SpanID) string {
	const byteLength = 8

	bytes := spanID.Bytes()
	spanBytes := make([]byte, 0)

	for (len(spanBytes) + len(bytes)) < byteLength {
		spanBytes = append(spanBytes, 0)
	}

	for _, byte := range bytes {
		spanBytes = append(spanBytes, byte)
	}

	return hex.EncodeToString(spanBytes)
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
