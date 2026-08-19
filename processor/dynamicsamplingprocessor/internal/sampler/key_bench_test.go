// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"fmt"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

// BenchmarkExtractKey measures per-scope extraction cost across trace sizes.
// resource. is O(resources); scope., span., any., and root. walk every span,
// and root. additionally invokes the root matcher per span.
func BenchmarkExtractKey(b *testing.B) {
	sizes := []int{10, 1000, 10000}
	scopes := []string{
		`resource.attributes["service.name"]`,
		`scope.attributes["lib.name"]`,
		`span.attributes["http.route"]`,
		`span.attributes["missing.key"]`,
		`root.attributes["http.route"]`,
		`any.attributes["service.name"]`,
		// worst case: the lookup misses at resource, scope, and every span
		`any.attributes["missing.key"]`,
	}
	for _, size := range sizes {
		spans := benchTrace(size)
		isRoot := func(_ ptrace.ResourceSpans, _ ptrace.ScopeSpans, span ptrace.Span) bool {
			return span.ParentSpanID().IsEmpty()
		}
		for _, sel := range scopes {
			selectors, err := ParseSelectors([]string{sel})
			if err != nil {
				b.Fatal(err)
			}
			name := strings.ReplaceAll(strings.ReplaceAll(sel, `.attributes["`, ":"), `"]`, "")
			b.Run(fmt.Sprintf("%dspans/%s", size, name), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					ExtractKey(spans, selectors, isRoot)
				}
			})
		}
	}
}

func benchTrace(spanCount int) []ptrace.ResourceSpans {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "api")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().Attributes().PutStr("lib.name", "otel")
	for i := range spanCount {
		span := ss.Spans().AppendEmpty()
		span.SetSpanID([8]byte{byte(i + 1), byte(i >> 8)})
		if i > 0 {
			span.SetParentSpanID([8]byte{1})
		}
		span.Attributes().PutStr("http.route", "/api")
	}
	return []ptrace.ResourceSpans{rs}
}
