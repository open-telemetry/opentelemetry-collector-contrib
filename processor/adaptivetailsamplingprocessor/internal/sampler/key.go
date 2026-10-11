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
	wantResource := sel.Origin == OriginResource || sel.Origin == OriginAny
	wantScope := sel.Origin == OriginScope || sel.Origin == OriginAny
	wantSpan := sel.Origin == OriginSpan || sel.Origin == OriginAny
	for _, rs := range spans {
		// Without the root prefix, resource and scope attributes are read
		// per resource/scope regardless of the spans they carry. The root
		// prefix instead reads them from the matched root span's resource and
		// scope, so that collection happens inside the span loop below.
		if !sel.Root {
			if wantResource {
				collectAttrValue(rs.Resource().Attributes(), sel.Key, seen)
			}
			if sel.Origin == OriginResource {
				continue
			}
		}
		for _, ss := range rs.ScopeSpans().All() {
			if !sel.Root {
				if wantScope {
					collectAttrValue(ss.Scope().Attributes(), sel.Key, seen)
				}
				if sel.Origin == OriginScope {
					continue
				}
			}
			for _, span := range ss.Spans().All() {
				if sel.Root {
					if isRoot == nil || !isRoot(rs, ss, span) {
						continue
					}
					if wantResource {
						collectAttrValue(rs.Resource().Attributes(), sel.Key, seen)
					}
					if wantScope {
						collectAttrValue(ss.Scope().Attributes(), sel.Key, seen)
					}
				}
				if wantSpan {
					collectAttrValue(span.Attributes(), sel.Key, seen)
				}
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
