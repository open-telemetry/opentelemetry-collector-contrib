// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traceutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestBuildTraceTreeEmpty(t *testing.T) {
	tree := BuildTraceTree(nil)

	assert.Empty(t, tree.Roots)
	assert.Empty(t, tree.Nodes)
}

func TestBuildTraceTreeBuildsHierarchy(t *testing.T) {
	parent := newTraceTreeSpan(1, 0, 100, 0)
	child := newTraceTreeSpan(2, 20, 40, 1)

	tree := BuildTraceTree([]ptrace.Span{child, parent})

	require.Len(t, tree.Roots, 1)
	require.Len(t, tree.Nodes, 2)
	assert.Equal(t, parent.SpanID(), tree.Roots[0].Span.SpanID())
	require.Len(t, tree.Roots[0].Children, 1)
	assert.Equal(t, child.SpanID(), tree.Roots[0].Children[0].Span.SpanID())
	assert.Equal(t, tree.Roots[0], tree.Roots[0].Children[0].Parent)
}

func TestBuildTraceTreeTreatsMissingParentAsRoot(t *testing.T) {
	orphan := newTraceTreeSpan(2, 20, 40, 9)

	tree := BuildTraceTree([]ptrace.Span{orphan})

	require.Len(t, tree.Roots, 1)
	assert.Equal(t, orphan.SpanID(), tree.Roots[0].Span.SpanID())
	assert.Nil(t, tree.Roots[0].Parent)
	assert.Empty(t, tree.Roots[0].Children)
}

func newTraceTreeSpan(spanID byte, startNS, endNS int64, parentSpanID byte) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetTraceID(pcommon.TraceID([16]byte{1}))
	span.SetSpanID(pcommon.SpanID([8]byte{spanID}))
	if parentSpanID != 0 {
		span.SetParentSpanID(pcommon.SpanID([8]byte{parentSpanID}))
	}
	span.SetStartTimestamp(pcommon.Timestamp(startNS))
	span.SetEndTimestamp(pcommon.Timestamp(endNS))
	return span
}
