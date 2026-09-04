// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"

import (
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	missingValuePlaceholder = "<missing>"
	fieldSeparator          = "•" // bullet
	valueSeparator          = ","
)

// RootMatcher reports whether a span satisfies the processor's root-span
// condition. Only consulted for root-scoped selectors.
type RootMatcher func(rs ptrace.ResourceSpans, ss ptrace.ScopeSpans, span ptrace.Span) bool

// ExtractKey builds a deterministic sampling key from the values selected by
// the fingerprint selectors across the trace. Distinct values per selector
// are sorted to guarantee a stable key independent of span ordering.
func ExtractKey(spans []ptrace.ResourceSpans, selectors []Selector, isRoot RootMatcher) string {
	parts := make([]string, len(selectors))
	for i, sel := range selectors {
		parts[i] = extractSelector(spans, sel, isRoot)
	}
	return strings.Join(parts, fieldSeparator)
}

func extractSelector(spans []ptrace.ResourceSpans, sel Selector, isRoot RootMatcher) string {
	seen := make(map[string]struct{})
	for _, rs := range spans {
		if sel.Scope == ScopeResource || sel.Scope == ScopeAny {
			collectAttrValue(rs.Resource().Attributes(), sel.Key, seen)
			if sel.Scope == ScopeResource {
				continue
			}
		}
		for _, ss := range rs.ScopeSpans().All() {
			if sel.Scope == ScopeScope || sel.Scope == ScopeAny {
				collectAttrValue(ss.Scope().Attributes(), sel.Key, seen)
				if sel.Scope == ScopeScope {
					continue
				}
			}
			for _, span := range ss.Spans().All() {
				if sel.Scope == ScopeRoot && (isRoot == nil || !isRoot(rs, ss, span)) {
					continue
				}
				collectAttrValue(span.Attributes(), sel.Key, seen)
			}
		}
	}
	if len(seen) == 0 {
		return missingValuePlaceholder
	}
	values := make([]string, 0, len(seen))
	for v := range seen {
		values = append(values, v)
	}
	sort.Strings(values)
	return strings.Join(values, valueSeparator)
}

func collectAttrValue(attrs pcommon.Map, field string, out map[string]struct{}) {
	if v, ok := attrs.Get(field); ok {
		out[v.AsString()] = struct{}{}
	}
}
