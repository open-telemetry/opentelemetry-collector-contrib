// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
