// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/transformconfig"
)

// errCond parses as a valid boolean condition for every context but errors at
// evaluation time (ParseJSON of a non-object), so it drives the Eval error
// branch of each consumer. errStmt errors at execution time the same way.
const (
	errCond = `ParseJSON("1") == "x"`
	errStmt = `set(attributes["test"], ParseJSON("1"))`
)

func TestConsume_EvalError(t *testing.T) {
	t.Run("traces", func(t *testing.T) {
		for _, ctx := range []transformconfig.ContextID{transformconfig.Resource, transformconfig.Scope, transformconfig.Span, transformconfig.SpanEvent} {
			t.Run(string(ctx), func(t *testing.T) {
				consumer, err := newTraceParserCollection(t).
					ParseContextStatements(transformconfig.ContextStatements{Context: ctx, Conditions: []string{errCond}, Statements: []string{`set(attributes["x"], "y")`}})
				require.NoError(t, err)
				require.Error(t, consumer.ConsumeTraces(t.Context(), newTestTraces(), nil))
			})
		}
	})

	t.Run("metrics", func(t *testing.T) {
		for _, ctx := range []transformconfig.ContextID{transformconfig.Resource, transformconfig.Scope, transformconfig.Metric} {
			t.Run(string(ctx), func(t *testing.T) {
				stmt := `set(attributes["x"], "y")`
				if ctx == transformconfig.Metric {
					stmt = `set(description, "y")`
				}
				consumer, err := newMetricParserCollection(t).
					ParseContextStatements(transformconfig.ContextStatements{Context: ctx, Conditions: []string{errCond}, Statements: []string{stmt}})
				require.NoError(t, err)
				require.Error(t, consumer.ConsumeMetrics(t.Context(), newTestMetrics(), nil))
			})
		}
	})

	t.Run("logs", func(t *testing.T) {
		for _, ctx := range []transformconfig.ContextID{transformconfig.Resource, transformconfig.Scope, transformconfig.Log} {
			t.Run(string(ctx), func(t *testing.T) {
				consumer, err := newLogParserCollection(t).
					ParseContextStatements(transformconfig.ContextStatements{Context: ctx, Conditions: []string{errCond}, Statements: []string{`set(attributes["x"], "y")`}})
				require.NoError(t, err)
				require.Error(t, consumer.ConsumeLogs(t.Context(), newTestLogs(), nil))
			})
		}
	})

	t.Run("profiles", func(t *testing.T) {
		for _, ctx := range []transformconfig.ContextID{transformconfig.Resource, transformconfig.Scope, transformconfig.Profile} {
			t.Run(string(ctx), func(t *testing.T) {
				stmt := `set(attributes["x"], "y")`
				if ctx == transformconfig.Profile {
					stmt = `set(original_payload_format, "y")`
				}
				consumer, err := newProfileParserCollection(t).
					ParseContextStatements(transformconfig.ContextStatements{Context: ctx, Conditions: []string{errCond}, Statements: []string{stmt}})
				require.NoError(t, err)
				require.Error(t, consumer.ConsumeProfiles(t.Context(), newTestProfiles(), nil))
			})
		}
	})
}

func TestConsumeMetrics_DataPointErrors(t *testing.T) {
	types := []struct {
		name string
		md   func() pmetric.Metrics
	}{
		{"sum", sumMetrics},
		{"gauge", gaugeMetrics},
		{"histogram", histogramMetrics},
		{"exp_histogram", expHistogramMetrics},
		{"summary", summaryMetrics},
	}
	for _, tc := range types {
		t.Run(tc.name+"/execute", func(t *testing.T) {
			consumer, err := newMetricParserCollection(t).
				ParseContextStatements(transformconfig.ContextStatements{Context: transformconfig.DataPoint, Statements: []string{errStmt}})
			require.NoError(t, err)
			require.Error(t, consumer.ConsumeMetrics(t.Context(), tc.md(), nil))
		})
		t.Run(tc.name+"/eval", func(t *testing.T) {
			consumer, err := newMetricParserCollection(t).
				ParseContextStatements(transformconfig.ContextStatements{Context: transformconfig.DataPoint, Conditions: []string{errCond}, Statements: []string{`set(attributes["x"], "y")`}})
			require.NoError(t, err)
			require.Error(t, consumer.ConsumeMetrics(t.Context(), tc.md(), nil))
		})
	}
}

