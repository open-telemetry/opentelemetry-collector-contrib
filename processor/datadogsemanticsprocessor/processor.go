// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogsemanticsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor"

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/trace/transform"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func (tp *tracesProcessor) processTraces(ctx context.Context, td ptrace.Traces) (output ptrace.Traces, err error) {
	rspans := td.ResourceSpans()
	for i := 0; i < rspans.Len(); i++ {
		rspan := rspans.At(i)
		for j := 0; j < rspan.ScopeSpans().Len(); j++ {
			libspans := rspan.ScopeSpans().At(j)
			for k := 0; k < libspans.Spans().Len(); k++ {
				otelspan := libspans.Spans().At(k)
				var ddspan *pb.Span

				ddspan = transform.OtelSpanToDDSpan(otelspan, rspan.Resource(), libspans.Scope(), tp.agentCfg, []string{""})
				if tp.overrideIncomingDatadogFields {
					otelspan.Attributes().RemoveIf(func(k string, v pcommon.Value) bool {
						return strings.HasPrefix(k, "datadog.")
					})
				}
				if _, ok := otelspan.Attributes().Get("datadog.service"); !ok {
					otelspan.Attributes().PutStr("datadog.service", ddspan.Service)
				}
				if _, ok := otelspan.Attributes().Get("datadog.name"); !ok {
					otelspan.Attributes().PutStr("datadog.name", ddspan.Name)
				}
				if _, ok := otelspan.Attributes().Get("datadog.resource"); !ok {
					otelspan.Attributes().PutStr("datadog.resource", ddspan.Resource)
				}
				if _, ok := otelspan.Attributes().Get("datadog.trace_id"); !ok {
					otelspan.Attributes().PutStr("datadog.trace_id", fmt.Sprintf("%d", ddspan.TraceID))
				}
				if _, ok := otelspan.Attributes().Get("datadog.span_id"); !ok {
					otelspan.Attributes().PutStr("datadog.span_id", fmt.Sprintf("%d", ddspan.SpanID))
				}
				if _, ok := otelspan.Attributes().Get("datadog.parent_id"); !ok {
					otelspan.Attributes().PutStr("datadog.parent_id", fmt.Sprintf("%d", ddspan.ParentID))
				}
				if _, ok := otelspan.Attributes().Get("datadog.type"); !ok {
					otelspan.Attributes().PutStr("datadog.type", ddspan.Type)
				}
				if _, ok := otelspan.Attributes().Get("datadog.meta"); !ok {
					metaMap := otelspan.Attributes().PutEmptyMap("datadog.meta")
					for key, value := range ddspan.Meta {
						metaMap.PutStr(key, value)
					}
				}
				if _, ok := otelspan.Attributes().Get("datadog.metrics"); !ok {
					metricsMap := otelspan.Attributes().PutEmptyMap("datadog.metrics")
					for key, value := range ddspan.Metrics {
						metricsMap.PutDouble(key, value)
					}
				}
			}
		}
	}

	return td, err // XXX can the input spans be modified? or do we need to clone them to add the attributes then output that?
}
