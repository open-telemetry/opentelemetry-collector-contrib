// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceStatements_ConsumeAllSignals(t *testing.T) {
	cs := ContextStatements{Context: Resource, Statements: []string{`set(attributes["test"], "pass")`}}

	t.Run("traces", func(t *testing.T) {
		consumer, err := newTraceParserCollection(t).ParseContextStatements(cs)
		require.NoError(t, err)
		assert.Equal(t, Resource, consumer.Context())
		td := newTestTraces()
		require.NoError(t, consumer.ConsumeTraces(t.Context(), td, nil))
		v, ok := td.ResourceSpans().At(0).Resource().Attributes().Get("test")
		require.True(t, ok)
		assert.Equal(t, "pass", v.Str())
	})

	t.Run("metrics", func(t *testing.T) {
		consumer, err := newMetricParserCollection(t).ParseContextStatements(cs)
		require.NoError(t, err)
		md := newTestMetrics()
		require.NoError(t, consumer.ConsumeMetrics(t.Context(), md, nil))
		v, ok := md.ResourceMetrics().At(0).Resource().Attributes().Get("test")
		require.True(t, ok)
		assert.Equal(t, "pass", v.Str())
	})

	t.Run("logs", func(t *testing.T) {
		consumer, err := newLogParserCollection(t).ParseContextStatements(cs)
		require.NoError(t, err)
		ld := newTestLogs()
		require.NoError(t, consumer.ConsumeLogs(t.Context(), ld, nil))
		v, ok := ld.ResourceLogs().At(0).Resource().Attributes().Get("test")
		require.True(t, ok)
		assert.Equal(t, "pass", v.Str())
	})

	t.Run("profiles", func(t *testing.T) {
		consumer, err := newProfileParserCollection(t).ParseContextStatements(cs)
		require.NoError(t, err)
		pd := newTestProfiles()
		require.NoError(t, consumer.ConsumeProfiles(t.Context(), pd, nil))
		v, ok := pd.ResourceProfiles().At(0).Resource().Attributes().Get("test")
		require.True(t, ok)
		assert.Equal(t, "pass", v.Str())
	})
}

func TestScopeStatements_ConsumeAllSignals(t *testing.T) {
	cs := ContextStatements{Context: Scope, Conditions: []string{`name == "scope"`}, Statements: []string{`set(attributes["test"], "pass")`}}

	t.Run("traces", func(t *testing.T) {
		consumer, err := newTraceParserCollection(t).ParseContextStatements(cs)
		require.NoError(t, err)
		assert.Equal(t, Scope, consumer.Context())
		td := newTestTraces()
		require.NoError(t, consumer.ConsumeTraces(t.Context(), td, nil))
		v, ok := td.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().Get("test")
		require.True(t, ok)
		assert.Equal(t, "pass", v.Str())
	})

	t.Run("metrics", func(t *testing.T) {
		consumer, err := newMetricParserCollection(t).ParseContextStatements(cs)
		require.NoError(t, err)
		md := newTestMetrics()
		require.NoError(t, consumer.ConsumeMetrics(t.Context(), md, nil))
		v, ok := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().Get("test")
		require.True(t, ok)
		assert.Equal(t, "pass", v.Str())
	})

	t.Run("logs", func(t *testing.T) {
		consumer, err := newLogParserCollection(t).ParseContextStatements(cs)
		require.NoError(t, err)
		ld := newTestLogs()
		require.NoError(t, consumer.ConsumeLogs(t.Context(), ld, nil))
		v, ok := ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().Get("test")
		require.True(t, ok)
		assert.Equal(t, "pass", v.Str())
	})

	t.Run("profiles", func(t *testing.T) {
		consumer, err := newProfileParserCollection(t).ParseContextStatements(cs)
		require.NoError(t, err)
		pd := newTestProfiles()
		require.NoError(t, consumer.ConsumeProfiles(t.Context(), pd, nil))
		v, ok := pd.ResourceProfiles().At(0).ScopeProfiles().At(0).Scope().Attributes().Get("test")
		require.True(t, ok)
		assert.Equal(t, "pass", v.Str())
	})
}

func TestResourceScopeStatements_PropagateError(t *testing.T) {
	// erroring statement (ParseJSON of a non-object) drives the Execute error
	// branch of the resource and scope consumers for every signal.
	stmt := []string{`set(attributes["test"], ParseJSON("1"))`}

	for _, ctx := range []ContextID{Resource, Scope} {
		cs := ContextStatements{Context: ctx, Statements: stmt}
		t.Run(string(ctx), func(t *testing.T) {
			traceConsumer, err := newTraceParserCollection(t).ParseContextStatements(cs)
			require.NoError(t, err)
			require.Error(t, traceConsumer.ConsumeTraces(t.Context(), newTestTraces(), nil))

			metricConsumer, err := newMetricParserCollection(t).ParseContextStatements(cs)
			require.NoError(t, err)
			require.Error(t, metricConsumer.ConsumeMetrics(t.Context(), newTestMetrics(), nil))

			logConsumer, err := newLogParserCollection(t).ParseContextStatements(cs)
			require.NoError(t, err)
			require.Error(t, logConsumer.ConsumeLogs(t.Context(), newTestLogs(), nil))

			profileConsumer, err := newProfileParserCollection(t).ParseContextStatements(cs)
			require.NoError(t, err)
			require.Error(t, profileConsumer.ConsumeProfiles(t.Context(), newTestProfiles(), nil))
		})
	}
}
