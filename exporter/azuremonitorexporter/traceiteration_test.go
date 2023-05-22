// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type mockVisitor struct {
	mock.Mock
}

func (v *mockVisitor) visit(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) (ok bool) {
	args := v.Called(resource, scope, span)
	return args.Bool(0)
}

// Tests the iteration logic over a ptrace.Traces type when no ResourceSpans are provided
func TestTraceDataIterationNoResourceSpans(t *testing.T) {
	traces := ptrace.NewTraces()

	visitor := getMockVisitor(true)

	Accept(traces, visitor)

	visitor.AssertNumberOfCalls(t, "visit", 0)
}

// Tests the iteration logic over a ptrace.Traces type when a ResourceSpans is nil
func TestTraceDataIterationResourceSpansIsEmpty(t *testing.T) {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty()

	visitor := getMockVisitor(true)

	Accept(traces, visitor)

	visitor.AssertNumberOfCalls(t, "visit", 0)
}

// Tests the iteration logic over a ptrace.Traces type when ScopeSpans is nil
func TestTraceDataIterationScopeSpansIsEmpty(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty()

	visitor := getMockVisitor(true)

	Accept(traces, visitor)

	visitor.AssertNumberOfCalls(t, "visit", 0)
}

// Tests the iteration logic over a ptrace.Traces type when there are no Spans
func TestTraceDataIterationNoSpans(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty()

	visitor := getMockVisitor(true)

	Accept(traces, visitor)

	visitor.AssertNumberOfCalls(t, "visit", 0)
}

// Tests the iteration logic if the visitor returns true
func TestTraceDataIterationNoShortCircuit(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ilss := rs.ScopeSpans().AppendEmpty()
	ilss.Spans().AppendEmpty()
	ilss.Spans().AppendEmpty()

	visitor := getMockVisitor(true)

	Accept(traces, visitor)

	visitor.AssertNumberOfCalls(t, "visit", 2)
}

// Tests the iteration logic short circuit if the visitor returns false
func TestTraceDataIterationShortCircuit(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ilss := rs.ScopeSpans().AppendEmpty()
	ilss.Spans().AppendEmpty()
	ilss.Spans().AppendEmpty()

	visitor := getMockVisitor(false)

	Accept(traces, visitor)

	visitor.AssertNumberOfCalls(t, "visit", 1)
}

func getMockVisitor(returns bool) *mockVisitor {
	visitor := new(mockVisitor)
	visitor.On("visit", mock.Anything, mock.Anything, mock.Anything).Return(returns)
	return visitor
}
