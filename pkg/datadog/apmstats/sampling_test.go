// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apmstats // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/apmstats"

import (
	"testing"

	traceutilotel "github.com/DataDog/datadog-agent/pkg/trace/otel/traceutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func newSpanWithTraceState(spanID [8]byte, traceState string) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.SpanID(spanID))
	span.TraceState().FromRaw(traceState)
	return span
}

func TestSamplingProbFromSpan(t *testing.T) {
	tests := []struct {
		name       string
		traceState string
		wantProb   float64
		wantOK     bool
	}{
		{
			name:       "50% sampling (th:8)",
			traceState: "ot=th:8",
			wantProb:   0.5,
			wantOK:     true,
		},
		{
			name:       "100% sampling (th:0)",
			traceState: "ot=th:0",
			wantProb:   1.0,
			wantOK:     true,
		},
		{
			name:       "p-encoding 50% (p:1)",
			traceState: "ot=p:1;r:1",
			wantProb:   0.5,
			wantOK:     true,
		},
		{
			name:       "p-encoding 6.25% (p:4)",
			traceState: "ot=p:4;r:4",
			wantProb:   0.0625,
			wantOK:     true,
		},
		{
			name:       "p-encoding 100% (p:0)",
			traceState: "ot=p:0",
			wantProb:   1.0,
			wantOK:     true,
		},
		{
			name:       "th absent rv present",
			traceState: "ot=rv:abcdefabcdefab",
			wantProb:   0,
			wantOK:     false,
		},
		{
			name:       "no tracestate",
			traceState: "",
			wantProb:   0,
			wantOK:     false,
		},
		{
			name:       "non-ot vendor field only",
			traceState: "zz=vendorcontent",
			wantProb:   0,
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := newSpanWithTraceState([8]byte{1, 2, 3, 4, 5, 6, 7, 8}, tt.traceState)
			prob, ok := samplingProbFromSpan(span, nil)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.InDelta(t, tt.wantProb, prob, 1e-9)
			}
		})
	}
}

func TestSamplingProbFromSpan_MalformedTracestate(t *testing.T) {
	// Malformed tracestate — must not panic and must return (0, false).
	// The value "ot=th:zz" is invalid hex for the th field.
	// Note: "ot=th:zz" parses the ot field but "zz" is not valid hex, so
	// NewW3CTraceState may return an error or silently skip; either way ok=false.
	span := newSpanWithTraceState([8]byte{1}, "ot=th:zz")
	assert.NotPanics(t, func() {
		prob, ok := samplingProbFromSpan(span, nil)
		assert.False(t, ok)
		assert.Equal(t, float64(0), prob)
	})
}

func TestRawTracestatesBySpanID(t *testing.T) {
	spanID1 := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2 := [8]byte{9, 10, 11, 12, 13, 14, 15, 16}

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	// Span 1: has a tracestate
	s1 := ss.Spans().AppendEmpty()
	s1.SetSpanID(pcommon.SpanID(spanID1))
	s1.TraceState().FromRaw("ot=th:8")

	// Span 2: no tracestate — should not appear in the map
	s2 := ss.Spans().AppendEmpty()
	s2.SetSpanID(pcommon.SpanID(spanID2))

	result := rawTracestatesBySpanID(td)

	require.Len(t, result, 1)

	// The key must match the DD uint64 span ID used by OtelSpanToDDSpanMinimal.
	expectedKey := traceutilotel.OTelSpanIDToUint64(pcommon.SpanID(spanID1))
	raw, ok := result[expectedKey]
	require.True(t, ok)
	assert.Equal(t, "ot=th:8", raw)

	// Span 2 must not appear.
	_, ok = result[traceutilotel.OTelSpanIDToUint64(pcommon.SpanID(spanID2))]
	assert.False(t, ok)
}
