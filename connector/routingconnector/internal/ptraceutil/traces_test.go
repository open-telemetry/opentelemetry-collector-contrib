// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ptraceutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestMoveResourcesIf(t *testing.T) {
	testCases := []struct {
		from       ptrace.Traces
		to         ptrace.Traces
		expectFrom ptrace.Traces
		expectTo   ptrace.Traces
		moveIf     func(ptrace.ResourceSpans) bool
		name       string
	}{
		{
			name: "move_none",
			moveIf: func(ptrace.ResourceSpans) bool {
				return false
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			to:         ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectTo:   ptrace.NewTraces(),
		},
		{
			name: "move_all",
			moveIf: func(ptrace.ResourceSpans) bool {
				return true
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			to:         ptrace.NewTraces(),
			expectFrom: ptrace.NewTraces(),
			expectTo:   ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
		},
		{
			name: "move_one",
			moveIf: func(rl ptrace.ResourceSpans) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA"
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			to:         ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTraces("B", "CD", "EF", "FG"),
			expectTo:   ptraceutiltest.NewTraces("A", "CD", "EF", "FG"),
		},
		{
			name: "move_to_preexisting",
			moveIf: func(rl ptrace.ResourceSpans) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			to:         ptraceutiltest.NewTraces("1", "2", "3", "4"),
			expectFrom: ptraceutiltest.NewTraces("A", "CD", "EF", "FG"),
			expectTo: func() ptrace.Traces {
				move := ptraceutiltest.NewTraces("B", "CD", "EF", "FG")
				moveTo := ptraceutiltest.NewTraces("1", "2", "3", "4")
				move.ResourceSpans().MoveAndAppendTo(moveTo.ResourceSpans())
				return moveTo
			}(),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ptraceutil.MoveResourcesIf(tt.from, tt.to, tt.moveIf)
			assert.NoError(t, ptracetest.CompareTraces(tt.expectFrom, tt.from), "from not modified as expected")
			assert.NoError(t, ptracetest.CompareTraces(tt.expectTo, tt.to), "to not as expected")
		})
	}
}

func TestMoveSpansWithContextIf(t *testing.T) {
	testCases := []struct {
		from       ptrace.Traces
		to         ptrace.Traces
		expectFrom ptrace.Traces
		expectTo   ptrace.Traces
		moveIf     func(ptrace.ResourceSpans, ptrace.ScopeSpans, ptrace.Span) bool
		name       string
	}{
		{
			name: "move_none",
			moveIf: func(ptrace.ResourceSpans, ptrace.ScopeSpans, ptrace.Span) bool {
				return false
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			to:         ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			expectTo:   ptrace.NewTraces(),
		},
		{
			name: "move_all",
			moveIf: func(ptrace.ResourceSpans, ptrace.ScopeSpans, ptrace.Span) bool {
				return true
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			to:         ptrace.NewTraces(),
			expectFrom: ptrace.NewTraces(),
			expectTo:   ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
		},
		{
			name: "move_all_from_one_resource",
			moveIf: func(rl ptrace.ResourceSpans, _ ptrace.ScopeSpans, _ ptrace.Span) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			to:         ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTraces("A", "CD", "EF", "GH"),
			expectTo:   ptraceutiltest.NewTraces("B", "CD", "EF", "GH"),
		},
		{
			name: "move_all_from_one_scope",
			moveIf: func(rl ptrace.ResourceSpans, sl ptrace.ScopeSpans, _ ptrace.Span) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && sl.Scope().Name() == "scopeC"
			},
			from: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			to:   ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTracesFromOpts(
				ptraceutiltest.Resource("A",
					ptraceutiltest.Scope("C",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
					ptraceutiltest.Scope("D",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
				ptraceutiltest.Resource("B",
					ptraceutiltest.Scope("D",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
			),
			expectTo: ptraceutiltest.NewTraces("B", "C", "EF", "GH"),
		},
		{
			name: "move_all_from_one_scope_in_each_resource",
			moveIf: func(_ ptrace.ResourceSpans, sl ptrace.ScopeSpans, _ ptrace.Span) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			to:         ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTraces("AB", "C", "EF", "GH"),
			expectTo:   ptraceutiltest.NewTraces("AB", "D", "EF", "GH"),
		},
		{
			name: "move_one",
			moveIf: func(rl ptrace.ResourceSpans, sl ptrace.ScopeSpans, m ptrace.Span) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA" && sl.Scope().Name() == "scopeD" && m.Name() == "spanF"
			},
			from: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			to:   ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTracesFromOpts(
				ptraceutiltest.Resource("A",
					ptraceutiltest.Scope("C",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
					ptraceutiltest.Scope("D",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
				ptraceutiltest.Resource("B",
					ptraceutiltest.Scope("C",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
					ptraceutiltest.Scope("D",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
			),
			expectTo: ptraceutiltest.NewTraces("A", "D", "F", "GH"),
		},
		{
			name: "move_one_from_each_scope",
			moveIf: func(_ ptrace.ResourceSpans, _ ptrace.ScopeSpans, m ptrace.Span) bool {
				return m.Name() == "spanE"
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			to:         ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTraces("AB", "CD", "F", "GH"),
			expectTo:   ptraceutiltest.NewTraces("AB", "CD", "E", "GH"),
		},
		{
			name: "move_one_from_each_scope_in_one_resource",
			moveIf: func(rl ptrace.ResourceSpans, _ ptrace.ScopeSpans, m ptrace.Span) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && m.Name() == "spanE"
			},
			from: ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			to:   ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTracesFromOpts(
				ptraceutiltest.Resource("A",
					ptraceutiltest.Scope("C",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
					ptraceutiltest.Scope("D",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
				ptraceutiltest.Resource("B",
					ptraceutiltest.Scope("C",
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
					ptraceutiltest.Scope("D",
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
			),
			expectTo: ptraceutiltest.NewTraces("B", "CD", "E", "GH"),
		},
		{
			name: "move_some_to_preexisting",
			moveIf: func(_ ptrace.ResourceSpans, sl ptrace.ScopeSpans, _ ptrace.Span) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "GH"),
			to:         ptraceutiltest.NewTraces("1", "2", "3", "4"),
			expectFrom: ptraceutiltest.NewTraces("AB", "C", "EF", "GH"),
			expectTo: ptraceutiltest.NewTracesFromOpts(
				ptraceutiltest.Resource("1", ptraceutiltest.Scope("2", ptraceutiltest.Span("3", ptraceutiltest.SpanEvent("4")))),
				ptraceutiltest.Resource("A",
					ptraceutiltest.Scope("D",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
				ptraceutiltest.Resource("B",
					ptraceutiltest.Scope("D",
						ptraceutiltest.Span("E", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
						ptraceutiltest.Span("F", ptraceutiltest.SpanEvent("G"), ptraceutiltest.SpanEvent("H")),
					),
				),
			),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ptraceutil.MoveSpansWithContextIf(tt.from, tt.to, tt.moveIf)
			assert.NoError(t, ptracetest.CompareTraces(tt.expectFrom, tt.from), "from not modified as expected")
			assert.NoError(t, ptracetest.CompareTraces(tt.expectTo, tt.to), "to not as expected")
		})
	}
}

func BenchmarkMoveResourcesIfTraces(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		from := ptraceutiltest.NewTraces("AB", "CD", "EF", "GH")
		to := ptrace.NewTraces()
		ptraceutil.MoveResourcesIf(from, to, func(ptrace.ResourceSpans) bool {
			return true
		})
		assert.Equal(b, 0, from.SpanCount())
		assert.Equal(b, 8, to.SpanCount())
	}
}
