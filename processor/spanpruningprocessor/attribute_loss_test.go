// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

func TestAnalyzeAttributeLoss_NoDiversity(t *testing.T) {
	// All spans have identical attributes - no diversity loss
	nodes := createTestSpanNodes(t, []map[string]string{
		{"db.operation": "select", "db.name": "users"},
		{"db.operation": "select", "db.name": "users"},
		{"db.operation": "select", "db.name": "users"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])
	assert.True(t, result.isEmpty(), "no loss when all values are identical")
}

func TestAnalyzeAttributeLoss_WithDiversity(t *testing.T) {
	// Spans have different values for some attributes (all present in all spans)
	nodes := createTestSpanNodes(t, []map[string]string{
		{"db.operation": "select", "db.name": "users", "db.statement": "SELECT * FROM users"},
		{"db.operation": "select", "db.name": "orders", "db.statement": "SELECT * FROM orders"},
		{"db.operation": "select", "db.name": "products", "db.statement": "SELECT * FROM products"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])

	// Should have 2 attributes with diversity: db.name (3 values, 2 lost) and db.statement (3 values, 2 lost)
	assert.Len(t, result.diverse, 2, "should have 2 diverse attributes")
	assert.Empty(t, result.missing, "no missing attributes")

	// Verify sorted by uniqueValues descending (both have 2 lost values)
	assert.Equal(t, 2, result.diverse[0].uniqueValues)
	assert.Equal(t, 2, result.diverse[1].uniqueValues)

	// Keys should be sorted alphabetically when uniqueValues are equal
	keys := []string{result.diverse[0].key, result.diverse[1].key}
	assert.Contains(t, keys, "db.name")
	assert.Contains(t, keys, "db.statement")
}

func TestAnalyzeAttributeLoss_MixedDiversity(t *testing.T) {
	// Some attributes have diversity, others don't
	nodes := createTestSpanNodes(t, []map[string]string{
		{"db.operation": "select", "http.method": "GET", "http.route": "/api/users"},
		{"db.operation": "select", "http.method": "POST", "http.route": "/api/users"},
		{"db.operation": "select", "http.method": "PUT", "http.route": "/api/users"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])

	// Only http.method has diversity (3 values, 2 lost)
	// db.operation and http.route have identical values
	assert.Len(t, result.diverse, 1, "should have 1 diverse attribute")
	assert.Equal(t, "http.method", result.diverse[0].key)
	assert.Equal(t, 2, result.diverse[0].uniqueValues)
	assert.Empty(t, result.missing)
}

func TestAnalyzeAttributeLoss_SortOrder(t *testing.T) {
	// Test that results are sorted by uniqueValues descending, then key ascending
	nodes := createTestSpanNodes(t, []map[string]string{
		{"a": "1", "b": "1", "c": "1"},
		{"a": "2", "b": "2", "c": "1"},
		{"a": "3", "b": "2", "c": "1"},
		{"a": "4", "b": "2", "c": "1"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])

	// a has 4 unique values (3 lost), b has 2 unique values (1 lost), c has 1 (no diversity)
	assert.Len(t, result.diverse, 2)
	assert.Equal(t, "a", result.diverse[0].key)
	assert.Equal(t, 3, result.diverse[0].uniqueValues) // 4 - 1 = 3 lost
	assert.Equal(t, "b", result.diverse[1].key)
	assert.Equal(t, 1, result.diverse[1].uniqueValues) // 2 - 1 = 1 lost
}

func TestAnalyzeAttributeLoss_SingleNode(t *testing.T) {
	// Single node should return empty (no aggregation happening)
	nodes := createTestSpanNodes(t, []map[string]string{
		{"db.operation": "select"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])
	assert.True(t, result.isEmpty(), "single node should return empty")
}

func TestAnalyzeAttributeLoss_EmptyNodes(t *testing.T) {
	result := analyzeAttributeLoss(nil, nil)
	assert.True(t, result.isEmpty())

	result = analyzeAttributeLoss([]*spanNode{}, nil)
	assert.True(t, result.isEmpty())
}

func TestAnalyzeAttributeLoss_MissingAttributes(t *testing.T) {
	// Attributes not present in ALL spans should be in 'missing'
	// span1 (template) has {a, b}, span2 has {a, c}, span3 has {a, b}
	// 'a' is in all with diversity -> diverse (2 lost)
	// 'b' missing from span2, but template has it -> 1 lost (value "2")
	// 'c' missing from span1 (template) and span3 -> 1 lost (all values)
	nodes := createTestSpanNodes(t, []map[string]string{
		{"a": "1", "b": "1"},
		{"a": "2", "c": "1"},
		{"a": "3", "b": "2"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])

	// 'a' has 3 unique values, present in all -> diverse (2 lost)
	assert.Len(t, result.diverse, 1)
	assert.Equal(t, "a", result.diverse[0].key)
	assert.Equal(t, 2, result.diverse[0].uniqueValues) // 3 - 1 = 2 lost

	// 'b' and 'c' are missing from some spans -> missing
	assert.Len(t, result.missing, 2)

	// 'b': template has b=1, other value is b=2 -> 1 lost
	// 'c': template lacks c, value c=1 exists -> 1 lost
	// Both have 1 lost, sorted alphabetically
	assert.Equal(t, "b", result.missing[0].key)
	assert.Equal(t, 1, result.missing[0].uniqueValues)
	assert.Equal(t, "c", result.missing[1].key)
	assert.Equal(t, 1, result.missing[1].uniqueValues)
}

func TestAnalyzeAttributeLoss_AllMissingFromSome(t *testing.T) {
	// When each span has a unique attribute, all should be in 'missing'
	// span1 (template) has {a}, span2 has {b}, span3 has {c}
	// Template has 'a', so 'a' has 0 lost (only 1 value, template keeps it)
	// Template lacks 'b' and 'c', so they lose all their values (1 each)
	nodes := createTestSpanNodes(t, []map[string]string{
		{"a": "1"},
		{"b": "1"},
		{"c": "1"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])

	// No attribute is present in all spans, so diverse should be empty
	assert.Empty(t, result.diverse)

	// 'a' has 1 value, template has it -> 0 lost (not reported)
	// 'b' and 'c' have 1 value each, template lacks them -> 1 lost each
	assert.Len(t, result.missing, 2, "b and c missing from template")
	for _, attr := range result.missing {
		assert.Equal(t, 1, attr.uniqueValues, "1 value lost for %s", attr.key)
		assert.NotEqual(t, "a", attr.key, "a should not be in missing (template has it, no loss)")
	}
}

func TestAnalyzeAttributeLoss_BothDiverseAndMissing(t *testing.T) {
	// Test that a single aggregation can have both diverse and missing attributes
	// span1 (template): {a:1, b:1}
	// span2: {a:2, b:1, c:1}  <- c only here
	// span3: {a:3, b:1, c:2}  <- c only here with different value
	// Result: a is diverse (2 lost), c is missing and template lacks it (2 lost)
	nodes := createTestSpanNodes(t, []map[string]string{
		{"a": "1", "b": "1"},
		{"a": "2", "b": "1", "c": "1"},
		{"a": "3", "b": "1", "c": "2"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])

	// 'a' has 3 unique values, present in all -> diverse (2 lost)
	assert.Len(t, result.diverse, 1)
	assert.Equal(t, "a", result.diverse[0].key)
	assert.Equal(t, 2, result.diverse[0].uniqueValues) // 3 - 1 = 2 lost

	// 'c' missing from template, has 2 values -> 2 lost
	assert.Len(t, result.missing, 1)
	assert.Equal(t, "c", result.missing[0].key)
	assert.Equal(t, 2, result.missing[0].uniqueValues)

	// 'b' has 1 value, present in all -> no loss
	assert.False(t, result.isEmpty(), "should have both diverse and missing")
}

func TestAnalyzeAttributeLoss_MissingWithTemplateHavingIt(t *testing.T) {
	// When template has the attribute, only non-template values are lost
	// span1 (template): {a:1, c:1}
	// span2: {a:2}            <- c missing
	// span3: {a:3}            <- c missing
	// Result: a is diverse (2 lost), c is missing but template has it (0 lost)
	nodes := createTestSpanNodes(t, []map[string]string{
		{"a": "1", "c": "1"},
		{"a": "2"},
		{"a": "3"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])

	// 'a' has 3 unique values -> 2 lost
	assert.Len(t, result.diverse, 1)
	assert.Equal(t, "a", result.diverse[0].key)
	assert.Equal(t, 2, result.diverse[0].uniqueValues)

	// 'c' missing from others but template has c=1 -> 0 lost (not reported)
	assert.Empty(t, result.missing, "c preserved by template")
}

func TestAnalyzeAttributeLoss_EmptyStringValues(t *testing.T) {
	// Empty string is a valid distinct value
	nodes := createTestSpanNodes(t, []map[string]string{
		{"a": "", "b": "value"},
		{"a": "non-empty", "b": ""},
		{"a": "", "b": "value"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])

	// 'a' has 2 unique values: "" and "non-empty" -> 1 lost
	// 'b' has 2 unique values: "value" and "" -> 1 lost
	assert.Len(t, result.diverse, 2)
	assert.Empty(t, result.missing)

	// Both should show 1 lost value (2 unique - 1 kept = 1 lost)
	for _, attr := range result.diverse {
		assert.Equal(t, 1, attr.uniqueValues, "attribute %s should have 1 lost value", attr.key)
	}
}

func TestFormatAttributeCardinality_Empty(t *testing.T) {
	result := formatAttributeCardinality(nil)
	assert.Empty(t, result)

	result = formatAttributeCardinality([]attributeCardinality{})
	assert.Empty(t, result)
}

func TestFormatAttributeCardinality_Single(t *testing.T) {
	attrs := []attributeCardinality{
		{key: "db.statement", uniqueValues: 12},
	}

	result := formatAttributeCardinality(attrs)
	assert.Equal(t, "db.statement(12)", result)
}

func TestFormatAttributeCardinality_Multiple(t *testing.T) {
	attrs := []attributeCardinality{
		{key: "db.statement", uniqueValues: 12},
		{key: "db.system", uniqueValues: 3},
		{key: "http.route", uniqueValues: 2},
	}

	result := formatAttributeCardinality(attrs)
	assert.Equal(t, "db.statement(12),db.system(3),http.route(2)", result)
}

func TestFormatAttributeCardinality_Truncation(t *testing.T) {
	// Create more than maxLostAttributesEntries (10) entries
	attrs := make([]attributeCardinality, 15)
	for i := range 15 {
		attrs[i] = attributeCardinality{
			key:          "attr" + strconv.Itoa(i),
			uniqueValues: 15 - i, // descending order
		}
	}

	result := formatAttributeCardinality(attrs)

	// Should only have first 10 entries plus ",..."
	assert.Contains(t, result, "attr0(15)")
	assert.Contains(t, result, "attr9(6)")
	assert.NotContains(t, result, "attr10")
	assert.True(t, strings.HasSuffix(result, ",..."), "should end with truncation indicator")
}

func TestFormatAttributeCardinality_ExactlyMax(t *testing.T) {
	// Exactly maxLostAttributesEntries (10) entries should not be truncated
	attrs := make([]attributeCardinality, 10)
	for i := range 10 {
		attrs[i] = attributeCardinality{
			key:          "attr" + strconv.Itoa(i),
			uniqueValues: 10 - i,
		}
	}

	result := formatAttributeCardinality(attrs)

	assert.NotContains(t, result, "...", "should not have truncation indicator at exactly max")
	assert.Contains(t, result, "attr9(1)")
}

// Integration test: verify diverse_attributes is set on parent summary spans
func TestLeafSpanPruning_DiverseAttributesOnParentSummary(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.MaxParentDepth = -1 // Enable parent aggregation
	cfg.EnableAttributeLossAnalysis = true

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with parent spans that have different attributes
	td := createTestTraceWithDiverseParentAttributes(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Find the parent aggregated span
	handlerAgg, found := findSummarySpanByName(td, "handler")
	require.True(t, found, "handler summary should exist")

	// Check for diverse_attributes attribute
	diverseAttrs, exists := handlerAgg.Attributes().Get("aggregation.diverse_attributes")
	require.True(t, exists, "diverse_attributes should exist on parent summary span")

	// Verify the format contains user.id (which has different values)
	diverseAttrsStr := diverseAttrs.Str()
	assert.Contains(t, diverseAttrsStr, "user.id", "should contain user.id")
	assert.Contains(t, diverseAttrsStr, "(2)", "should show 2 lost values (3 unique - 1 kept)")
}

func TestLeafSpanPruning_NoAttributeLossOnIdenticalLeafSpans(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.EnableAttributeLossAnalysis = true
	cfg.MaxParentDepth = 0 // Disable parent aggregation

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with identical leaf spans
	td := createTestTraceWithLeafSpans(t, 3, "SELECT", map[string]string{"db.operation": "select"})

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Find the leaf summary span
	summarySpan, found := findSummarySpanByName(td, "SELECT")
	require.True(t, found, "SELECT summary should exist")

	// Leaf spans are grouped by identical attributes, so no loss attributes
	_, diverseExists := summarySpan.Attributes().Get("aggregation.diverse_attributes")
	_, missingExists := summarySpan.Attributes().Get("aggregation.missing_attributes")
	assert.False(t, diverseExists, "diverse_attributes should NOT exist on identical spans")
	assert.False(t, missingExists, "missing_attributes should NOT exist on identical spans")
}

// Integration test: verify diverse_attributes is set on leaf summary spans with diverse attributes
func TestLeafSpanPruning_DiverseAttributesOnLeafSummary(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.EnableAttributeLossAnalysis = true
	cfg.MaxParentDepth = 0                           // Disable parent aggregation
	cfg.GroupByAttributes = []string{"db.operation"} // Only group by db.operation

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with leaf spans that have same db.operation but different db.statement
	td := createTestTraceWithDiverseLeafAttributes(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Find the leaf summary span
	summarySpan, found := findSummarySpanByName(td, "db_query")
	require.True(t, found, "db_query summary should exist")

	// Leaf spans have diverse db.statement values
	diverseAttrs, exists := summarySpan.Attributes().Get("aggregation.diverse_attributes")
	require.True(t, exists, "diverse_attributes should exist on leaf summary span with diverse attributes")

	// Verify the format contains the attribute key with diversity
	diverseAttrsStr := diverseAttrs.Str()
	assert.Contains(t, diverseAttrsStr, "db.statement", "should contain db.statement")
	assert.Contains(t, diverseAttrsStr, "(2)", "should show 2 lost values (3 unique - 1 kept)")
}

// Integration test: verify missing_attributes is set when attributes are absent from some spans
func TestLeafSpanPruning_MissingAttributesOnLeafSummary(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.EnableAttributeLossAnalysis = true
	cfg.MaxParentDepth = 0
	cfg.GroupByAttributes = []string{"db.operation"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with leaf spans where some have extra attributes
	td := createTestTraceWithMissingLeafAttributes(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Find the leaf summary span
	summarySpan, found := findSummarySpanByName(td, "db_query")
	require.True(t, found, "db_query summary should exist")

	// Check for missing_attributes
	missingAttrs, exists := summarySpan.Attributes().Get("aggregation.missing_attributes")
	require.True(t, exists, "missing_attributes should exist")

	missingAttrsStr := missingAttrs.Str()
	assert.Contains(t, missingAttrsStr, "extra.attr", "should contain extra attributes (missing from some spans)")
}

// Helper function to create test span nodes
func createTestSpanNodes(t *testing.T, attrSets []map[string]string) []*spanNode {
	t.Helper()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	nodes := make([]*spanNode, 0, len(attrSets))
	for i, attrs := range attrSets {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("test")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))

		for k, v := range attrs {
			span.Attributes().PutStr(k, v)
		}

		nodes = append(nodes, &spanNode{
			span:       span,
			scopeSpans: ss,
		})
	}

	return nodes
}

// Helper to create trace with diverse parent attributes
func createTestTraceWithDiverseParentAttributes(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootSpanID)
	root.SetName("root")
	root.Status().SetCode(ptrace.StatusCodeOk)

	// 3 handler spans with different user.id attributes
	userIDs := []string{"user-001", "user-002", "user-003"}
	for i := range 3 {
		handlerID := pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0})
		handler := ss.Spans().AppendEmpty()
		handler.SetTraceID(traceID)
		handler.SetSpanID(handlerID)
		handler.SetParentSpanID(rootSpanID)
		handler.SetName("handler")
		handler.Status().SetCode(ptrace.StatusCodeOk)
		handler.Attributes().PutStr("user.id", userIDs[i])
		handler.Attributes().PutStr("http.method", "GET") // Same for all

		// Each handler has a leaf SELECT span (for aggregation trigger)
		selectSpan := ss.Spans().AppendEmpty()
		selectSpan.SetTraceID(traceID)
		selectSpan.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		selectSpan.SetParentSpanID(handlerID)
		selectSpan.SetName("SELECT")
		selectSpan.Status().SetCode(ptrace.StatusCodeOk)
		selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
		selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

// Helper to create trace with diverse leaf attributes (same grouping key, different other attributes)
func createTestTraceWithDiverseLeafAttributes(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parent := ss.Spans().AppendEmpty()
	parent.SetTraceID(traceID)
	parent.SetSpanID(parentSpanID)
	parent.SetName("parent")
	parent.Status().SetCode(ptrace.StatusCodeOk)

	// 3 leaf spans with same db.operation (grouping key) but different db.statement
	statements := []string{
		"SELECT * FROM users WHERE id=1",
		"SELECT * FROM users WHERE id=2",
		"SELECT * FROM orders WHERE user_id=1",
	}
	for i := range 3 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
		span.Attributes().PutStr("db.operation", "select")      // Same for all (grouping key)
		span.Attributes().PutStr("db.statement", statements[i]) // Different for each
	}

	return td
}

// Helper to create trace with missing leaf attributes (some spans have extra attrs)
func createTestTraceWithMissingLeafAttributes(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parent := ss.Spans().AppendEmpty()
	parent.SetTraceID(traceID)
	parent.SetSpanID(parentSpanID)
	parent.SetName("parent")
	parent.Status().SetCode(ptrace.StatusCodeOk)

	// 3 leaf spans with varying attribute presence
	// Regardless of which span becomes template, some attributes will be missing
	for i := range 3 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
		span.Attributes().PutStr("db.operation", "select") // Same for all

		// Each span has a different extra attribute
		// This ensures missing_attributes is always set regardless of template selection
		switch i {
		case 0:
			span.Attributes().PutStr("extra.attr0", "value0")
		case 1:
			span.Attributes().PutStr("extra.attr1", "value1")
		case 2:
			span.Attributes().PutStr("extra.attr2", "value2")
		}
	}

	return td
}

// Test that attribute loss analysis is skipped when EnableAttributeLossAnalysis is false
func TestLeafSpanPruning_AttributeLossAnalysisDisabled(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.EnableAttributeLossAnalysis = false // Explicitly disabled

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with spans that WOULD have diverse attributes if analysis was enabled
	td := createTestTraceWithDiverseLeafAttributes(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Find the summary span
	summarySpan, found := findSummarySpanByName(td, "db_query")
	require.True(t, found, "db_query summary should exist")

	// Verify NO attribute loss attributes are added when analysis is disabled
	_, diverseExists := summarySpan.Attributes().Get("aggregation.diverse_attributes")
	_, missingExists := summarySpan.Attributes().Get("aggregation.missing_attributes")

	assert.False(t, diverseExists, "diverse_attributes should NOT exist when EnableAttributeLossAnalysis is false")
	assert.False(t, missingExists, "missing_attributes should NOT exist when EnableAttributeLossAnalysis is false")

	// Verify aggregation still works (span_count should exist)
	spanCount, exists := summarySpan.Attributes().Get("aggregation.span_count")
	assert.True(t, exists, "span_count should exist")
	assert.Equal(t, int64(3), spanCount.Int())
}
