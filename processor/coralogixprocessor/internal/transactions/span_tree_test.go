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
)

func TestBuildSpanTreeSingleSpan(t *testing.T) {
	logger := zap.NewNop()
	span := createSpan("span1", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{}), 100)
	spans := []ptrace.Span{span}

	root := buildSpanTree(spans, logger)

	assert.NotNil(t, root)
	assert.Equal(t, span.SpanID(), root.span.SpanID())
	assert.Empty(t, root.children)
}

func TestBuildSpanTreeParentChild(t *testing.T) {
	logger := zap.NewNop()
	parentSpan := createSpan("parent", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{}), 100)
	childSpan := createSpan("child", pcommon.SpanID([8]byte{2}), parentSpan.SpanID(), 200)
	spans := []ptrace.Span{childSpan, parentSpan}

	root := buildSpanTree(spans, logger)

	assert.NotNil(t, root)
	assert.Equal(t, parentSpan.SpanID(), root.span.SpanID())
	assert.Len(t, root.children, 1)
	assert.Equal(t, childSpan.SpanID(), root.children[0].span.SpanID())
}

func TestBuildSpanTreeMultipleRoots(t *testing.T) {
	logger := zap.NewNop()
	root1 := createSpan("root1", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{}), 100)
	root2 := createSpan("root2", pcommon.SpanID([8]byte{2}), pcommon.SpanID([8]byte{}), 50)
	spans := []ptrace.Span{root1, root2}

	root := buildSpanTree(spans, logger)

	assert.NotNil(t, root)
	assert.Equal(t, root2.SpanID(), root.span.SpanID())
}

func TestBuildSpanTreeNoRoot(t *testing.T) {
	logger := zap.NewNop()
	span1 := createSpan("span1", pcommon.SpanID([8]byte{1}), pcommon.SpanID([8]byte{3}), 100)
	span2 := createSpan("span2", pcommon.SpanID([8]byte{2}), pcommon.SpanID([8]byte{3}), 50)
	spans := []ptrace.Span{span1, span2}

	root := buildSpanTree(spans, logger)

	assert.NotNil(t, root)
	assert.Equal(t, span2.SpanID(), root.span.SpanID())
}

func TestBuildSpanTreeEmpty(t *testing.T) {
	logger := zap.NewNop()
	spans := []ptrace.Span{}

	root := buildSpanTree(spans, logger)

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
