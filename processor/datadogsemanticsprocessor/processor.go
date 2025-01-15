// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogsemanticsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor"

import (
	"context"
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/trace/traceutil"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func (tp *tracesProcessor) processTraces(ctx context.Context, td ptrace.Traces) (output ptrace.Traces, err error) {
	rspans := td.ResourceSpans()
	for i := 0; i < rspans.Len(); i++ {
		rspan := rspans.At(i)
		otelres := rspan.Resource()
		for j := 0; j < rspan.ScopeSpans().Len(); j++ {
			libspans := rspan.ScopeSpans().At(j)
			for k := 0; k < libspans.Spans().Len(); k++ {
				otelspan := libspans.Spans().At(k)
				sattr := otelspan.Attributes()
				if tp.overrideIncomingDatadogFields {
					sattr.RemoveIf(func(k string, v pcommon.Value) bool {
						return strings.HasPrefix(k, "datadog.")
					})
				}
				if _, ok := sattr.Get("datadog.service"); !ok {
					sattr.PutStr("datadog.service", traceutil.GetOTelService(otelres, true))
				}
				if _, ok := sattr.Get("datadog.name"); !ok {
					sattr.PutStr("datadog.name", traceutil.GetOTelOperationNameV2(otelspan))
				}
				if _, ok := sattr.Get("datadog.resource"); !ok {
					sattr.PutStr("datadog.resource", traceutil.GetOTelResourceV2(otelspan, otelres))
				}
				if _, ok := sattr.Get("datadog.trace_id"); !ok {
					sattr.PutStr("datadog.trace_id", fmt.Sprintf("%d", traceutil.OTelTraceIDToUint64(otelspan.TraceID())))
				}
				if _, ok := sattr.Get("datadog.span_id"); !ok {
					sattr.PutStr("datadog.span_id", fmt.Sprintf("%d", traceutil.OTelSpanIDToUint64(otelspan.SpanID())))
				}
				if _, ok := sattr.Get("datadog.parent_id"); !ok {
					sattr.PutStr("datadog.parent_id", fmt.Sprintf("%d", traceutil.OTelSpanIDToUint64(otelspan.ParentSpanID())))
				}
				if _, ok := sattr.Get("datadog.type"); !ok {
					sattr.PutStr("datadog.type", traceutil.GetOTelSpanType(otelspan, otelres))
				}

				var metaMap pcommon.Map
				if existingMetaMap, ok := sattr.Get("datadog.meta"); !ok {
					metaMap = sattr.PutEmptyMap("datadog.meta")
				} else {
					metaMap = existingMetaMap.Map()
				}
				var metricsMap pcommon.Map
				if existingMetricsMap, ok := sattr.Get("datadog.metrics"); !ok {
					metricsMap = sattr.PutEmptyMap("datadog.metrics")
				} else {
					metricsMap = existingMetricsMap.Map()
				}
				spanKind := otelspan.Kind()
				metaMap.PutStr("span.kind", traceutil.OTelSpanKindName(spanKind))
				code := traceutil.GetOTelStatusCode(otelspan)
				if code != 0 {
					metricsMap.PutDouble(traceutil.TagStatusCode, float64(code))
				}
				if otelspan.Status().Code() == ptrace.StatusCodeError {
					sattr.PutInt("datadog.error", 1)
				} else {
					sattr.PutInt("datadog.error", 0)
				}
				isTopLevel := otelspan.ParentSpanID() == pcommon.NewSpanIDEmpty() || spanKind == ptrace.SpanKindServer || spanKind == ptrace.SpanKindConsumer
				if isTopLevel {
					metricsMap.PutDouble("_top_level", 1.0)
				} else {
					metricsMap.PutDouble("_top_level", 0.0)
				}
				if spanKind == ptrace.SpanKindClient || spanKind == ptrace.SpanKindProducer {
					// Compute stats for client-side spans
					metricsMap.PutDouble("_dd.measured", 1.0)
				}
				for _, peerTagKey := range tp.peerTagKeys {
					if peerTagVal := traceutil.GetOTelAttrValInResAndSpanAttrs(otelspan, otelres, false, peerTagKey); peerTagVal != "" {
						metaMap.PutStr(peerTagKey, peerTagVal)
					}
				}
			}
		}
	}

	return td, err // XXX can the input spans be modified? or do we need to clone them to add the attributes then output that?
}
