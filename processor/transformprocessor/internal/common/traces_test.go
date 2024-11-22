package common

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"testing"
)

func Benchmark_ConsumeTraces(b *testing.B) {

	statementsMatchingResourceCondition := ContextStatements{
		Context: SpanEvent,
		ResourceConditions: []string{
			`attributes["foo"] == "bar"`,
		},
		Statements: []string{
			`set(attributes["namespace"], "test") where name == "something"`,
		},
	}

	statementsMatchingResourceConditionInWhereClause := ContextStatements{
		Context: SpanEvent,
		Statements: []string{
			`set(attributes["namespace"], "test") where resource.attributes["foo"] == "bar" and name == "something"`,
		},
	}

	statementsNonMatchingResourceCondition := ContextStatements{
		Context: SpanEvent,
		ResourceConditions: []string{
			`attributes["foo"] == "baz"`,
		},
		Statements: []string{
			`set(attributes["namespace"], "test") where name == "something"`,
		},
	}

	statementsNonMatchingResourceConditionInWhereClause := ContextStatements{
		Context: SpanEvent,
		Statements: []string{
			`set(attributes["namespace"], "test") where resource.attributes["foo"] == "baz" and name == "something"`,
		},
	}

	nSpans := 1000
	nEvents := 1000

	b.Run(
		"matching condition in resource condition",
		func(b *testing.B) {
			benchmarkConsumeTraces(b, statementsMatchingResourceCondition, nSpans, nEvents)
		},
	)
	b.Run(
		"matching resource condition in where clause",
		func(b *testing.B) {
			benchmarkConsumeTraces(b, statementsMatchingResourceConditionInWhereClause, nSpans, nEvents)
		},
	)
	b.Run(
		"non matching resource condition",
		func(b *testing.B) {
			benchmarkConsumeTraces(b, statementsNonMatchingResourceCondition, nSpans, nEvents)
		},
	)
	b.Run(
		"non matching resource condition in where clause",
		func(b *testing.B) {
			benchmarkConsumeTraces(b, statementsNonMatchingResourceConditionInWhereClause, nSpans, nEvents)
		},
	)
}

func benchmarkConsumeTraces(b *testing.B, statements ContextStatements, nSpans, nEvents int) {
	pc, err := NewTraceParserCollection(
		componenttest.NewNopTelemetrySettings(),
		WithSpanParser(ottlfuncs.StandardFuncs[ottlspan.TransformContext]()),
		WithSpanEventParser(ottlfuncs.StandardFuncs[ottlspanevent.TransformContext]()),
	)
	require.NoError(b, err)

	contextStatements, err := pc.ParseContextStatements(statements)
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = contextStatements.ConsumeTraces(context.Background(), generateTraces(nSpans, nEvents))
	}
}

func generateTraces(nSpans, nEvents int) ptrace.Traces {
	traces := ptrace.NewTraces()

	rSpans := traces.ResourceSpans().AppendEmpty()
	rSpans.Resource().Attributes().PutStr("foo", "bar")
	sSpans := rSpans.ScopeSpans().AppendEmpty()

	for i := 0; i < nSpans; i++ {
		span := sSpans.Spans().AppendEmpty()
		span.SetName("something")
		span.Attributes().PutStr("foo", "bar")

		for j := 0; j < nEvents; j++ {
			ev := span.Events().AppendEmpty()
			ev.SetName("something")
			ev.Attributes().PutStr("foo", "bar")
		}
	}

	return traces
}
