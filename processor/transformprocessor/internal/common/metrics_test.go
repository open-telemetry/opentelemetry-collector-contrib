package common

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"testing"
)

func Benchmark_ConsumeMetrics(b *testing.B) {
	statementsMatchingResourceCondition := ContextStatements{
		Context: DataPoint,
		ResourceConditions: []string{
			`attributes["foo"] == "bar"`,
		},
		Statements: []string{
			`set(attributes["namespace"], "test") where value_int > 1`,
		},
	}

	statementsMatchingResourceConditionInWhereClause := ContextStatements{
		Context: DataPoint,
		Statements: []string{
			`set(attributes["namespace"], "test") where resource.attributes["foo"] == "bar" and value_int > 1`,
		},
	}

	statementsNonMatchingResourceCondition := ContextStatements{
		Context: DataPoint,
		ResourceConditions: []string{
			`attributes["foo"] == "baz"`,
		},
		Statements: []string{
			`set(attributes["namespace"], "test") where value_int > 1`,
		},
	}

	statementsNonMatchingResourceConditionInWhereClause := ContextStatements{
		Context: DataPoint,
		Statements: []string{
			`set(attributes["namespace"], "test") where resource.attributes["foo"] == "baz" and value_int > 1`,
		},
	}

	nMetrics := 100
	nDataPoints := 10000

	b.Run(
		"matching condition in resource condition",
		func(b *testing.B) {
			benchmarkConsumeMetrics(b, statementsMatchingResourceCondition, nMetrics, nDataPoints)
		},
	)
	b.Run(
		"matching resource condition in where clause",
		func(b *testing.B) {
			benchmarkConsumeMetrics(b, statementsMatchingResourceConditionInWhereClause, nMetrics, nDataPoints)
		},
	)
	b.Run(
		"non matching resource condition",
		func(b *testing.B) {
			benchmarkConsumeMetrics(b, statementsNonMatchingResourceCondition, nMetrics, nDataPoints)
		},
	)
	b.Run(
		"non matching resource condition in where clause",
		func(b *testing.B) {
			benchmarkConsumeMetrics(b, statementsNonMatchingResourceConditionInWhereClause, nMetrics, nDataPoints)
		},
	)
}

func benchmarkConsumeMetrics(b *testing.B, statements ContextStatements, nMetrics, nDatapoints int) {
	pc, err := NewMetricParserCollection(
		componenttest.NewNopTelemetrySettings(),
		WithMetricParser(ottlfuncs.StandardFuncs[ottlmetric.TransformContext]()),
		WithDataPointParser(ottlfuncs.StandardFuncs[ottldatapoint.TransformContext]()),
	)
	require.NoError(b, err)

	contextStatements, err := pc.ParseContextStatements(statements)
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = contextStatements.ConsumeMetrics(context.Background(), generateMetrics(nMetrics, nDatapoints))
	}
}

func generateMetrics(nrMetrics, nrDatapoints int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()

	rMetrics := metrics.ResourceMetrics().AppendEmpty()
	rMetrics.Resource().Attributes().PutStr("foo", "bar")
	sMetrics := rMetrics.ScopeMetrics().AppendEmpty()

	for i := 0; i < nrMetrics; i++ {
		m := sMetrics.Metrics().AppendEmpty()
		gauge := m.SetEmptyGauge()
		for j := 0; j < nrDatapoints; j++ {
			dp := gauge.DataPoints().AppendEmpty()
			dp.Attributes().PutStr("foo", "bar")
			dp.SetIntValue(5)
		}
	}

	return metrics
}
