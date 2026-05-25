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

func TestAssertMetrics_AttributeExistsMatcher(t *testing.T) {
	m := buildSampleMetrics()
	rm := m.ResourceMetrics().At(0)
	rm.Resource().Attributes().PutStr("service.instance.id", "generated-1")
	dps := rm.ScopeMetrics().At(0).Metrics().At(1).Sum().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dps.At(i).Attributes().PutStr("request.id", "request-1")
	}

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes:
        service.instance.id/exists: true
        service.name: svc
      scopes:
        - name: github.com/example/receiver
          version: v0.0.1
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
            - name: svc.requests
              type: sum
              unit: "{requests}"
              temporality: cumulative
              monotonic: true
              datapoints:
                - attributes:
                    method: GET
                    request.id/exists: true
                - attributes:
                    method: POST
                    request.id/exists: true
`), 0o600))

	require.NoError(t, AssertMetrics(path, m))

	rm.Resource().Attributes().PutStr("service.instance.id", "generated-2")
	dps.At(0).Attributes().PutStr("request.id", "request-2")
	require.NoError(t, AssertMetrics(path, m))
}

func TestAssertMetrics_AttributeExistsMatcherIsOrderInsensitive(t *testing.T) {
	m := pmetric.NewMetrics()
	appendResourceWithKindAndID(m, "zzz", "a")
	appendResourceWithKindAndID(m, "aaa", "b")

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes:
        id/exists: true
        kind: a
      scopes:
        - name: scope
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
    - attributes:
        id/exists: true
        kind: b
      scopes:
        - name: scope
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
`), 0o600))

	require.NoError(t, AssertMetrics(path, m))
}

func TestAssertMetrics_AttributeExistsMatcherOnDatapointsIsOrderInsensitive(t *testing.T) {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("scope")
	g := sm.Metrics().AppendEmpty()
	g.SetName("svc.active")
	g.SetUnit("1")
	dps := g.SetEmptyGauge().DataPoints()
	appendDatapointWithKindAndID(dps, "zzz", "a")
	appendDatapointWithKindAndID(dps, "aaa", "b")

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - scopes:
        - name: scope
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
              datapoints:
                - attributes:
                    id/exists: true
                    kind: a
                - attributes:
                    id/exists: true
                    kind: b
`), 0o600))

	require.NoError(t, AssertMetrics(path, m))
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

func appendResourceWithKindAndID(m pmetric.Metrics, id, kind string) {
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("id", id)
	rm.Resource().Attributes().PutStr("kind", kind)

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("scope")

	g := sm.Metrics().AppendEmpty()
	g.SetName("svc.active")
	g.SetUnit("1")
	g.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
}

func appendDatapointWithKindAndID(dps pmetric.NumberDataPointSlice, id, kind string) {
	dp := dps.AppendEmpty()
	dp.Attributes().PutStr("id", id)
	dp.Attributes().PutStr("kind", kind)
	dp.SetIntValue(1)
}

func TestAssertMetrics_AttributeIncludeResourceAttributes(t *testing.T) {
	m := buildSampleMetrics()
	// Add an extra resource attribute that the assertion does not mention.
	rm := m.ResourceMetrics().At(0)
	rm.Resource().Attributes().PutStr("extra.env", "staging")

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes/include:
        service.name: svc
      scopes:
        - name: github.com/example/receiver
          version: v0.0.1
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
            - name: svc.requests
              type: sum
              unit: "{requests}"
              temporality: cumulative
              monotonic: true
              datapoints:
                - attributes:
                    method: GET
                - attributes:
                    method: POST
`), 0o600))

	require.NoError(t, AssertMetrics(path, m))
}

func TestAssertMetrics_AttributeIncludeResourceAttributesMissingKey(t *testing.T) {
	m := buildSampleMetrics()

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	// Assert an attribute that does not exist on the resource.
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes/include:
        service.name: svc
        missing.key: required
      scopes:
        - name: github.com/example/receiver
          version: v0.0.1
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
            - name: svc.requests
              type: sum
              unit: "{requests}"
              temporality: cumulative
              monotonic: true
              datapoints:
                - attributes:
                    method: GET
                - attributes:
                    method: POST
`), 0o600))

	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing expected resource")
}

func TestAssertMetrics_AttributeIncludeDatapointAttributes(t *testing.T) {
	m := buildSampleMetrics()
	// Add extra datapoint attributes that the assertion does not mention.
	rm := m.ResourceMetrics().At(0)
	dps := rm.ScopeMetrics().At(0).Metrics().At(1).Sum().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dps.At(i).Attributes().PutStr("region", "us-east-1")
	}

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes:
        service.name: svc
      scopes:
        - name: github.com/example/receiver
          version: v0.0.1
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
            - name: svc.requests
              type: sum
              unit: "{requests}"
              temporality: cumulative
              monotonic: true
              datapoints:
                - attributes/include:
                    method: GET
                - attributes/include:
                    method: POST
`), 0o600))

	require.NoError(t, AssertMetrics(path, m))
}

func TestAssertMetrics_AttributeIncludeWithExists(t *testing.T) {
	m := buildSampleMetrics()
	rm := m.ResourceMetrics().At(0)
	rm.Resource().Attributes().PutStr("service.instance.id", "generated-abc")
	rm.Resource().Attributes().PutStr("extra.env", "staging")

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes/include:
        service.name: svc
        service.instance.id/exists: true
      scopes:
        - name: github.com/example/receiver
          version: v0.0.1
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
            - name: svc.requests
              type: sum
              unit: "{requests}"
              temporality: cumulative
              monotonic: true
              datapoints:
                - attributes:
                    method: GET
                - attributes:
                    method: POST
`), 0o600))

	require.NoError(t, AssertMetrics(path, m))
}

func TestAssertMetrics_AttributeIncludeBothKeysIsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes:
        service.name: svc
      attributes/include:
        service.name: svc
      scopes:
        - name: scope
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
`), 0o600))

	err := AssertMetrics(path, buildSampleMetrics())
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot specify both")
}
