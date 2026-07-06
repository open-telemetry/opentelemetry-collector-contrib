// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/sampler"

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

// ExtractKey builds a deterministic sampling key from the values of the
// configured key fields across all spans in the trace. Resource and span
// attributes are both scanned. Distinct values per field are sorted to
// guarantee a stable key independent of span ordering.
func ExtractKey(spans []ptrace.ResourceSpans, fields []string) string {
	parts := make([]string, len(fields))
	for i, field := range fields {
		parts[i] = extractField(spans, field)
	}
	return strings.Join(parts, fieldSeparator)
}

func extractField(spans []ptrace.ResourceSpans, field string) string {
	seen := make(map[string]struct{})
	for _, rs := range spans {
		collectAttrValue(rs.Resource().Attributes(), field, seen)
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				collectAttrValue(span.Attributes(), field, seen)
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
