// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

// The grouper under test routes each span on span.attributes["subject"].
const tracesSubjectExpr = `span.attributes["subject"]`

// appendSpan adds a span named name and, when subject is non-empty, the
// "subject" attribute the grouper routes on.
func appendSpan(ss ptrace.ScopeSpans, name, subject string) {
	s := ss.Spans().AppendEmpty()
	s.SetName(name)
	if subject != "" {
		s.Attributes().PutStr("subject", subject)
	}
}

func spanGroupsBySubject(t *testing.T, groups []Group[ptrace.Traces]) map[string]ptrace.Traces {
	t.Helper()
	m := make(map[string]ptrace.Traces, len(groups))
	for _, g := range groups {
		_, dup := m[g.Subject]
		require.Falsef(t, dup, "duplicate subject %q", g.Subject)
		m[g.Subject] = g.Data
	}
	return m
}

func TestTracesGrouper(t *testing.T) {
	t.Parallel()

	t.Run("splits one scope across subjects", func(t *testing.T) {
		g, err := NewTracesGrouper(tracesSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := ptrace.NewTraces()
		ss := in.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		appendSpan(ss, "s1", "a")
		appendSpan(ss, "s2", "b")
		appendSpan(ss, "s3", "a")

		groups, err := g.Group(t.Context(), in)
		require.NoError(t, err)
		got := spanGroupsBySubject(t, groups)
		require.Len(t, got, 2)

		wantA := ptrace.NewTraces()
		ssA := wantA.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		appendSpan(ssA, "s1", "a")
		appendSpan(ssA, "s3", "a")
		assert.NoError(t, ptracetest.CompareTraces(wantA, got["a"]))

		wantB := ptrace.NewTraces()
		appendSpan(wantB.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty(), "s2", "b")
		assert.NoError(t, ptracetest.CompareTraces(wantB, got["b"]))
	})

	t.Run("merges one subject across resources and preserves boundaries", func(t *testing.T) {
		g, err := NewTracesGrouper(tracesSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := ptrace.NewTraces()
		rs1 := in.ResourceSpans().AppendEmpty()
		rs1.Resource().Attributes().PutStr("host", "h1")
		appendSpan(rs1.ScopeSpans().AppendEmpty(), "s1", "a")
		rs2 := in.ResourceSpans().AppendEmpty()
		rs2.Resource().Attributes().PutStr("host", "h2")
		appendSpan(rs2.ScopeSpans().AppendEmpty(), "s2", "a")

		groups, err := g.Group(t.Context(), in)
		require.NoError(t, err)
		got := spanGroupsBySubject(t, groups)
		require.Len(t, got, 1)

		want := ptrace.NewTraces()
		w1 := want.ResourceSpans().AppendEmpty()
		w1.Resource().Attributes().PutStr("host", "h1")
		appendSpan(w1.ScopeSpans().AppendEmpty(), "s1", "a")
		w2 := want.ResourceSpans().AppendEmpty()
		w2.Resource().Attributes().PutStr("host", "h2")
		appendSpan(w2.ScopeSpans().AppendEmpty(), "s2", "a")
		assert.NoError(t, ptracetest.CompareTraces(want, got["a"]))
	})

	t.Run("empty input yields no groups", func(t *testing.T) {
		g, err := NewTracesGrouper(tracesSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		groups, err := g.Group(t.Context(), ptrace.NewTraces())
		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("a non-string subject is dropped with an error", func(t *testing.T) {
		g, err := NewTracesGrouper(tracesSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := ptrace.NewTraces()
		ss := in.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		appendSpan(ss, "ok", "a")
		bad := ss.Spans().AppendEmpty()
		bad.SetName("bad")
		bad.Attributes().PutInt("subject", 7) // not a string

		groups, err := g.Group(t.Context(), in)
		assert.Error(t, err)
		got := spanGroupsBySubject(t, groups)
		require.Len(t, got, 1)

		want := ptrace.NewTraces()
		appendSpan(want.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty(), "ok", "a")
		assert.NoError(t, ptracetest.CompareTraces(want, got["a"]))
	})

	t.Run("a subject expression that fails to evaluate drops the span", func(t *testing.T) {
		g, err := NewTracesGrouper(`Len(0)`, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := ptrace.NewTraces()
		appendSpan(in.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty(), "s1", "a")

		groups, err := g.Group(t.Context(), in)
		assert.Error(t, err)
		assert.Empty(t, groups)
	})
}