func TestConsumeMetrics_ExemplarErrors(t *testing.T) {
	types := []struct {
		name string
		md   func() pmetric.Metrics
	}{
		{"gauge", gaugeMetrics},
		{"histogram", histogramMetrics},
		{"exp_histogram", expHistogramMetrics},
	}
	for _, tc := range types {
		t.Run(tc.name+"/execute", func(t *testing.T) {
			consumer, err := newMetricParserCollection(t).
				ParseContextStatements(transformconfig.ContextStatements{Context: transformconfig.Exemplar, Statements: []string{`set(filtered_attributes["test"], ParseJSON("1"))`}})
			require.NoError(t, err)
			require.Error(t, consumer.ConsumeMetrics(t.Context(), tc.md(), nil))
		})
		t.Run(tc.name+"/eval", func(t *testing.T) {
			consumer, err := newMetricParserCollection(t).
				ParseContextStatements(transformconfig.ContextStatements{Context: transformconfig.Exemplar, Conditions: []string{errCond}, Statements: []string{`set(filtered_attributes["x"], "y")`}})
			require.NoError(t, err)
			require.Error(t, consumer.ConsumeMetrics(t.Context(), tc.md(), nil))
		})
	}
}

func TestParse_ErrorModeOverride(t *testing.T) {
	traces := newTraceParserCollection(t)
	metrics := newMetricParserCollection(t)
	logs := newLogParserCollection(t)
	profiles := newProfileParserCollection(t)

	cases := []struct {
		name  string
		parse func(transformconfig.ContextStatements) error
		cs    transformconfig.ContextStatements
	}{
		{"span", func(cs transformconfig.ContextStatements) error {
			_, err := traces.ParseContextStatements(cs)
			return err
		}, transformconfig.ContextStatements{Context: transformconfig.Span, Statements: []string{`set(attributes["x"], "y")`}}},
		{"spanevent", func(cs transformconfig.ContextStatements) error {
			_, err := traces.ParseContextStatements(cs)
			return err
		}, transformconfig.ContextStatements{Context: transformconfig.SpanEvent, Statements: []string{`set(attributes["x"], "y")`}}},
		{"resource", func(cs transformconfig.ContextStatements) error {
			_, err := traces.ParseContextStatements(cs)
			return err
		}, transformconfig.ContextStatements{Context: transformconfig.Resource, Statements: []string{`set(attributes["x"], "y")`}}},
		{"scope", func(cs transformconfig.ContextStatements) error {
			_, err := traces.ParseContextStatements(cs)
			return err
		}, transformconfig.ContextStatements{Context: transformconfig.Scope, Statements: []string{`set(attributes["x"], "y")`}}},
		{"metric", func(cs transformconfig.ContextStatements) error {
			_, err := metrics.ParseContextStatements(cs)
			return err
		}, transformconfig.ContextStatements{Context: transformconfig.Metric, Statements: []string{`set(description, "y")`}}},
		{"datapoint", func(cs transformconfig.ContextStatements) error {
			_, err := metrics.ParseContextStatements(cs)
			return err
		}, transformconfig.ContextStatements{Context: transformconfig.DataPoint, Statements: []string{`set(attributes["x"], "y")`}}},
		{"exemplar", func(cs transformconfig.ContextStatements) error {
			_, err := metrics.ParseContextStatements(cs)
			return err
		}, transformconfig.ContextStatements{Context: transformconfig.Exemplar, Statements: []string{`set(filtered_attributes["x"], "y")`}}},
		{"log", func(cs transformconfig.ContextStatements) error {
			_, err := logs.ParseContextStatements(cs)
			return err
		}, transformconfig.ContextStatements{Context: transformconfig.Log, Statements: []string{`set(attributes["x"], "y")`}}},
		{"profile", func(cs transformconfig.ContextStatements) error {
			_, err := profiles.ParseContextStatements(cs)
			return err
		}, transformconfig.ContextStatements{Context: transformconfig.Profile, Statements: []string{`set(original_payload_format, "y")`}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := tc.cs
			cs.ErrorMode = ottl.IgnoreError
			require.NoError(t, tc.parse(cs))
		})
	}
}

