// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
)

// ---- helper marshalers ----

type tracesMarshaler struct{ m ptrace.ProtoMarshaler }

func (t tracesMarshaler) MarshalTraces(td ptrace.Traces) ([]marshaler.Message, error) {
	b, err := t.m.MarshalTraces(td)
	if err != nil {
		return nil, err
	}
	return []marshaler.Message{{Value: b}}, nil
}

type logsMarshaler struct{ m plog.ProtoMarshaler }

func (l logsMarshaler) MarshalLogs(ld plog.Logs) ([]marshaler.Message, error) {
	b, err := l.m.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}
	return []marshaler.Message{{Value: b}}, nil
}

type metricsMarshaler struct{ m pmetric.ProtoMarshaler }

func (m metricsMarshaler) MarshalMetrics(md pmetric.Metrics) ([]marshaler.Message, error) {
	b, err := m.m.MarshalMetrics(md)
	if err != nil {
		return nil, err
	}
	return []marshaler.Message{{Value: b}}, nil
}

type profilesMarshaler struct{ m pprofile.ProtoMarshaler }

func (p profilesMarshaler) MarshalProfiles(pd pprofile.Profiles) ([]marshaler.Message, error) {
	b, err := p.m.MarshalProfiles(pd)
	if err != nil {
		return nil, err
	}
	return []marshaler.Message{{Value: b}}, nil
}

// ---- traces tests ----

func TestSplitTraces_FitsInLimit(t *testing.T) {
	td := makeTraces(4, 1, 1)
	m := tracesMarshaler{}

	msgs, _ := m.MarshalTraces(td)
	limit := len(msgs[0].Value) + 100 // well above actual size

	chunks, err := SplitTraces(td, m, limit)
	require.NoError(t, err)
	require.Len(t, chunks, 1, "no split expected when payload fits")
	assert.Equal(t, td.ResourceSpans().Len(), chunks[0].ResourceSpans().Len())
}

func TestSplitTraces_SplitsAtResourceLevel(t *testing.T) {
	// 8 resource spans; marshal the first one to find its per-resource size.
	td := makeTraces(8, 1, 1)
	m := tracesMarshaler{}

	// Pick a limit that fits 1 resource span but not all 8.
	single := makeTraces(1, 1, 1)
	singleMsgs, _ := m.MarshalTraces(single)
	limit := len(singleMsgs[0].Value) + 20

	chunks, err := SplitTraces(td, m, limit)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "expected multiple chunks")

	// Verify every chunk fits in the limit.
	for _, c := range chunks {
		msgs, _ := m.MarshalTraces(c)
		assert.LessOrEqual(t, len(msgs[0].Value), limit)
	}

	// Verify all spans are preserved.
	total := 0
	for _, c := range chunks {
		for _, rs := range c.ResourceSpans().All() {
			for _, ss := range rs.ScopeSpans().All() {
				total += ss.Spans().Len()
			}
		}
	}
	assert.Equal(t, 8, total)
}

func TestSplitTraces_SplitsAtScopeLevel(t *testing.T) {
	// 1 resource span with 8 scope spans, each with 1 span.
	td := makeTraces(1, 8, 1)
	m := tracesMarshaler{}

	single := makeTraces(1, 1, 1)
	singleMsgs, _ := m.MarshalTraces(single)
	limit := len(singleMsgs[0].Value) + 20

	chunks, err := SplitTraces(td, m, limit)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	for _, c := range chunks {
		msgs, _ := m.MarshalTraces(c)
		assert.LessOrEqual(t, len(msgs[0].Value), limit)
	}
}

func TestSplitTraces_SplitsAtSpanLevel(t *testing.T) {
	// 1 resource span, 1 scope span, 8 spans.
	td := makeTraces(1, 1, 8)
	m := tracesMarshaler{}

	single := makeTraces(1, 1, 1)
	singleMsgs, _ := m.MarshalTraces(single)
	limit := len(singleMsgs[0].Value) + 20

	chunks, err := SplitTraces(td, m, limit)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	for _, c := range chunks {
		msgs, _ := m.MarshalTraces(c)
		assert.LessOrEqual(t, len(msgs[0].Value), limit)
	}
}

func TestSplitTraces_UnsplittableSingleSpan(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	sp := ss.Spans().AppendEmpty()
	sp.Attributes().PutStr("large", strings.Repeat("x", 500))

	m := tracesMarshaler{}
	msgs, _ := m.MarshalTraces(td)
	// Set limit well below the actual size so it can never fit.
	limit := len(msgs[0].Value) / 2

	chunks, err := SplitTraces(td, m, limit)
	require.NoError(t, err)
	// Cannot split further — must be returned as-is.
	require.Len(t, chunks, 1)
}

// ---- logs tests ----

func TestSplitLogs_FitsInLimit(t *testing.T) {
	ld := makeLogs(4, 1, 1)
	m := logsMarshaler{}

	msgs, _ := m.MarshalLogs(ld)
	limit := len(msgs[0].Value) + 100

	chunks, err := SplitLogs(ld, m, limit)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
}

