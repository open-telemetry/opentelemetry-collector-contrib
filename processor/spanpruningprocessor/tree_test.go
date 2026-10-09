// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

func TestParentCycleSkipsTrace(t *testing.T) {
	id := func(b byte) pcommon.SpanID { return pcommon.SpanID([8]byte{b, 0, 0, 0, 0, 0, 0, 0}) }

	tests := []struct {
		name    string
		parents map[byte]byte // span -> parent; each cycle gets five leaf children
	}{
		{name: "self parent", parents: map[byte]byte{1: 1}},
		{name: "two span cycle", parents: map[byte]byte{1: 2, 2: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := ptrace.NewTraces()
			ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
			addSpan := func(spanID, parentID pcommon.SpanID, name string) {
				span := ss.Spans().AppendEmpty()
				span.SetTraceID(testTraceID)
				span.SetSpanID(spanID)
				span.SetParentSpanID(parentID)
				span.SetName(name)
				span.SetStartTimestamp(pcommon.Timestamp(1_000_000_000))
				span.SetEndTimestamp(pcommon.Timestamp(1_002_000_000))
			}
			for s, p := range tt.parents {
				addSpan(id(s), id(p), "loop")
			}
			for i := range 5 {
				addSpan(id(byte(20+i)), id(1), "SELECT")
			}
			before := countSpans(td)

			cfg := NewFactory().CreateDefaultConfig().(*Config)
			tp, err := NewFactory().CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			require.NoError(t, err)

			done := make(chan error, 1)
			go func() { done <- tp.ConsumeTraces(t.Context(), td) }()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("processing a trace with a parent cycle did not return")
			}
			assert.Equal(t, before, countSpans(td), "a trace with a parent cycle is left untouched")
		})
	}
}

func TestHasParentCycle(t *testing.T) {
	root := &spanNode{}
	child := &spanNode{parent: root}
	root.children = []*spanNode{child}
	orphan := &spanNode{}
	acyclic := map[pcommon.SpanID]*spanNode{{1}: root, {2}: child, {3}: orphan}
	assert.False(t, hasParentCycle(acyclic))

	a, b := &spanNode{}, &spanNode{}
	a.parent, b.parent = b, a
	a.children, b.children = []*spanNode{b}, []*spanNode{a}
	under := &spanNode{parent: a}
	a.children = append(a.children, under)
	cyclic := map[pcommon.SpanID]*spanNode{{1}: root, {2}: child, {4}: a, {5}: b, {6}: under}
	assert.True(t, hasParentCycle(cyclic))
}
