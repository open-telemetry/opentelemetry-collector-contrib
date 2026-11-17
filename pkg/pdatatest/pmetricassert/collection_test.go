// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// assertSample runs body against buildSampleMetrics: one resource
// (service.name=svc), one scope (github.com/example/receiver v0.0.1), and the
// metrics svc.active (gauge, one attribute-less datapoint) and svc.requests
// (sum, datapoints method=GET and method=POST).
func assertSample(t *testing.T, body string) error {
	t.Helper()
	return AssertMetrics(writeAssertionYAML(t, body), buildSampleMetrics())
}

// A metric matched by metrics/include that omits datapoints asserts the
// metric's identity only. This is the primary case from #48472: pinning the
// datapoints of a multi-series metric is exactly what the operator avoids.
func TestAssertMetrics_MetricsIncludeIgnoresUnlistedDatapoints(t *testing.T) {
	require.NoError(t, assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics/include:
          - name: svc.requests
            type: sum
            unit: "{requests}"
            temporality: cumulative
            monotonic: true
`))
}

// metrics/include still validates the identity of the metrics it does list.
func TestAssertMetrics_MetricsIncludeChecksListedMetric(t *testing.T) {
	err := assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics/include:
          - name: svc.requests
            type: sum
            unit: "{wrong}"
            temporality: cumulative
            monotonic: true
`)
	require.ErrorContains(t, err, `unit mismatch: expected "{wrong}", got "{requests}"`)
}

func TestAssertMetrics_MetricsIncludeMissingMetric(t *testing.T) {
	err := assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics/include:
          - name: svc.latency
            type: histogram
`)
	require.ErrorContains(t, err, `missing expected metric "svc.latency"`)
}

// Exact is the default: an unlisted metric fails without an operator.
func TestAssertMetrics_ExactIsTheDefaultForCollections(t *testing.T) {
	err := assertSample(t, `version: 1
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
`)
	require.ErrorContains(t, err, `unexpected metric "svc.requests"`)
}

// An exact collection nested inside an /include item stays exact.
func TestAssertMetrics_ExactCollectionNestedUnderInclude(t *testing.T) {
	err := assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics:
          - name: svc.active
            type: gauge
            unit: "1"
`)
	require.ErrorContains(t, err, `unexpected metric "svc.requests"`)
}

func TestAssertMetrics_DatapointsInclude(t *testing.T) {
	t.Run("subset of datapoints", func(t *testing.T) {
		require.NoError(t, assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics/include:
          - name: svc.requests
            type: sum
            unit: "{requests}"
            temporality: cumulative
            monotonic: true
            datapoints/include:
              - attributes:
                  method: GET
`))
	})

	t.Run("values are still compared", func(t *testing.T) {
		err := assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics/include:
          - name: svc.requests
            type: sum
            unit: "{requests}"
            temporality: cumulative
            monotonic: true
            datapoints/include:
              - attributes:
                  method: GET
                int_value: 41
`)
		require.ErrorContains(t, err, "int_value mismatch: expected 41, got 42")
	})

	t.Run("missing datapoint", func(t *testing.T) {
		err := assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics/include:
          - name: svc.requests
            type: sum
            unit: "{requests}"
            temporality: cumulative
            monotonic: true
            datapoints/include:
              - attributes:
                  method: DELETE
`)
		require.ErrorContains(t, err, `missing datapoint with attributes [["method","DELETE"]]`)
	})
}

func TestAssertMetrics_ResourcesAndScopesInclude(t *testing.T) {
	m := buildSampleMetrics()
	// A second resource and scope the assertion does not mention.
	appendResourceWithKindAndID(m, "id-1", "pod")
	extra := m.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
	extra.Scope().SetName("github.com/example/other")
	extra.Metrics().AppendEmpty().SetName("other.metric")
	extra.Metrics().At(0).SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

	body := `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics/include:
          - name: svc.active
            type: gauge
            unit: "1"
`
	require.NoError(t, AssertMetrics(writeAssertionYAML(t, body), m))

	// The same document without the operators rejects the extra resource.
	exact := `version: 1
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
`
	require.Error(t, AssertMetrics(writeAssertionYAML(t, exact), m))
}

// A resource listed under resources/include with no scopes asserts only that
// the resource is present.
func TestAssertMetrics_ResourcesIncludeWithoutScopes(t *testing.T) {
	require.NoError(t, assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes/include:
      service.name: svc
`))

	err := assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes/include:
      service.name: other
`)
	require.ErrorContains(t, err, "missing expected resource")
}

// An explicit empty collection still asserts that the collection is empty.
func TestAssertMetrics_EmptyExactCollection(t *testing.T) {
	err := AssertMetrics(writeAssertionYAML(t, `version: 1
signal: metrics
resources: []
`), buildSampleMetrics())
	require.ErrorContains(t, err, "unexpected resource")

	require.NoError(t, AssertMetrics(writeAssertionYAML(t, `version: 1
signal: metrics
resources: []
`), pmetric.NewMetrics()))
}
