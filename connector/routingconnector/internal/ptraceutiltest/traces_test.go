// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ptraceutiltest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestNewTraces(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		expected := ptrace.NewTraces()
		assert.NoError(t, ptracetest.CompareTraces(expected, ptraceutiltest.NewTraces("", "", "", "")))
		assert.NoError(t, ptracetest.CompareTraces(expected, ptraceutiltest.NewTracesFromOpts()))
	})

	t.Run("simple", func(t *testing.T) {
		expected := func() ptrace.Traces {
			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scopeB") // resourceA.scopeB
			s := ss.Spans().AppendEmpty()
			s.SetName("spanC") // resourceA.scopeB.spanC
			se := s.Events().AppendEmpty()
			se.Attributes().PutStr("spanEventName", "spanEventD") // resourceA.scopeB.spanC.spanEventD
			return td
		}()
		fromOpts := ptraceutiltest.NewTracesFromOpts(
			ptraceutiltest.WithResource('A',
				ptraceutiltest.WithScope('B',
					ptraceutiltest.WithSpan('C', "D"),
				),
			),
		)
		assert.NoError(t, ptracetest.CompareTraces(expected, ptraceutiltest.NewTraces("A", "B", "C", "D")))
		assert.NoError(t, ptracetest.CompareTraces(expected, fromOpts))
	})

	t.Run("two_resources", func(t *testing.T) {
		expected := func() ptrace.Traces {
			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scopeC") // resourceA.scopeC
			s := ss.Spans().AppendEmpty()
			s.SetName("spanD") // resourceA.scopeC.spanD
			se := s.Events().AppendEmpty()
			se.Attributes().PutStr("spanEventName", "spanEventE") // resourceA.scopeC.spanD.spanEventE
			rs = td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("resourceName", "resourceB") // resourceB
			ss = rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scopeC") // resourceB.scopeC
			s = ss.Spans().AppendEmpty()
			s.SetName("spanD") // resourceB.scopeC.spanD
			se = s.Events().AppendEmpty()
			se.Attributes().PutStr("spanEventName", "spanEventE") // resourceB.scopeC.spanD.spanEventE
			return td
		}()
		fromOpts := ptraceutiltest.NewTracesFromOpts(
			ptraceutiltest.WithResource('A',
				ptraceutiltest.WithScope('C',
					ptraceutiltest.WithSpan('D', "E"),
				),
			),
			ptraceutiltest.WithResource('B',
				ptraceutiltest.WithScope('C',
					ptraceutiltest.WithSpan('D', "E"),
				),
			),
		)
		assert.NoError(t, ptracetest.CompareTraces(expected, ptraceutiltest.NewTraces("AB", "C", "D", "E")))
		assert.NoError(t, ptracetest.CompareTraces(expected, fromOpts))
	})

	t.Run("two_scopes", func(t *testing.T) {
		expected := func() ptrace.Traces {
			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scopeB") // resourceA.scopeB
			s := ss.Spans().AppendEmpty()
			s.SetName("spanD") // resourceA.scopeB.spanD
			se := s.Events().AppendEmpty()
			se.Attributes().PutStr("spanEventName", "spanEventE") // resourceA.scopeB.spanD.spanEventE
			ss = rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scopeC") // resourceA.scopeC
			s = ss.Spans().AppendEmpty()
			s.SetName("spanD") // resourceA.scopeC.spanD
			se = s.Events().AppendEmpty()
			se.Attributes().PutStr("spanEventName", "spanEventE") // resourceA.scopeC.spanD.spanEventE
			return td
		}()
		fromOpts := ptraceutiltest.NewTracesFromOpts(
			ptraceutiltest.WithResource('A',
				ptraceutiltest.WithScope('B',
					ptraceutiltest.WithSpan('D', "E"),
				),
				ptraceutiltest.WithScope('C',
					ptraceutiltest.WithSpan('D', "E"),
				),
			),
		)
		assert.NoError(t, ptracetest.CompareTraces(expected, ptraceutiltest.NewTraces("A", "BC", "D", "E")))
		assert.NoError(t, ptracetest.CompareTraces(expected, fromOpts))
	})

	t.Run("two_spans", func(t *testing.T) {
		expected := func() ptrace.Traces {
			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scopeB") // resourceA.scopeB
			s := ss.Spans().AppendEmpty()
			s.SetName("spanC") // resourceA.scopeB.spanC
			se := s.Events().AppendEmpty()
			se.Attributes().PutStr("spanEventName", "spanEventE") // resourceA.scopeB.spanC.spanEventE
			s = ss.Spans().AppendEmpty()
			s.SetName("spanD") // resourceA.scopeB.spanD
			se = s.Events().AppendEmpty()
			se.Attributes().PutStr("spanEventName", "spanEventE") // resourceA.scopeB.spanD.spanEventE
			return td
		}()
		fromOpts := ptraceutiltest.NewTracesFromOpts(
			ptraceutiltest.WithResource('A',
				ptraceutiltest.WithScope('B',
					ptraceutiltest.WithSpan('C', "E"),
					ptraceutiltest.WithSpan('D', "E"),
				),
			),
		)
		assert.NoError(t, ptracetest.CompareTraces(expected, ptraceutiltest.NewTraces("A", "B", "CD", "E")))
		assert.NoError(t, ptracetest.CompareTraces(expected, fromOpts))
	})

	t.Run("two_spanevents", func(t *testing.T) {
		expected := func() ptrace.Traces {
			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scopeB") // resourceA.scopeB
			s := ss.Spans().AppendEmpty()
			s.SetName("spanC") // resourceA.scopeB.spanC
			se := s.Events().AppendEmpty()
			se.Attributes().PutStr("spanEventName", "spanEventD") // resourceA.scopeB.spanC.spanEventD
			se = s.Events().AppendEmpty()
			se.Attributes().PutStr("spanEventName", "spanEventE") // resourceA.scopeB.spanC.spanEventE
			return td
		}()
		fromOpts := ptraceutiltest.NewTracesFromOpts(
			ptraceutiltest.WithResource('A',
				ptraceutiltest.WithScope('B',
					ptraceutiltest.WithSpan('C', "DE"),
				),
			),
		)
		assert.NoError(t, ptracetest.CompareTraces(expected, ptraceutiltest.NewTraces("A", "B", "C", "DE")))
		assert.NoError(t, ptracetest.CompareTraces(expected, fromOpts))
	})
}
