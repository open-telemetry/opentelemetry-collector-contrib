// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

func TestSetAttrOnScopeSpans_Empty(_ *testing.T) {
	traces := ptrace.NewTraces()

	SetAttrOnScopeSpans(traces, "test.attr", "value")
}

func TestSetAttrOnScopeSpans_Many(t *testing.T) {
	assertAttrExists := func(t *testing.T, attrs pcommon.Map, key, value string) {
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

	SetAttrOnScopeSpans(traces, "test.attr", "value")

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
	for b.Loop() {
		b.StopTimer()
		traces := ptrace.NewTraces()

		for range 5 {
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

		b.StartTimer()
		SetAttrOnScopeSpans(traces, "test.attr", "value")
	}
}

func TestWriteEffectiveThreshold(t *testing.T) {
	// th:8 encodes a 50% sampling threshold.
	th8, err := sampling.TValueToThreshold("8")
	require.NoError(t, err)
	// th:c encodes a 75% rejection threshold (= 25% sampling),
	// strictly more aggressive than th:8.
	thc, err := sampling.TValueToThreshold("c")
	require.NoError(t, err)

	ctx := context.Background()
	counter, err := noop.NewMeterProvider().Meter("test").Int64Counter("test")
	require.NoError(t, err)

	t.Run("writes th on bare span", func(t *testing.T) {
		traces := makeTracesWithTracestates("")
		WriteEffectiveThreshold(ctx, traces, th8, counter)
		assert.Equal(t, "ot=th:8", firstSpanTracestate(traces))
	})

	t.Run("rv on incoming span is preserved", func(t *testing.T) {
		traces := makeTracesWithTracestates("ot=rv:abcdef01234567")
		WriteEffectiveThreshold(ctx, traces, th8, counter)
		got := firstSpanTracestate(traces)
		assert.Contains(t, got, "th:8")
		assert.Contains(t, got, "rv:abcdef01234567")
	})

	t.Run("stricter existing th preserved", func(t *testing.T) {
		// Existing th:c is stricter than effective th:8. Spec
		// forbids lowering, so existing must remain.
		traces := makeTracesWithTracestates("ot=th:c")
		WriteEffectiveThreshold(ctx, traces, th8, counter)
		assert.Equal(t, "ot=th:c", firstSpanTracestate(traces))
	})

	t.Run("less-strict existing th raised", func(t *testing.T) {
		// Existing th:8 is less strict than effective th:c.
		// Update is allowed (raises threshold).
		traces := makeTracesWithTracestates("ot=th:8")
		WriteEffectiveThreshold(ctx, traces, thc, counter)
		assert.Equal(t, "ot=th:c", firstSpanTracestate(traces))
	})

	t.Run("invalid tracestate skipped", func(t *testing.T) {
		invalid := "ot=not_a_valid_key$"
		traces := makeTracesWithTracestates(invalid)
		WriteEffectiveThreshold(ctx, traces, th8, counter)
		assert.Equal(t, invalid, firstSpanTracestate(traces),
			"unparseable tracestate must be left untouched")
	})

	t.Run("invalid tracestate increments counter", func(t *testing.T) {
		reader := sdkmetric.NewManualReader()
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		realCounter, err := mp.Meter("test").Int64Counter("unparseable")
		require.NoError(t, err)

		// Three spans: two unparseable, one bare.
		traces := makeTracesWithTracestates("ot=bad$", "ot=also_bad$", "")
		WriteEffectiveThreshold(ctx, traces, th8, realCounter)

		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &rm))
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
		sum, ok := rm.ScopeMetrics[0].Metrics[0].Data.(metricdata.Sum[int64])
		require.True(t, ok)
		require.Len(t, sum.DataPoints, 1)
		assert.Equal(t, int64(2), sum.DataPoints[0].Value)
	})
}

func makeTracesWithTracestates(tracestates ...string) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	for _, ts := range tracestates {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.TraceState().FromRaw(ts)
	}
	return traces
}

func firstSpanTracestate(traces ptrace.Traces) string {
	return traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceState().AsRaw()
}