func TestSplitLogs_SplitsAtResourceLevel(t *testing.T) {
	ld := makeLogs(8, 1, 1)
	m := logsMarshaler{}

	single := makeLogs(1, 1, 1)
	singleMsgs, _ := m.MarshalLogs(single)
	limit := len(singleMsgs[0].Value) + 20

	chunks, err := SplitLogs(ld, m, limit)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	total := 0
	for _, c := range chunks {
		msgs, _ := m.MarshalLogs(c)
		assert.LessOrEqual(t, len(msgs[0].Value), limit)
		for _, rl := range c.ResourceLogs().All() {
			for _, sl := range rl.ScopeLogs().All() {
				total += sl.LogRecords().Len()
			}
		}
	}
	assert.Equal(t, 8, total)
}

func TestSplitLogs_UnsplittableSingleRecord(t *testing.T) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr(strings.Repeat("x", 500))

	m := logsMarshaler{}
	msgs, _ := m.MarshalLogs(ld)
	limit := len(msgs[0].Value) / 2

	chunks, err := SplitLogs(ld, m, limit)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
}

// ---- metrics tests ----

func TestSplitMetrics_FitsInLimit(t *testing.T) {
	md := makeMetrics(4, 1, 1)
	m := metricsMarshaler{}

	msgs, _ := m.MarshalMetrics(md)
	limit := len(msgs[0].Value) + 100

	chunks, err := SplitMetrics(md, m, limit)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
}

func TestSplitMetrics_SplitsAtResourceLevel(t *testing.T) {
	md := makeMetrics(8, 1, 1)
	m := metricsMarshaler{}

	single := makeMetrics(1, 1, 1)
	singleMsgs, _ := m.MarshalMetrics(single)
	limit := len(singleMsgs[0].Value) + 20

	chunks, err := SplitMetrics(md, m, limit)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	for _, c := range chunks {
		msgs, _ := m.MarshalMetrics(c)
		assert.LessOrEqual(t, len(msgs[0].Value), limit)
	}
}

// ---- profiles tests ----

func TestSplitProfiles_FitsInLimit(t *testing.T) {
	pd := makeProfiles(4, 1, 1)
	m := profilesMarshaler{}

	msgs, _ := m.MarshalProfiles(pd)
	limit := len(msgs[0].Value) + 100

	chunks, err := SplitProfiles(pd, m, limit)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
}

func TestSplitProfiles_SplitsAtResourceLevel(t *testing.T) {
	pd := makeProfiles(8, 1, 1)
	m := profilesMarshaler{}

	single := makeProfiles(1, 1, 1)
	singleMsgs, _ := m.MarshalProfiles(single)
	limit := len(singleMsgs[0].Value) + 20

	chunks, err := SplitProfiles(pd, m, limit)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	for _, c := range chunks {
		msgs, _ := m.MarshalProfiles(c)
		assert.LessOrEqual(t, len(msgs[0].Value), limit)
	}
}

// ---- pdata builders ----

// makeTraces creates a ptrace.Traces with nRS ResourceSpans, each containing
// nSS ScopeSpans, each containing nSpans Spans.
func makeTraces(nRS, nSS, nSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	for range nRS {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("res", "val")
		for range nSS {
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scope")
			for range nSpans {
				sp := ss.Spans().AppendEmpty()
				sp.SetName("span")
				sp.Attributes().PutStr("k", "v")
			}
		}
	}
	return td
}

// makeLogs creates a plog.Logs with nRL ResourceLogs, nSL ScopeLogs, nLR LogRecords.
func makeLogs(nRL, nSL, nLR int) plog.Logs {
	ld := plog.NewLogs()
	for range nRL {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("res", "val")
		for range nSL {
			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName("scope")
			for range nLR {
				lr := sl.LogRecords().AppendEmpty()
				lr.Body().SetStr("log body")
				lr.Attributes().PutStr("k", "v")
			}
		}
	}
	return ld
}

// makeMetrics creates a pmetric.Metrics with nRM ResourceMetrics, nSM ScopeMetrics,
// nM Metrics (each a gauge with one data point).
func makeMetrics(nRM, nSM, nM int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for range nRM {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("res", "val")
		for range nSM {
			sm := rm.ScopeMetrics().AppendEmpty()
			sm.Scope().SetName("scope")
			for range nM {
				m := sm.Metrics().AppendEmpty()
				m.SetName("metric")
				m.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)
			}
		}
	}
	return md
}

// makeProfiles creates a pprofile.Profiles with nRP ResourceProfiles, nSP ScopeProfiles,
// nP Profiles.
func makeProfiles(nRP, nSP, nP int) pprofile.Profiles {
	pd := pprofile.NewProfiles()
	for range nRP {
		rp := pd.ResourceProfiles().AppendEmpty()
		rp.Resource().Attributes().PutStr("res", "val")
		for range nSP {
			sp := rp.ScopeProfiles().AppendEmpty()
			sp.Scope().SetName("scope")
			for range nP {
				sp.Profiles().AppendEmpty()
			}
		}
	}
	return pd
}
