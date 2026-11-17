// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func newTraceParserCollection(t *testing.T) *TraceParserCollection {
	t.Helper()
	pc, err := NewTraceParserCollection(
		componenttest.NewNopTelemetrySettings(),
		WithSpanParser(ottlfuncs.StandardFuncs[*ottlspan.TransformContext]()),
		WithSpanEventParser(ottlfuncs.StandardFuncs[*ottlspanevent.TransformContext]()),
		WithTraceErrorMode(ottl.PropagateError),
	)
	require.NoError(t, err)
	return pc
}

func TestTraceParserCollection_ConsumeTraces(t *testing.T) {
	tests := []struct {
		name    string
		cs      ContextStatements
		assert  func(*testing.T, any)
		wantKey string
	}{
		{
			name:    "span context",
			cs:      ContextStatements{Context: Span, Statements: []string{`set(attributes["test"], "pass")`}},
			wantKey: "test",
		},
		{
			name:    "span event context",
			cs:      ContextStatements{Context: SpanEvent, Statements: []string{`set(attributes["test"], "pass")`}},
			wantKey: "test",
		},
		{
			name:    "span context with condition",
			cs:      ContextStatements{Context: Span, Conditions: []string{`name == "span"`}, Statements: []string{`set(attributes["cond"], "pass")`}},
			wantKey: "cond",
		},
		{
			name:    "inferred context",
			cs:      ContextStatements{Statements: []string{`set(span.attributes["inferred"], "pass")`}},
			wantKey: "inferred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := newTraceParserCollection(t)
			consumer, err := pc.ParseContextStatements(tt.cs)
			require.NoError(t, err)

			td := newTestTraces()
			require.NoError(t, consumer.ConsumeTraces(t.Context(), td, nil))

			span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			var attrs pcommon.Map
			if tt.cs.Context == SpanEvent {
				attrs = span.Events().At(0).Attributes()
			} else {
				attrs = span.Attributes()
			}
			v, ok := attrs.Get(tt.wantKey)
			require.True(t, ok)
			assert.Equal(t, "pass", v.Str())
		})
	}
}

func TestTraceParserCollection_Context(t *testing.T) {
	pc := newTraceParserCollection(t)

	span, err := pc.ParseContextStatements(ContextStatements{Context: Span, Statements: []string{`set(attributes["x"], "y")`}})
	require.NoError(t, err)
	assert.Equal(t, Span, span.Context())

	spanEvent, err := pc.ParseContextStatements(ContextStatements{Context: SpanEvent, Statements: []string{`set(attributes["x"], "y")`}})
	require.NoError(t, err)
	assert.Equal(t, SpanEvent, spanEvent.Context())
}

func TestTraceParserCollection_ConsumeTraces_PropagatesError(t *testing.T) {
	pc := newTraceParserCollection(t)
	consumer, err := pc.ParseContextStatements(ContextStatements{Context: Span, Statements: []string{`set(attributes["test"], ParseJSON("1"))`}})
	require.NoError(t, err)

	err = consumer.ConsumeTraces(t.Context(), newTestTraces(), nil)
	require.Error(t, err)
}

func TestTraceParserCollection_ParseContextStatements_Error(t *testing.T) {
	pc := newTraceParserCollection(t)
	_, err := pc.ParseContextStatements(ContextStatements{Context: Span, Statements: []string{`not a valid statement`}})
	require.Error(t, err)
}
