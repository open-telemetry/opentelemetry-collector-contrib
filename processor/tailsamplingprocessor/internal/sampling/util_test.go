// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSetAttrOnScopeSpans_Empty(_ *testing.T) {
	traces := ptrace.NewTraces()
	traceData := &TraceData{
		ReceivedBatches: traces,
	}

	SetAttrOnScopeSpans(traceData, "test.attr", "value")
}

func TestSetAttrOnScopeSpans_Many(t *testing.T) {
	assertAttrExists := func(t *testing.T, attrs pcommon.Map, key string, value string) {
		v, ok := attrs.Get(key)
		assert.True(t, ok)
		assert.Equal(t, value, v.AsString())
	}

	traces := ptrace.NewTraces()

	rs1 := traces.ResourceSpans().AppendEmpty()
	ss1 := rs1.ScopeSpans().AppendEmpty()
	span1 := ss1.Spans().AppendEmpty()
	span2 := ss1.Spans().AppendEmpty()
	ss2 := rs1.ScopeSpans().AppendEmpty()
	span3 := ss2.Spans().AppendEmpty()
	rs2 := traces.ResourceSpans().AppendEmpty()
	ss3 := rs2.ScopeSpans().AppendEmpty()
	span4 := ss3.Spans().AppendEmpty()

	traceData := &TraceData{
		ReceivedBatches: traces,
	}

	SetAttrOnScopeSpans(traceData, "test.attr", "value")

	assertAttrExists(t, ss1.Scope().Attributes(), "test.attr", "value")
	assertAttrExists(t, ss2.Scope().Attributes(), "test.attr", "value")
	assertAttrExists(t, ss3.Scope().Attributes(), "test.attr", "value")

	_, ok := span1.Attributes().Get("test.attr")
	assert.False(t, ok)
	_, ok = span2.Attributes().Get("test.attr")
	assert.False(t, ok)
	_, ok = span3.Attributes().Get("test.attr")
	assert.False(t, ok)
	_, ok = span4.Attributes().Get("test.attr")
	assert.False(t, ok)
}

func BenchmarkSetAttrOnScopeSpans(b *testing.B) {
	for n := 0; n < b.N; n++ {
		traces := ptrace.NewTraces()

		for i := 0; i < 5; i++ {
			rs := traces.ResourceSpans().AppendEmpty()
			ss1 := rs.ScopeSpans().AppendEmpty()
			ss1.Spans().AppendEmpty()
			ss1.Spans().AppendEmpty()
			ss1.Spans().AppendEmpty()

			ss2 := rs.ScopeSpans().AppendEmpty()
			ss2.Spans().AppendEmpty()
			ss2.Spans().AppendEmpty()

			ss3 := rs.ScopeSpans().AppendEmpty()
			ss3.Spans().AppendEmpty()
		}

		traceData := &TraceData{
			ReceivedBatches: traces,
		}

		b.StartTimer()
		SetAttrOnScopeSpans(traceData, "test.attr", "value")
		b.StopTimer()
	}
}