func TestParse_InvalidCondition(t *testing.T) {
	traces := newTraceParserCollection(t)
	metrics := newMetricParserCollection(t)
	logs := newLogParserCollection(t)
	profiles := newProfileParserCollection(t)

	badCond := []string{`NotARealFunction() == true`}
	cases := []struct {
		name  string
		parse func(transformconfig.ContextStatements) error
		ctx   transformconfig.ContextID
		stmt  string
	}{
		{"span", func(cs transformconfig.ContextStatements) error {
			_, err := traces.ParseContextStatements(cs)
			return err
		}, transformconfig.Span, `set(attributes["x"], "y")`},
		{"spanevent", func(cs transformconfig.ContextStatements) error {
			_, err := traces.ParseContextStatements(cs)
			return err
		}, transformconfig.SpanEvent, `set(attributes["x"], "y")`},
		{"resource", func(cs transformconfig.ContextStatements) error {
			_, err := traces.ParseContextStatements(cs)
			return err
		}, transformconfig.Resource, `set(attributes["x"], "y")`},
		{"scope", func(cs transformconfig.ContextStatements) error {
			_, err := traces.ParseContextStatements(cs)
			return err
		}, transformconfig.Scope, `set(attributes["x"], "y")`},
		{"metric", func(cs transformconfig.ContextStatements) error {
			_, err := metrics.ParseContextStatements(cs)
			return err
		}, transformconfig.Metric, `set(description, "y")`},
		{"datapoint", func(cs transformconfig.ContextStatements) error {
			_, err := metrics.ParseContextStatements(cs)
			return err
		}, transformconfig.DataPoint, `set(attributes["x"], "y")`},
		{"exemplar", func(cs transformconfig.ContextStatements) error {
			_, err := metrics.ParseContextStatements(cs)
			return err
		}, transformconfig.Exemplar, `set(filtered_attributes["x"], "y")`},
		{"log", func(cs transformconfig.ContextStatements) error {
			_, err := logs.ParseContextStatements(cs)
			return err
		}, transformconfig.Log, `set(attributes["x"], "y")`},
		{"profile", func(cs transformconfig.ContextStatements) error {
			_, err := profiles.ParseContextStatements(cs)
			return err
		}, transformconfig.Profile, `set(original_payload_format, "y")`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, tc.parse(transformconfig.ContextStatements{Context: tc.ctx, Conditions: badCond, Statements: []string{tc.stmt}}))
		})
	}
}

func TestParse_InferredResourceScope(t *testing.T) {
	pc := newTraceParserCollection(t)

	resource, err := pc.ParseContextStatements(transformconfig.ContextStatements{Statements: []string{`set(resource.attributes["x"], "y")`}})
	require.NoError(t, err)
	assert.Equal(t, transformconfig.Resource, resource.Context())

	scope, err := pc.ParseContextStatements(transformconfig.ContextStatements{Statements: []string{`set(scope.attributes["x"], "y")`}})
	require.NoError(t, err)
	assert.Equal(t, transformconfig.Scope, scope.Context())

	spanEvent, err := pc.ParseContextStatements(transformconfig.ContextStatements{Statements: []string{`set(spanevent.attributes["x"], "y")`}})
	require.NoError(t, err)
	assert.Equal(t, transformconfig.SpanEvent, spanEvent.Context())

	mc := newMetricParserCollection(t)

	metric, err := mc.ParseContextStatements(transformconfig.ContextStatements{Statements: []string{`set(metric.description, "y")`}})
	require.NoError(t, err)
	assert.Equal(t, transformconfig.Metric, metric.Context())

	dataPoint, err := mc.ParseContextStatements(transformconfig.ContextStatements{Statements: []string{`set(datapoint.attributes["x"], "y")`}})
	require.NoError(t, err)
	assert.Equal(t, transformconfig.DataPoint, dataPoint.Context())

	exemplar, err := mc.ParseContextStatements(transformconfig.ContextStatements{Statements: []string{`set(exemplar.filtered_attributes["x"], "y") where metric.name == "gauge"`}})
	require.NoError(t, err)
	assert.Equal(t, transformconfig.Exemplar, exemplar.Context())
}

func TestConsumeTraces_SpanEventExecuteError(t *testing.T) {
	consumer, err := newTraceParserCollection(t).
		ParseContextStatements(transformconfig.ContextStatements{Context: transformconfig.SpanEvent, Statements: []string{errStmt}})
	require.NoError(t, err)
	require.Error(t, consumer.ConsumeTraces(t.Context(), newTestTraces(), nil))
}

func gaugeMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("gauge")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.0)
	dp.Exemplars().AppendEmpty().SetDoubleValue(1.0)
	return md
}

func sumMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("sum")
	dp := m.SetEmptySum().DataPoints().AppendEmpty()
	dp.SetDoubleValue(2.0)
	dp.Exemplars().AppendEmpty().SetDoubleValue(2.0)
	return md
}

func histogramMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("histogram")
	dp := m.SetEmptyHistogram().DataPoints().AppendEmpty()
	dp.SetCount(1)
	dp.Exemplars().AppendEmpty().SetDoubleValue(3.0)
	return md
}

func expHistogramMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("exp_histogram")
	dp := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	dp.SetCount(1)
	dp.Exemplars().AppendEmpty().SetDoubleValue(4.0)
	return md
}

func summaryMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("summary")
	m.SetEmptySummary().DataPoints().AppendEmpty().SetCount(1)
	return md
}
