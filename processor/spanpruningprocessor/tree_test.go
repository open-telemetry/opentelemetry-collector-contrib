// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadatatest"
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

			tel := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			tp, err := NewFactory().CreateTraces(t.Context(), metadatatest.NewSettings(tel), cfg, consumertest.NewNop())
			require.NoError(t, err)

			done := make(chan error, 1)
			go func() { done <- tp.ConsumeTraces(t.Context(), td) }()
			select {
			case consumeErr := <-done:
				require.NoError(t, consumeErr)
			case <-time.After(5 * time.Second):
				t.Fatal("processing a trace with a parent cycle did not return")
			}
			assert.Equal(t, before, countSpans(td), "a trace with a parent cycle is left untouched")
			metadatatest.AssertEqualProcessorSpanpruningInvalidTraces(t, tel,
				[]metricdata.DataPoint[int64]{{Value: 1}}, metricdatatest.IgnoreTimestamp())
			_, err = tel.GetMetric("otelcol_processor_spanpruning_traces_processed")
			assert.Error(t, err, "an unpruned trace is not counted as processed")
		})
	}
}

func TestAssignDepths(t *testing.T) {
	root := &spanNode{}
	child := &spanNode{parent: root}
	grandchild := &spanNode{parent: child}
	orphan := &spanNode{}
	acyclic := map[pcommon.SpanID]*spanNode{{1}: grandchild, {2}: child, {3}: root, {4}: orphan}
	require.NoError(t, assignDepths(acyclic))
	assert.Equal(t, 0, root.depth())
	assert.Equal(t, 1, child.depth())
	assert.Equal(t, 2, grandchild.depth())
	assert.Equal(t, 0, orphan.depth())

	withID := func(b byte) *spanNode {
		span := ptrace.NewSpan()
		span.SetSpanID(pcommon.SpanID{b})
		return &spanNode{span: span}
	}
	a, b, c := withID(0xa0), withID(0xb0), withID(0xc0)
	a.parent, b.parent, c.parent = b, c, a
	under := &spanNode{parent: a}
	cyclic := map[pcommon.SpanID]*spanNode{{1}: root, {2}: child, {0xa0}: a, {0xb0}: b, {0xc0}: c, {7}: under}
	err := assignDepths(cyclic)
	require.Error(t, err, "a three span cycle")
	// Which cycle member is named depends on map iteration order.
	assert.Contains(t, []string{
		"parent cycle through span " + a.span.SpanID().String(),
		"parent cycle through span " + b.span.SpanID().String(),
		"parent cycle through span " + c.span.SpanID().String(),
	}, err.Error())

	self := withID(1)
	self.parent = self
	assert.EqualError(t, assignDepths(map[pcommon.SpanID]*spanNode{{1}: self}),
		"parent cycle through span "+pcommon.SpanID{1}.String(), "a self parent")
}
