// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestCleanTraceAttributes(t *testing.T) {
	// Create traces with empty/null attributes
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()

	// Add resource attributes including empty one
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	rs.Resource().Attributes().PutEmpty("empty.resource.attr")

	ss := rs.ScopeSpans().AppendEmpty()

	// Add scope attributes including empty one
	ss.Scope().Attributes().PutStr("scope.name", "test-scope")
	ss.Scope().Attributes().PutEmpty("empty.scope.attr")

	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")

	// Add span attributes including empty ones
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutEmpty("empty.span.attr")
	span.Attributes().PutInt("http.status_code", 200)
	span.Attributes().PutEmpty("another.empty.attr")

	// Add event with empty attribute
	event := span.Events().AppendEmpty()
	event.SetName("test-event")
	event.Attributes().PutStr("event.type", "test")
	event.Attributes().PutEmpty("empty.event.attr")

	// Add link with empty attribute
	link := span.Links().AppendEmpty()
	link.Attributes().PutStr("link.type", "test")
	link.Attributes().PutEmpty("empty.link.attr")

	// Count attributes before cleaning
	resourceAttrsBefore := rs.Resource().Attributes().Len()
	scopeAttrsBefore := ss.Scope().Attributes().Len()
	spanAttrsBefore := span.Attributes().Len()
	eventAttrsBefore := event.Attributes().Len()
	linkAttrsBefore := link.Attributes().Len()

	t.Logf("Before cleaning - Resource: %d, Scope: %d, Span: %d, Event: %d, Link: %d",
		resourceAttrsBefore, scopeAttrsBefore, spanAttrsBefore, eventAttrsBefore, linkAttrsBefore)

	// Clean the traces
	cleanTraceAttributes(td)

	// Verify empty attributes were removed
	resourceAttrsAfter := rs.Resource().Attributes().Len()
	scopeAttrsAfter := ss.Scope().Attributes().Len()
	spanAttrsAfter := span.Attributes().Len()
	eventAttrsAfter := event.Attributes().Len()
	linkAttrsAfter := link.Attributes().Len()

	t.Logf("After cleaning - Resource: %d, Scope: %d, Span: %d, Event: %d, Link: %d",
		resourceAttrsAfter, scopeAttrsAfter, spanAttrsAfter, eventAttrsAfter, linkAttrsAfter)

	assert.Equal(t, resourceAttrsBefore-1, resourceAttrsAfter, "Resource attributes should have 1 less")
	assert.Equal(t, scopeAttrsBefore-1, scopeAttrsAfter, "Scope attributes should have 1 less")
	assert.Equal(t, spanAttrsBefore-2, spanAttrsAfter, "Span attributes should have 2 less")
	assert.Equal(t, eventAttrsBefore-1, eventAttrsAfter, "Event attributes should have 1 less")
	assert.Equal(t, linkAttrsBefore-1, linkAttrsAfter, "Link attributes should have 1 less")

	// Verify that only non-empty attributes remain
	rs.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
		assert.NotEqual(t, pcommon.ValueTypeEmpty, v.Type(), "Found empty resource attribute: %s", k)
		return true
	})

	span.Attributes().Range(func(k string, v pcommon.Value) bool {
		assert.NotEqual(t, pcommon.ValueTypeEmpty, v.Type(), "Found empty span attribute: %s", k)
		return true
	})
}

func TestTracesJSONLMarshalerWithPdataCleaning(t *testing.T) {
	// Create traces with empty attributes
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	rs.Resource().Attributes().PutEmpty("empty.field")

	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutEmpty("http.status_code") // This should be removed

	// Marshal using our JSONL marshaler
	marshaler := &tracesJSONLMarshaler{}
	jsonBytes, err := marshaler.MarshalTraces(td)
	assert.NoError(t, err, "Failed to marshal traces")

	jsonStr := string(jsonBytes)
	t.Logf("JSONL marshaler output:\n%s", jsonStr)

	// Verify no null values in output
	assert.NotContains(t, jsonStr, ":null", "JSON output contains null values")
	assert.NotContains(t, jsonStr, ": null", "JSON output contains null values")

	// Verify it contains expected non-empty values
	assert.Contains(t, jsonStr, "service.name", "JSON output missing expected attribute")
	assert.Contains(t, jsonStr, "http.method", "JSON output missing expected attribute")

	// Verify empty attributes are not present
	assert.NotContains(t, jsonStr, "empty.field", "JSON output contains removed empty attribute")
	assert.NotContains(t, jsonStr, "http.status_code", "JSON output contains removed empty attribute")
}
