// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

// The grouper under test routes each metric on its name.
const metricsSubjectExpr = `metric.name`

// appendMetric adds a gauge metric named name (also the routing key).
func appendMetric(sm pmetric.ScopeMetrics, name string) {
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	m.SetEmptyGauge()
}

func metricGroupsBySubject(t *testing.T, groups []Group[pmetric.Metrics]) map[string]pmetric.Metrics {
	t.Helper()
	m := make(map[string]pmetric.Metrics, len(groups))
	for _, g := range groups {
		_, dup := m[g.Subject]
		require.Falsef(t, dup, "duplicate subject %q", g.Subject)
		m[g.Subject] = g.Data
	}
	return m
}

func TestMetricsGrouper(t *testing.T) {
	t.Parallel()

	t.Run("splits one scope across subjects", func(t *testing.T) {
		g, err := NewMetricsGrouper(metricsSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := pmetric.NewMetrics()
		sm := in.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
		appendMetric(sm, "a")
		appendMetric(sm, "b")

		groups, err := g.Group(t.Context(), in)
		require.NoError(t, err)
		got := metricGroupsBySubject(t, groups)
		require.Len(t, got, 2)

		wantA := pmetric.NewMetrics()
		appendMetric(wantA.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty(), "a")
		assert.NoError(t, pmetrictest.CompareMetrics(wantA, got["a"]))

		wantB := pmetric.NewMetrics()
		appendMetric(wantB.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty(), "b")
		assert.NoError(t, pmetrictest.CompareMetrics(wantB, got["b"]))
	})

	t.Run("merges one subject across resources and preserves boundaries", func(t *testing.T) {
		g, err := NewMetricsGrouper(metricsSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := pmetric.NewMetrics()
		rm1 := in.ResourceMetrics().AppendEmpty()
		rm1.Resource().Attributes().PutStr("host", "h1")
		appendMetric(rm1.ScopeMetrics().AppendEmpty(), "a")
		rm2 := in.ResourceMetrics().AppendEmpty()
		rm2.Resource().Attributes().PutStr("host", "h2")
		appendMetric(rm2.ScopeMetrics().AppendEmpty(), "a")

		groups, err := g.Group(t.Context(), in)
		require.NoError(t, err)
		got := metricGroupsBySubject(t, groups)
		require.Len(t, got, 1)

		want := pmetric.NewMetrics()
		w1 := want.ResourceMetrics().AppendEmpty()
		w1.Resource().Attributes().PutStr("host", "h1")
		appendMetric(w1.ScopeMetrics().AppendEmpty(), "a")
		w2 := want.ResourceMetrics().AppendEmpty()
		w2.Resource().Attributes().PutStr("host", "h2")
		appendMetric(w2.ScopeMetrics().AppendEmpty(), "a")
		assert.NoError(t, pmetrictest.CompareMetrics(want, got["a"]))
	})

	t.Run("empty input yields no groups", func(t *testing.T) {
		g, err := NewMetricsGrouper(metricsSubjectExpr, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		groups, err := g.Group(t.Context(), pmetric.NewMetrics())
		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("a subject expression that fails to evaluate drops the metric", func(t *testing.T) {
		g, err := NewMetricsGrouper(`Len(0)`, componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)

		in := pmetric.NewMetrics()
		appendMetric(in.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty(), "a")

		groups, err := g.Group(t.Context(), in)
		assert.Error(t, err)
		assert.Empty(t, groups)
	})
}
