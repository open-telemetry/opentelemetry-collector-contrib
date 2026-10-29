// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestAnalyzeAttributeLoss_NoDiversity(t *testing.T) {
	nodes := createTestSpanNodes(t, []map[string]string{
		{"db.operation": "select", "db.name": "users"},
		{"db.operation": "select", "db.name": "users"},
		{"db.operation": "select", "db.name": "users"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])
	assert.True(t, result.isEmpty(), "no loss expected when all values are identical")
}

func TestAnalyzeAttributeLoss_WithDiversity(t *testing.T) {
	nodes := createTestSpanNodes(t, []map[string]string{
		{"db.operation": "select", "db.name": "users", "db.statement": "SELECT * FROM users"},
		{"db.operation": "select", "db.name": "orders", "db.statement": "SELECT * FROM orders"},
		{"db.operation": "select", "db.name": "products", "db.statement": "SELECT * FROM products"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])
	require.Len(t, result.diverse, 2)
	assert.Empty(t, result.missing)
	assert.Equal(t, 2, result.diverse[0].uniqueValues)
	assert.Equal(t, 2, result.diverse[1].uniqueValues)
	assert.Equal(t, []string{"db.name", "db.statement"}, []string{result.diverse[0].key, result.diverse[1].key})
}

func TestAnalyzeAttributeLoss_MixedDiversity(t *testing.T) {
	nodes := createTestSpanNodes(t, []map[string]string{
		{"db.operation": "select", "http.method": "GET", "http.route": "/api/users"},
		{"db.operation": "select", "http.method": "POST", "http.route": "/api/users"},
		{"db.operation": "select", "http.method": "PUT", "http.route": "/api/users"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])
	require.Len(t, result.diverse, 1)
	assert.Equal(t, "http.method", result.diverse[0].key)
	assert.Equal(t, 2, result.diverse[0].uniqueValues)
	assert.Empty(t, result.missing)
}

func TestAnalyzeAttributeLoss_SortOrder(t *testing.T) {
	nodes := createTestSpanNodes(t, []map[string]string{
		{"a": "1", "b": "1", "c": "1"},
		{"a": "2", "b": "2", "c": "1"},
		{"a": "3", "b": "2", "c": "1"},
		{"a": "4", "b": "2", "c": "1"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])
	require.Len(t, result.diverse, 2)
	assert.Equal(t, "a", result.diverse[0].key)
	assert.Equal(t, 3, result.diverse[0].uniqueValues)
	assert.Equal(t, "b", result.diverse[1].key)
	assert.Equal(t, 1, result.diverse[1].uniqueValues)
}

func TestAnalyzeAttributeLoss_SingleNode(t *testing.T) {
	nodes := createTestSpanNodes(t, []map[string]string{{"db.operation": "select"}})
	assert.True(t, analyzeAttributeLoss(nodes, nodes[0]).isEmpty())
}

func TestAnalyzeAttributeLoss_MissingAttributesTemplateHasAttribute(t *testing.T) {
	nodes := createTestSpanNodes(t, []map[string]string{
		{"a": "1", "b": "1"},
		{"a": "2"},
		{"a": "3"},
	})

	result := analyzeAttributeLoss(nodes, nodes[0])
	require.Len(t, result.diverse, 1)
	assert.Equal(t, "a", result.diverse[0].key)
	assert.Equal(t, 2, result.diverse[0].uniqueValues)
	assert.Empty(t, result.missing, "template keeps the only value of b, so no missing loss")
}

func TestAnalyzeAttributeLoss_MissingAttributesTemplateLacksAttribute(t *testing.T) {
	nodes := createTestSpanNodes(t, []map[string]string{
		{"a": "1", "b": "1"},
		{"a": "2"},
		{"a": "3"},
	})

	result := analyzeAttributeLoss(nodes, nodes[1])
	require.Len(t, result.diverse, 1)
	assert.Equal(t, "a", result.diverse[0].key)
	assert.Equal(t, 2, result.diverse[0].uniqueValues)
	require.Len(t, result.missing, 1)
	assert.Equal(t, "b", result.missing[0].key)
	assert.Equal(t, 1, result.missing[0].uniqueValues)
}

func TestAnalyzeAttributeLoss_TemplateSelectionAffectsMissingLoss(t *testing.T) {
	nodes := createTestSpanNodes(t, []map[string]string{
		{"stable": "x", "optional": "one"},
		{"stable": "y"},
		{"stable": "z"},
	})

	withOptionalTemplate := analyzeAttributeLoss(nodes, nodes[0])
	withoutOptionalTemplate := analyzeAttributeLoss(nodes, nodes[1])

	assert.Empty(t, withOptionalTemplate.missing)
	require.Len(t, withoutOptionalTemplate.missing, 1)
	assert.Equal(t, "optional", withoutOptionalTemplate.missing[0].key)
	assert.Equal(t, 1, withoutOptionalTemplate.missing[0].uniqueValues)
}

func TestAnalyzeAttributeLoss_LargeCardinality(t *testing.T) {
	const count = 25
	attrSets := make([]map[string]string, 0, count)
	for i := range count {
		attrSets = append(attrSets, map[string]string{
			"query.id": strconv.Itoa(i),
			"fixed":    "same",
		})
	}

	nodes := createTestSpanNodes(t, attrSets)
	result := analyzeAttributeLoss(nodes, nodes[0])

	require.Len(t, result.diverse, 1)
	assert.Equal(t, "query.id", result.diverse[0].key)
	assert.Equal(t, count-1, result.diverse[0].uniqueValues)
	assert.Empty(t, result.missing)
}

func TestAnalyzeAttributeLoss_FormatAttributeCardinality(t *testing.T) {
	assert.Empty(t, formatAttributeCardinality(nil))
	assert.Empty(t, formatAttributeCardinality([]attributeCardinality{}))

	single := formatAttributeCardinality([]attributeCardinality{{key: "db.statement", uniqueValues: 12}})
	assert.Equal(t, "db.statement(12)", single)

	multiple := formatAttributeCardinality([]attributeCardinality{
		{key: "db.statement", uniqueValues: 12},
		{key: "db.system", uniqueValues: 3},
		{key: "http.route", uniqueValues: 2},
	})
	assert.Equal(t, "db.statement(12),db.system(3),http.route(2)", multiple)

	attrs := make([]attributeCardinality, 15)
	for i := range 15 {
		attrs[i] = attributeCardinality{key: "attr" + strconv.Itoa(i), uniqueValues: 15 - i}
	}
	truncated := formatAttributeCardinality(attrs)
	assert.Contains(t, truncated, "attr0(15)")
	assert.Contains(t, truncated, "attr9(6)")
	assert.NotContains(t, truncated, "attr10")
	assert.True(t, strings.HasSuffix(truncated, ",..."))
}

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

		nodes = append(nodes, &spanNode{span: span, scopeSpans: ss})
	}

	return nodes
}
