// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"

import (
	"sort"
	"strings"
	"time"

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
// durationBuckets are the cut points used by span.duration / trace.duration
// selectors; they may be nil when no duration selector is present.
func ExtractKey(spans []ptrace.ResourceSpans, selectors []Selector, isRoot RootMatcher, durationBuckets []time.Duration) string {
	parts := make([]string, len(selectors))
	for i, sel := range selectors {
		parts[i] = extractSelector(spans, sel, isRoot, durationBuckets)
	}
	return strings.Join(parts, fieldSeparator)
}

func extractSelector(spans []ptrace.ResourceSpans, sel Selector, isRoot RootMatcher, durationBuckets []time.Duration) string {
	switch sel.Kind {
	case KindTraceDuration:
		return extractTraceDuration(spans, durationBuckets)
	case KindSpanDuration:
		return extractSpanDuration(spans, sel, isRoot, durationBuckets)
	default:
		return extractAttribute(spans, sel, isRoot)
	}
}

func extractAttribute(spans []ptrace.ResourceSpans, sel Selector, isRoot RootMatcher) string {
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
	return joinSeen(seen)
}

func extractSpanDuration(spans []ptrace.ResourceSpans, sel Selector, isRoot RootMatcher, buckets []time.Duration) string {
	seen := make(map[string]struct{})
	for _, rs := range spans {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				if sel.Root && (isRoot == nil || !isRoot(rs, ss, span)) {
					continue
				}
				if d, ok := spanDuration(span); ok {
					seen[DurationBucketLabel(d, buckets)] = struct{}{}
				}
			}
		}
	}
	return joinSeen(seen)
}

func extractTraceDuration(spans []ptrace.ResourceSpans, buckets []time.Duration) string {
	var (
		minStart pcommon.Timestamp
		maxEnd   pcommon.Timestamp
		found    bool
	)
	for _, rs := range spans {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				start, end := span.StartTimestamp(), span.EndTimestamp()
				if start == 0 || end == 0 || end < start {
					continue
				}
				if !found || start < minStart {
					minStart = start
				}
				if !found || end > maxEnd {
					maxEnd = end
				}
				found = true
			}
		}
	}
	if !found {
		return missingValuePlaceholder
	}
	return DurationBucketLabel(maxEnd.AsTime().Sub(minStart.AsTime()), buckets)
}

func spanDuration(span ptrace.Span) (time.Duration, bool) {
	start, end := span.StartTimestamp(), span.EndTimestamp()
	if start == 0 || end == 0 || end < start {
		return 0, false
	}
	return end.AsTime().Sub(start.AsTime()), true
}

func joinSeen(seen map[string]struct{}) string {
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
