// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// cleanTraceAttributes removes empty/null attributes from all spans in the traces
func cleanTraceAttributes(td ptrace.Traces) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		// Clean resource attributes
		rs.Resource().Attributes().RemoveIf(func(_ string, v pcommon.Value) bool {
			return v.Type() == pcommon.ValueTypeEmpty
		})

		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			ss := sss.At(j)

			// Clean scope attributes
			ss.Scope().Attributes().RemoveIf(func(_ string, v pcommon.Value) bool {
				return v.Type() == pcommon.ValueTypeEmpty
			})

			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				// Clean span attributes
				span.Attributes().RemoveIf(func(_ string, v pcommon.Value) bool {
					return v.Type() == pcommon.ValueTypeEmpty
				})

				// Clean span event attributes
				events := span.Events()
				for l := 0; l < events.Len(); l++ {
					events.At(l).Attributes().RemoveIf(func(_ string, v pcommon.Value) bool {
						return v.Type() == pcommon.ValueTypeEmpty
					})
				}

				// Clean span link attributes
				links := span.Links()
				for l := 0; l < links.Len(); l++ {
					links.At(l).Attributes().RemoveIf(func(_ string, v pcommon.Value) bool {
						return v.Type() == pcommon.ValueTypeEmpty
					})
				}
			}
		}
	}
}
