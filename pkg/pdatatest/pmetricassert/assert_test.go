// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestAssertMetrics_RoundTrip(t *testing.T) {
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, m))
	require.NoError(t, AssertMetrics(path, m))
}

func TestAssertMetrics_IgnoresValuesAndTimestamps(t *testing.T) {
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, m))

	// Mutate values and timestamps; assertion must still pass because the
	// default schema only compares identity fields.
	rm := m.ResourceMetrics().At(0)
	metric := rm.ScopeMetrics().At(0).Metrics().At(1) // the sum metric
	dp := metric.Sum().DataPoints().At(0)
	dp.SetIntValue(dp.IntValue() + 9999)
	dp.SetTimestamp(dp.Timestamp() + 1_000_000_000)

	require.NoError(t, AssertMetrics(path, m))
}

func TestAssertMetrics_DetectsMissingMetric(t *testing.T) {
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, m))

	// Remove the sum metric; assertion should fail.
	rm := m.ResourceMetrics().At(0)
	metrics := rm.ScopeMetrics().At(0).Metrics()
	metrics.RemoveIf(func(metric pmetric.Metric) bool {
		return metric.Name() == "svc.requests"
	})

	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), `missing expected metric "svc.requests"`)
}

func TestAssertMetrics_DetectsUnexpectedDatapoint(t *testing.T) {
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, m))

	// Add an unexpected datapoint attribute permutation.
	rm := m.ResourceMetrics().At(0)
	sum := rm.ScopeMetrics().At(0).Metrics().At(1).Sum()
	dp := sum.DataPoints().AppendEmpty()
	dp.Attributes().PutStr("method", "PATCH")
	dp.SetIntValue(1)

	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected datapoint")
}

func TestAssertMetrics_SingleEmptyDatapointShorthand(t *testing.T) {
	// A YAML snippet that omits `datapoints:` entirely must match a metric
	// with exactly one datapoint that has no attributes.
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("scope")
	g := sm.Metrics().AppendEmpty()
	g.SetName("svc.active")
	g.SetUnit("1")
	g.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, m))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "datapoints:",
		"single empty-attribute datapoint should be compacted to no `datapoints:` key")

	require.NoError(t, AssertMetrics(path, m))
}

func buildSampleMetrics() pmetric.Metrics {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("github.com/example/receiver")
	sm.Scope().SetVersion("v0.0.1")

	// Gauge
	g := sm.Metrics().AppendEmpty()
	g.SetName("svc.active")
	g.SetUnit("1")
	gp := g.SetEmptyGauge().DataPoints().AppendEmpty()
	gp.SetIntValue(7)
	gp.SetTimestamp(pcommon.Timestamp(1))

	// Sum with attributes
	s := sm.Metrics().AppendEmpty()
	s.SetName("svc.requests")
	s.SetUnit("{requests}")
	sum := s.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	for _, method := range []string{"GET", "POST"} {
		dp := sum.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("method", method)
		dp.SetIntValue(42)
		dp.SetTimestamp(pcommon.Timestamp(1))
	}

	return m
}
