// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

// The grouper under test routes each record on log.attributes["subject"].
const logsSubjectExpr = `log.attributes["subject"]`

// appendLog adds a record carrying a "name" attribute and, when subject is
// non-empty, the "subject" attribute the grouper routes on.
func appendLog(sl plog.ScopeLogs, name, subject string) {
	lr := sl.LogRecords().AppendEmpty()
	lr.Attributes().PutStr("name", name)
	if subject != "" {
		lr.Attributes().PutStr("subject", subject)
	}
}

// logGroupsBySubject indexes the grouper output by subject and asserts subjects
// are unique.
func logGroupsBySubject(t *testing.T, groups []Group[plog.Logs]) map[string]plog.Logs {
	t.Helper()
	m := make(map[string]plog.Logs, len(groups))
	for _, g := range groups {
		_, dup := m[g.Subject]
		require.Falsef(t, dup, "duplicate subject %q", g.Subject)
		m[g.Subject] = g.Data
	}
	return m
}

func TestLogsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("splits one scope across subjects", func(t *testing.T) {
		g, err := NewLogsGrouper(logsSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := plog.NewLogs()
		sl := in.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
		appendLog(sl, "r1", "a")
		appendLog(sl, "r2", "b")
		appendLog(sl, "r3", "a")

		groups, err := g.Group(t.Context(), in)
		require.NoError(t, err)
		got := logGroupsBySubject(t, groups)
		require.Len(t, got, 2)

		wantA := plog.NewLogs()
		slA := wantA.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
		appendLog(slA, "r1", "a")
		appendLog(slA, "r3", "a")
		assert.NoError(t, plogtest.CompareLogs(wantA, got["a"]))

		wantB := plog.NewLogs()
		appendLog(wantB.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty(), "r2", "b")
		assert.NoError(t, plogtest.CompareLogs(wantB, got["b"]))
	})

	t.Run("merges one subject across resources and preserves boundaries", func(t *testing.T) {
		g, err := NewLogsGrouper(logsSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := plog.NewLogs()
		rl1 := in.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("host", "h1")
		appendLog(rl1.ScopeLogs().AppendEmpty(), "r1", "a")
		rl2 := in.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("host", "h2")
		appendLog(rl2.ScopeLogs().AppendEmpty(), "r2", "a")

		groups, err := g.Group(t.Context(), in)
		require.NoError(t, err)
		got := logGroupsBySubject(t, groups)
		require.Len(t, got, 1)

		// Both source resources land in the single "a" group, each keeping its
		// own resource attributes rather than being flattened together.
		want := plog.NewLogs()
		w1 := want.ResourceLogs().AppendEmpty()
		w1.Resource().Attributes().PutStr("host", "h1")
		appendLog(w1.ScopeLogs().AppendEmpty(), "r1", "a")
		w2 := want.ResourceLogs().AppendEmpty()
		w2.Resource().Attributes().PutStr("host", "h2")
		appendLog(w2.ScopeLogs().AppendEmpty(), "r2", "a")
		assert.NoError(t, plogtest.CompareLogs(want, got["a"]))
	})

	t.Run("empty input yields no groups", func(t *testing.T) {
		g, err := NewLogsGrouper(logsSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		groups, err := g.Group(t.Context(), plog.NewLogs())
		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("a non-string subject is dropped with an error", func(t *testing.T) {
		g, err := NewLogsGrouper(logsSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := plog.NewLogs()
		sl := in.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
		appendLog(sl, "ok", "a")
		bad := sl.LogRecords().AppendEmpty()
		bad.Attributes().PutStr("name", "bad")
		bad.Attributes().PutInt("subject", 7) // not a string

		groups, err := g.Group(t.Context(), in)
		assert.Error(t, err)
		got := logGroupsBySubject(t, groups)
		require.Len(t, got, 1)

		want := plog.NewLogs()
		appendLog(want.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty(), "ok", "a")
		assert.NoError(t, plogtest.CompareLogs(want, got["a"]))
	})

	t.Run("a subject expression that fails to evaluate drops the record", func(t *testing.T) {
		// Len over an int errors at evaluation time.
		g, err := NewLogsGrouper(`Len(0)`, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := plog.NewLogs()
		appendLog(in.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty(), "r1", "a")

		groups, err := g.Group(t.Context(), in)
		assert.Error(t, err)
		assert.Empty(t, groups)
	})
}
