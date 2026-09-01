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

func TestAssertMetrics_IncludeValues(t *testing.T) {
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, m, IncludeValues()))

	// Assertion must pass against the original metrics.
	require.NoError(t, AssertMetrics(path, m))

	// Mutate values; assertion must fail because the snapshot includes values.
	rm := m.ResourceMetrics().At(0)
	metric := rm.ScopeMetrics().At(0).Metrics().At(1) // the sum metric
	dp := metric.Sum().DataPoints().At(0)
	dp.SetIntValue(dp.IntValue() + 9999)

	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "value mismatch")
}

func TestWriteAssertionFile_IncludeHistogramExplicitBounds(t *testing.T) {
	m := buildHistogramMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, m, IncludeHistogramExplicitBounds()))

	doc, err := readDocument(path)
	require.NoError(t, err)
	datapoints := doc.Resources[0].Scopes[0].Metrics[0].Datapoints
	require.Equal(t, []datapointAssertion{{
		ExplicitBounds: &[]float64{0.005, 0.01, 0.025},
	}}, datapoints)

	dp := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0)
	dp.SetCount(999)
	dp.SetSum(123.45)
	dp.SetMin(0.001)
	dp.SetMax(100)
	dp.BucketCounts().FromRaw([]uint64{9, 8, 7, 6})
	require.NoError(t, AssertMetrics(path, m))

	dp.ExplicitBounds().FromRaw([]float64{0.005, 0.02, 0.025})
	err = AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "explicit_bounds mismatch")
}

func TestWriteAssertionFile_IncludeEmptyHistogramExplicitBounds(t *testing.T) {
	m := buildHistogramMetrics()
	dp := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0)
	dp.ExplicitBounds().FromRaw([]float64{})
	dp.BucketCounts().FromRaw([]uint64{10})

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, m, IncludeHistogramExplicitBounds()))
	require.NoError(t, AssertMetrics(path, m))

	dp.ExplicitBounds().FromRaw([]float64{1})
	dp.BucketCounts().FromRaw([]uint64{0, 10})
	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "explicit_bounds mismatch")
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

