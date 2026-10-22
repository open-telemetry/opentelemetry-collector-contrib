// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// --- parser tests ---

func TestNewTracesPartitioner_ResourceContext(t *testing.T) {
	expressions := []string{
		`resource.attributes["tenant.id"]`,
		`resource.attributes["service.name"]`,
	}
	p, err := newTracesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*resourceTracesPartitioner)
	assert.True(t, ok, "expected resourceTracesPartitioner when all expressions use resource context")
}

func TestNewTracesPartitioner_ScopeContext(t *testing.T) {
	expressions := []string{`scope.name`}
	p, err := newTracesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*scopeTracesPartitioner)
	assert.True(t, ok, "expected scopeTracesPartitioner when expression uses scope context")
}

func TestNewTracesPartitioner_SpanContext(t *testing.T) {
	expressions := []string{`span.name`}
	p, err := newTracesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*spanTracesPartitioner)
	assert.True(t, ok, "expected spanTracesPartitioner when expression uses span context")
}

func TestNewTracesPartitioner_OTelColContext(t *testing.T) {
	expressions := []string{`otelcol.client.metadata["x-tenant-id"][0]`}
	p, err := newTracesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*otelcolTracesPartitioner)
	assert.True(t, ok, "expected otelcolTracesPartitioner when expression uses otelcol context")
}

func TestNewTracesPartitioner_MixedResourceAndScope(t *testing.T) {
	expressions := []string{
		`resource.attributes["tenant.id"]`,
		`scope.name`,
	}
	p, err := newTracesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*scopeTracesPartitioner)
	assert.True(t, ok, "expected scopeTracesPartitioner for mixed resource+scope")
}

func TestNewTracesPartitioner_MixedContextPromotesToSpan(t *testing.T) {
	expressions := []string{
		`resource.attributes["tenant.id"]`,
		`span.name`,
	}
	p, err := newTracesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*spanTracesPartitioner)
	assert.True(t, ok, "expected spanTracesPartitioner when mixed resource+span contexts used")
}

func TestNewTracesPartitioner_RejectsSpanEvent(t *testing.T) {
	expressions := []string{`spanevent.name`}
	_, err := newTracesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	assert.Error(t, err, "spanevent context should be rejected")
}

func TestNewTracesPartitioner_InvalidExpression(t *testing.T) {
	expressions := []string{`not_a_valid_expression(`}
	_, err := newTracesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	assert.Error(t, err)
}

// --- partitioner tests ---

func TestPartitionTraces_SinglePartition(t *testing.T) {
	traces := ptrace.NewTraces()
	for range 2 {
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("tenant.id", "t1")
		rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("op")
	}

	result := partitionTraces(t, traces,
		`resource.attributes["tenant.id"]`,
	)
	assert.Len(t, result, 1)
	assert.Equal(t, 2, result[0].data.ResourceSpans().Len())
}

func TestPartitionTraces_MultiplePartitions(t *testing.T) {
	traces := ptrace.NewTraces()
	for _, tenant := range []string{"t1", "t2"} {
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("tenant.id", tenant)
		rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("op")
	}

	result := partitionTraces(t, traces,
		`resource.attributes["tenant.id"]`,
	)
	require.Len(t, result, 2)

	tenants := make(map[string]bool)
	for _, pt := range result {
		require.Len(t, pt.values, 1)
		tenants[pt.values[0].value] = true
	}
	assert.True(t, tenants["t1"])
	assert.True(t, tenants["t2"])
}

func TestPartitionTraces_SpanPartitioning(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test")
	ss := rs.ScopeSpans().AppendEmpty()

	for _, name := range []string{"op-a", "op-b", "op-a"} {
		ss.Spans().AppendEmpty().SetName(name)
	}

	result := partitionTraces(t, traces,
		`span.name`,
	)
	require.Len(t, result, 2)

	counts := make(map[string]int)
	for _, pt := range result {
		require.Len(t, pt.values, 1)
		counts[pt.values[0].value] = pt.data.SpanCount()
	}
	assert.Equal(t, 2, counts["op-a"])
	assert.Equal(t, 1, counts["op-b"])
}

func TestPartitionTraces_ScopePartitioning(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test")

	ss1 := rs.ScopeSpans().AppendEmpty()
	ss1.Scope().SetName("scope-a")
	ss1.Spans().AppendEmpty().SetName("op1")
	ss1.Spans().AppendEmpty().SetName("op2")

	ss2 := rs.ScopeSpans().AppendEmpty()
	ss2.Scope().SetName("scope-b")
	ss2.Spans().AppendEmpty().SetName("op3")

	result := partitionTraces(t, traces,
		`scope.name`,
	)
	require.Len(t, result, 2)

	counts := make(map[string]int)
	for _, pt := range result {
		require.Len(t, pt.values, 1)
		counts[pt.values[0].value] = pt.data.SpanCount()
	}
	assert.Equal(t, 2, counts["scope-a"])
	assert.Equal(t, 1, counts["scope-b"])
}

func TestPartitionTraces_NilAttributeValue(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("other", "value")
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("op")

	result := partitionTraces(t, traces,
		`resource.attributes["missing"]`,
	)
	require.Len(t, result, 1)
	require.Len(t, result[0].values, 1)
	assert.True(t, result[0].values[0].isNil)
}

func TestPartitionTraces_PreservesSchemaURL(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl("https://example.com/resource-schema")
	rs.Resource().Attributes().PutStr("service.name", "test")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.SetSchemaUrl("https://example.com/scope-schema")
	ss.Scope().SetName("s")
	ss.Spans().AppendEmpty().SetName("op")

	result := partitionTraces(t, traces,
		`scope.name`,
	)
	require.Len(t, result, 1)
	destRS := result[0].data.ResourceSpans().At(0)
	assert.Equal(t, "https://example.com/resource-schema", destRS.SchemaUrl())
	assert.Equal(t, "https://example.com/scope-schema", destRS.ScopeSpans().At(0).SchemaUrl())
}

func TestPartitionTraces_SpanPartitionerDedup(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("s")
	for _, name := range []string{"op-a", "op-b", "op-a"} {
		ss.Spans().AppendEmpty().SetName(name)
	}

	result := partitionTraces(t, traces,
		`span.name`,
	)
	require.Len(t, result, 2)
	for _, pt := range result {
		assert.Equal(t, 1, pt.data.ResourceSpans().Len())
		assert.Equal(t, 1, pt.data.ResourceSpans().At(0).ScopeSpans().Len())
	}
}

func TestPartitionTraces_EmptyInput(t *testing.T) {
	result := partitionTraces(t, ptrace.NewTraces(),
		`resource.attributes["tenant.id"]`,
	)
	assert.Empty(t, result)
}

func partitionTraces(t *testing.T, traces ptrace.Traces, expressions ...string) []partitionedTraces {
	t.Helper()
	p, err := newTracesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	result, err := p.partitionTraces(t.Context(), traces)
	require.NoError(t, err)
	return result
}
