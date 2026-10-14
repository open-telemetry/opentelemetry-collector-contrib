// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlexemplar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func newMetricParserCollection(t *testing.T) *MetricParserCollection {
	t.Helper()
	pc, err := NewMetricParserCollection(
		componenttest.NewNopTelemetrySettings(),
		WithMetricParser(ottlfuncs.StandardFuncs[*ottlmetric.TransformContext]()),
		WithDataPointParser(ottlfuncs.StandardFuncs[*ottldatapoint.TransformContext]()),
		WithExemplarParser(ottlfuncs.StandardFuncs[*ottlexemplar.TransformContext]()),
		WithMetricErrorMode(ottl.PropagateError),
	)
	require.NoError(t, err)
	return pc
}

func TestMetricParserCollection_MetricContext(t *testing.T) {
	pc := newMetricParserCollection(t)
	consumer, err := pc.ParseContextStatements(ContextStatements{Context: Metric, Statements: []string{`set(description, "pass")`}})
	require.NoError(t, err)
	assert.Equal(t, Metric, consumer.Context())

	md := newTestMetrics()
	require.NoError(t, consumer.ConsumeMetrics(t.Context(), md, nil))

	metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < metrics.Len(); i++ {
		assert.Equal(t, "pass", metrics.At(i).Description())
	}
}

func TestMetricParserCollection_DataPointContext(t *testing.T) {
	pc := newMetricParserCollection(t)
	consumer, err := pc.ParseContextStatements(ContextStatements{Context: DataPoint, Statements: []string{`set(attributes["test"], "pass")`}})
	require.NoError(t, err)
	assert.Equal(t, DataPoint, consumer.Context())

	md := newTestMetrics()
	require.NoError(t, consumer.ConsumeMetrics(t.Context(), md, nil))

	metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	v, ok := metrics.At(0).Gauge().DataPoints().At(0).Attributes().Get("test")
	require.True(t, ok)
	assert.Equal(t, "pass", v.Str())
}

func TestMetricParserCollection_ExemplarContext(t *testing.T) {
	pc := newMetricParserCollection(t)
	consumer, err := pc.ParseContextStatements(ContextStatements{Context: Exemplar, Statements: []string{`set(filtered_attributes["test"], "pass")`}})
	require.NoError(t, err)
	assert.Equal(t, Exemplar, consumer.Context())

	md := newTestMetrics()
	require.NoError(t, consumer.ConsumeMetrics(t.Context(), md, nil))

	metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	v, ok := metrics.At(0).Gauge().DataPoints().At(0).Exemplars().At(0).FilteredAttributes().Get("test")
	require.True(t, ok)
	assert.Equal(t, "pass", v.Str())
}

func TestMetricParserCollection_ConsumeMetrics_PropagatesError(t *testing.T) {
	tests := []struct {
		name string
		cs   ContextStatements
	}{
		{"metric", ContextStatements{Context: Metric, Statements: []string{`set(name, ParseJSON("1"))`}}},
		{"datapoint", ContextStatements{Context: DataPoint, Statements: []string{`set(attributes["test"], ParseJSON("1"))`}}},
		{"exemplar", ContextStatements{Context: Exemplar, Statements: []string{`set(filtered_attributes["test"], ParseJSON("1"))`}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := newMetricParserCollection(t)
			consumer, err := pc.ParseContextStatements(tt.cs)
			require.NoError(t, err)
			require.Error(t, consumer.ConsumeMetrics(t.Context(), newTestMetrics(), nil))
		})
	}
}

func TestMetricParserCollection_ParseContextStatements_Error(t *testing.T) {
	pc := newMetricParserCollection(t)
	_, err := pc.ParseContextStatements(ContextStatements{Context: Metric, Statements: []string{`not a valid statement`}})
	require.Error(t, err)
}