func TestAssertMetrics_AttributeRegexMatcher(t *testing.T) {
	m := buildSampleMetrics()
	rm := m.ResourceMetrics().At(0)
	rm.Resource().Attributes().PutStr("host.name", "worker-42")
	dps := rm.ScopeMetrics().At(0).Metrics().At(1).Sum().DataPoints()
	dps.At(0).Attributes().PutStr("request.id", "request-123")
	dps.At(1).Attributes().PutStr("request.id", "request-456")

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes:
        host.name/regex: worker-[0-9]+
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
                    request.id/regex: request-[0-9]+
                - attributes:
                    method: POST
                    request.id/regex: request-[0-9]+
`), 0o600))

	require.NoError(t, AssertMetrics(path, m))

	rm.Resource().Attributes().PutStr("host.name", "worker-7")
	dps.At(0).Attributes().PutStr("request.id", "request-789")
	require.NoError(t, AssertMetrics(path, m))
}

func TestCompareAttributes_RegexMatcherRequiresFullStringMatch(t *testing.T) {
	err := compareAttributes(
		map[string]any{"host.name/regex": "worker-[0-9]+"},
		map[string]any{"host.name": "worker-42-extra"},
		attributeModeExact,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), `attribute "host.name" value "worker-42-extra" does not match regex "worker-[0-9]+"`)
}

func TestCompareAttributes_RegexMatcherSchemaErrors(t *testing.T) {
	t.Run("expected value must be string", func(t *testing.T) {
		err := compareAttributes(
			map[string]any{"host.name/regex": true},
			map[string]any{"host.name": "worker-42"},
			attributeModeExact,
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), `attribute "host.name"/regex must be a string pattern`)
	})

	t.Run("actual value must be string", func(t *testing.T) {
		err := compareAttributes(
			map[string]any{"host.name/regex": "worker-[0-9]+"},
			map[string]any{"host.name": int64(42)},
			attributeModeExact,
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), `attribute "host.name" must be a string to match /regex`)
	})

	t.Run("pattern must compile", func(t *testing.T) {
		err := compareAttributes(
			map[string]any{"host.name/regex": "["},
			map[string]any{"host.name": "worker-42"},
			attributeModeExact,
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), `attribute "host.name"/regex has invalid pattern "["`)
	})
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

func TestAssertMetrics_ScopeVersionExistsMatcher(t *testing.T) {
	// The scope version/exists matcher is the assertion-file equivalent of
	// pmetrictest.IgnoreScopeVersion: the version must be present but its
	// exact value is volatile.
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes:
        service.name: svc
      scopes:
        - name: github.com/example/receiver
          version/exists: true
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

	// A different scope version still satisfies version/exists.
	m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().SetVersion("v9.9.9")
	require.NoError(t, AssertMetrics(path, m))
}

func TestAssertMetrics_ScopeVersionExistsMatcherDetectsMissingVersion(t *testing.T) {
	m := buildSampleMetrics()
	m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().SetVersion("")

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes:
        service.name: svc
      scopes:
        - name: github.com/example/receiver
          version/exists: true
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
	require.Contains(t, err.Error(), "missing expected scope")
}

func TestAssertMetrics_ScopeVersionRegexMatcher(t *testing.T) {
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes:
        service.name: svc
      scopes:
        - name: github.com/example/receiver
          version/regex: v[0-9]+\.[0-9]+\.[0-9]+
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

	// Another semver-shaped version still matches.
	m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().SetVersion("v1.2.3")
	require.NoError(t, AssertMetrics(path, m))

	// A non-matching version fails.
	m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().SetVersion("snapshot")
	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing expected scope")

	// The pattern must match the full version: a value the regex matches only
	// as a prefix must still fail (the matcher anchors with ^(?:...)$).
	m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().SetVersion("v1.2.3-rc1")
	err = AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing expected scope")
}

func TestAssertMetrics_ScopeMatcherRoundTripStaysExact(t *testing.T) {
	// WriteAssertionFile must keep emitting plain name:/version: scalars, not
	// operator-suffixed keys, so generated snapshots are unchanged.
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, m))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(raw), "name: github.com/example/receiver")
	require.Contains(t, string(raw), "version: v0.0.1")
	require.NotContains(t, string(raw), "/exists")
	require.NotContains(t, string(raw), "/regex")

	require.NoError(t, AssertMetrics(path, m))
}

func TestReadDocument_ScopeMatcherSchemaErrors(t *testing.T) {
	t.Run("exists must be true", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
		require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - scopes:
        - name: scope
          version/exists: false
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
`), 0o600))
		_, err := readDocument(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "scope version/exists must be true")
	})

	t.Run("at most one version matcher", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
		require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - scopes:
        - name: scope
          version: v1
          version/regex: v[0-9]+
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
`), 0o600))
		_, err := readDocument(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "must use at most one of")
	})

	t.Run("regex pattern must compile", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
		require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - scopes:
        - name: scope
          version/regex: "["
          metrics:
            - name: svc.active
              type: gauge
              unit: "1"
`), 0o600))
		_, err := readDocument(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), `scope version/regex has invalid pattern "["`)
	})
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

