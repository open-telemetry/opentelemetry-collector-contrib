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
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func metricNameToCompact(metricName string, attributes pcommon.Map) string {
	if attributes.Len() == 0 {
		return metricName + "{}"
	}

	var labels = []string{}
	attributes.Sort().Range(func(key string, value pcommon.Value) bool {
		labels = append(labels, fmt.Sprintf("%s=\"%s\"", key, value.AsString()))

		return true
	})

	return fmt.Sprintf("%s{%s}", metricName, strings.Join(labels, ","))
}

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
