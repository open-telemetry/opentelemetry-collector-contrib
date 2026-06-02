// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transactions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/traceutil"
)

func TestBuildSpanTreeSingleSpan(t *testing.T) {
	logger := zap.NewNop()
	span := createSpan("span1", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{}), 100)
	spans := []ptrace.Span{span}

	root := buildSpanTree(traceutil.BuildTraceTree(spans), logger)

	assert.NotNil(t, root)
	assert.Equal(t, span.SpanID(), root.Span.SpanID())
	assert.Empty(t, root.Children)
}

func TestBuildSpanTreeParentChild(t *testing.T) {
	logger := zap.NewNop()
	parentSpan := createSpan("parent", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{}), 100)
	childSpan := createSpan("child", pcommon.SpanID([8]byte{2}), parentSpan.SpanID(), 200)
	spans := []ptrace.Span{childSpan, parentSpan}

	root := buildSpanTree(traceutil.BuildTraceTree(spans), logger)

	assert.NotNil(t, root)
	assert.Equal(t, parentSpan.SpanID(), root.Span.SpanID())
	assert.Len(t, root.Children, 1)
	assert.Equal(t, childSpan.SpanID(), root.Children[0].Span.SpanID())
}

func TestBuildSpanTreeMultipleRoots(t *testing.T) {
	logger := zap.NewNop()
	root1 := createSpan("root1", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{}), 100)
	root2 := createSpan("root2", pcommon.SpanID([8]byte{2}), pcommon.SpanID([8]byte{}), 50)
	spans := []ptrace.Span{root1, root2}

	root := buildSpanTree(traceutil.BuildTraceTree(spans), logger)

	assert.NotNil(t, root)
	assert.Equal(t, root2.SpanID(), root.Span.SpanID())
}

func TestBuildSpanTreePrefersExplicitRootOverEarlierOrphan(t *testing.T) {
	logger := zap.NewNop()
	explicitRoot := createSpan("root", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{}), 100)
	orphan := createSpan("orphan", pcommon.SpanID([8]byte{2}), pcommon.SpanID([8]byte{9}), 50)
	child := createSpan("child", pcommon.SpanID([8]byte{3}), explicitRoot.SpanID(), 150)
	spans := []ptrace.Span{explicitRoot, orphan, child}

	root := buildSpanTree(traceutil.BuildTraceTree(spans), logger)

	assert.NotNil(t, root)
	assert.Equal(t, pcommon.SpanID([8]byte{1}), root.Span.SpanID())
	assert.Len(t, root.Children, 1)
	assert.Equal(t, child.SpanID(), root.Children[0].Span.SpanID())
}

func TestBuildSpanTreeFallsBackToEarliestOrphanWhenNoExplicitRootExists(t *testing.T) {
	logger := zap.NewNop()
	span1 := createSpan("span1", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{3}), 100)
	span2 := createSpan("span2", pcommon.SpanID([8]byte{2}), pcommon.SpanID([8]byte{3}), 50)
	spans := []ptrace.Span{span1, span2}

	root := buildSpanTree(traceutil.BuildTraceTree(spans), logger)

	assert.NotNil(t, root)
	assert.Equal(t, span2.SpanID(), root.Span.SpanID())
}

func TestBuildSpanTreeUsesSpanIDTieBreakerForEqualStartRoots(t *testing.T) {
	logger := zap.NewNop()
	root1 := createSpan("root1", pcommon.SpanID([8]byte{2}), pcommon.SpanID([8]byte{}), 100)
	root2 := createSpan("root2", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{}), 100)

	root := buildSpanTree(traceutil.BuildTraceTree([]ptrace.Span{root1, root2}), logger)

	assert.NotNil(t, root)
	assert.Equal(t, root2.SpanID(), root.Span.SpanID())
}

func TestBuildSpanTreeEmpty(t *testing.T) {
	logger := zap.NewNop()
	spans := []ptrace.Span{}

	root := buildSpanTree(traceutil.BuildTraceTree(spans), logger)

	assert.Nil(t, root)
}

func createSpan(name string, spanID, parentSpanID pcommon.SpanID, startTime int64) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetName(name)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, startTime)))
	return span
}