func buildHistogramMetrics() pmetric.Metrics {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("scope")

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("request.duration")
	histogram := metric.SetEmptyHistogram()
	histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := histogram.DataPoints().AppendEmpty()
	dp.SetCount(10)
	dp.SetSum(0.5)
	dp.SetMin(0.001)
	dp.SetMax(0.2)
	dp.ExplicitBounds().FromRaw([]float64{0.005, 0.01, 0.025})
	dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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

func TestAssertMetrics_NumericValueModifiers(t *testing.T) {
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")

	// The sample sum "svc.requests" has int_value 42 for both GET and POST.
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
                - attributes:
                    method: GET
                  int_value/gt: 40
                  int_value/lt: 50
                - attributes:
                    method: POST
                  int_value/gte: 42
                  int_value/lte: 42
`), 0o600))

	require.NoError(t, AssertMetrics(path, m))

	// Update values to violate conditions.
	rm := m.ResourceMetrics().At(0)
	dps := rm.ScopeMetrics().At(0).Metrics().At(1).Sum().DataPoints()
	dps.At(0).SetIntValue(39) // GET < 40 (fails int_value/gt: 40)

	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), `int_value 39 is not > 40`)
}

func TestAssertMetrics_NumericDoubleValueModifiers(t *testing.T) {
	m := buildSampleMetrics()
	// Turn svc.active into a double gauge with a runtime-dependent value.
	rm := m.ResourceMetrics().At(0)
	gauge := rm.ScopeMetrics().At(0).Metrics().At(0).Gauge()
	gauge.DataPoints().At(0).SetDoubleValue(3.5)

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
              datapoints:
                - double_value/gt: 0
                  double_value/lt: 10
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

	gauge.DataPoints().At(0).SetDoubleValue(11) // not < 10
	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), `double_value 11 is not < 10`)
}

func TestAssertMetrics_UnknownDatapointOperator(t *testing.T) {
	m := buildSampleMetrics()
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
                - attributes:
                    method: GET
                  int_value/gtee: 40
                  int_value/regex: "4."
                - attributes:
                    method: POST
`), 0o600))

	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), `unsupported datapoint assertion key "int_value/gtee"`)
	require.Contains(t, err.Error(), `unsupported datapoint assertion key "int_value/regex"`)
}

func TestAssertMetrics_UnknownAttributeOperatorFailsLoudly(t *testing.T) {
	m := buildSampleMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")

	// A mistyped operator on an attribute key falls back to exact matching
	// on the literal key (attribute keys may legitimately contain '/'), so
	// the assertion must fail rather than silently pass.
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
                - attributes:
                    method: GET
                    method/gtee: GET
                - attributes:
                    method: POST
`), 0o600))

	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), `missing datapoint with attributes`)
}

func TestAssertMetrics_NumericAttributeModifiers(t *testing.T) {
	m := buildSampleMetrics()
	rm := m.ResourceMetrics().At(0)
	rm.Resource().Attributes().PutInt("queue.depth", 10)

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
signal: metrics
resources:
    - attributes:
        service.name: svc
        queue.depth/gte: 5
        queue.depth/lt: 20
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

	// Update attribute to violate condition.
	rm.Resource().Attributes().PutInt("queue.depth", 30) // >= 5, but not < 20
	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), `missing expected resource:`)
}

// TestAssertMetrics_NumericOperatorsExample exercises the committed
// testdata/numeric_operators.assert.yaml example against metrics whose values
// are meaningful but not exact — the motivating use case for the numeric
// comparison operators.
func TestAssertMetrics_NumericOperatorsExample(t *testing.T) {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "example")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("github.com/example/receiver")
	sm.Scope().SetVersion("v0.1.0")

	duration := sm.Metrics().AppendEmpty()
	duration.SetName("http.server.request.duration")
	duration.SetUnit("s")
	duration.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(0.42) // runtime-dependent

	active := sm.Metrics().AppendEmpty()
	active.SetName("http.server.active_requests")
	active.SetUnit("{requests}")
	activeSum := active.SetEmptySum()
	activeSum.SetIsMonotonic(false)
	activeSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	activeSum.DataPoints().AppendEmpty().SetIntValue(3) // runtime-dependent

	path := filepath.Join("testdata", "numeric_operators.assert.yaml")
	require.NoError(t, AssertMetrics(path, m))

	// A value outside the asserted range fails.
	duration.Gauge().DataPoints().At(0).SetDoubleValue(120)
	err := AssertMetrics(path, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), `double_value 120 is not < 60`)
}
