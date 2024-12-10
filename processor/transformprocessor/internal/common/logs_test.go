package common

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"testing"
)

func Benchmark_ConsumeLogs(b *testing.B) {
	statementsMatchingResourceCondition := ContextStatements{
		Context: Log,
		ResourceConditions: []string{
			`attributes["foo"] == "bar"`,
		},
		Statements: []string{
			`set(attributes["namespace"], "test") where body == "something"`,
		},
	}

	statementsMatchingResourceConditionInWhereClause := ContextStatements{
		Context: Log,
		Statements: []string{
			`set(attributes["namespace"], "test") where resource.attributes["foo"] == "bar" and body == "something"`,
		},
	}

	statementsNonMatchingResourceCondition := ContextStatements{
		Context: Log,
		ResourceConditions: []string{
			`attributes["foo"] == "baz"`,
		},
		Statements: []string{
			`set(attributes["namespace"], "test") where body == "something"`,
		},
	}

	statementsNonMatchingResourceConditionInWhereClause := ContextStatements{
		Context: Log,
		Statements: []string{
			`set(attributes["namespace"], "test") where resource.attributes["foo"] == "baz" and body == "something"`,
		},
	}

	nLogRecords := 1000

	b.Run(
		"matching condition in resource condition",
		func(b *testing.B) {
			benchmarkConsumeLogs(b, statementsMatchingResourceCondition, nLogRecords)
		},
	)
	b.Run(
		"matching resource condition in where clause",
		func(b *testing.B) {
			benchmarkConsumeLogs(b, statementsMatchingResourceConditionInWhereClause, nLogRecords)
		},
	)
	b.Run(
		"non matching resource condition",
		func(b *testing.B) {
			benchmarkConsumeLogs(b, statementsNonMatchingResourceCondition, nLogRecords)
		},
	)
	b.Run(
		"non matching resource condition in where clause",
		func(b *testing.B) {
			benchmarkConsumeLogs(b, statementsNonMatchingResourceConditionInWhereClause, nLogRecords)
		},
	)
}

func benchmarkConsumeLogs(b *testing.B, statements ContextStatements, nLogRecords int) {
	pc, err := NewLogParserCollection(
		componenttest.NewNopTelemetrySettings(),
		WithLogParser(ottlfuncs.StandardFuncs[ottllog.TransformContext]()),
	)
	require.NoError(b, err)

	contextStatements, err := pc.ParseContextStatements(statements)
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = contextStatements.ConsumeLogs(context.Background(), generateLogs(nLogRecords))
	}
	require.NoError(b, err)
}

func generateLogs(n int) plog.Logs {
	logs := plog.NewLogs()

	rLogs := logs.ResourceLogs().AppendEmpty()
	rLogs.Resource().Attributes().PutStr("foo", "bar")
	sLogs := rLogs.ScopeLogs().AppendEmpty()

	for i := 0; i < n; i++ {
		logRecord := sLogs.LogRecords().AppendEmpty()
		logRecord.Body().SetStr("something")
		logRecord.Attributes().PutStr("foo", "bar")
	}
	return logs
}
