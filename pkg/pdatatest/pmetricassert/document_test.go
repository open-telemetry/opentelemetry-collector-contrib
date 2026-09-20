// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeAssertionYAML(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestReadDocument_CollectionIncludeSuffixes(t *testing.T) {
	path := writeAssertionYAML(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: scope-a
        metrics/include:
          - name: svc.requests
            type: sum
            unit: "{requests}"
            temporality: cumulative
            monotonic: true
            datapoints/include:
              - attributes:
                  method: GET
`)

	doc, err := readDocument(path)
	require.NoError(t, err)

	require.Equal(t, collectionModeInclude, doc.ResourcesMode)
	require.Len(t, doc.Resources, 1)

	res := doc.Resources[0]
	require.Equal(t, collectionModeInclude, res.ScopesMode)
	require.Len(t, res.Scopes, 1)

	scope := res.Scopes[0]
	require.Equal(t, collectionModeInclude, scope.MetricsMode)
	require.Len(t, scope.Metrics, 1)

	metric := scope.Metrics[0]
	require.Equal(t, collectionModeInclude, metric.DatapointsMode)
	require.Len(t, metric.Datapoints, 1)
	require.Equal(t, "GET", metric.Datapoints[0].Attributes["method"])
}

func TestReadDocument_DefaultCollectionModeIsExact(t *testing.T) {
	path := writeAssertionYAML(t, `version: 1
signal: metrics
resources:
  - attributes:
      service.name: svc
    scopes:
      - name: scope-a
        metrics:
          - name: svc.active
            type: gauge
            unit: "1"
`)

	doc, err := readDocument(path)
	require.NoError(t, err)

	require.Equal(t, collectionModeExact, doc.ResourcesMode)
	require.Equal(t, collectionModeExact, doc.Resources[0].ScopesMode)
	require.Equal(t, collectionModeExact, doc.Resources[0].Scopes[0].MetricsMode)

	metric := doc.Resources[0].Scopes[0].Metrics[0]
	require.Equal(t, collectionModeExact, metric.DatapointsMode)
	// The single empty-attribute datapoint shorthand still applies in an
	// exact collection.
	require.Equal(t, []datapointAssertion{{}}, metric.Datapoints)
}

// An /include item that omits a nested collection asserts nothing about it,
// rather than asserting that it is empty.
func TestReadDocument_OmittedNestedCollectionUnderInclude(t *testing.T) {
	path := writeAssertionYAML(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
`)

	doc, err := readDocument(path)
	require.NoError(t, err)

	res := doc.Resources[0]
	require.Equal(t, collectionModeInclude, res.ScopesMode)
	require.Empty(t, res.Scopes)
}

func TestReadDocument_OmittedDatapointsUnderMetricsInclude(t *testing.T) {
	path := writeAssertionYAML(t, `version: 1
signal: metrics
resources/include:
  - attributes:
      service.name: svc
    scopes/include:
      - name: scope-a
        metrics/include:
          - name: svc.requests
            type: sum
`)

	doc, err := readDocument(path)
	require.NoError(t, err)

	metric := doc.Resources[0].Scopes[0].Metrics[0]
	require.Equal(t, collectionModeInclude, metric.DatapointsMode)
	require.Empty(t, metric.Datapoints, "no implicit datapoint may be injected under /include")
}

func TestReadDocument_CollectionOperatorConflicts(t *testing.T) {
	tests := map[string]struct {
		body    string
		wantErr string
	}{
		"resources": {
			body: `version: 1
signal: metrics
resources: []
resources/include: []
`,
			wantErr: `cannot specify both "resources" and "resources/include"`,
		},
		"scopes": {
			body: `version: 1
signal: metrics
resources:
  - attributes: {}
    scopes: []
    scopes/include: []
`,
			wantErr: `cannot specify both "scopes" and "scopes/include"`,
		},
		"metrics": {
			body: `version: 1
signal: metrics
resources:
  - attributes: {}
    scopes:
      - name: scope-a
        metrics: []
        metrics/include: []
`,
			wantErr: `cannot specify both "metrics" and "metrics/include"`,
		},
		"datapoints": {
			body: `version: 1
signal: metrics
resources:
  - attributes: {}
    scopes:
      - name: scope-a
        metrics:
          - name: svc.active
            type: gauge
            datapoints: []
            datapoints/include: []
`,
			wantErr: `cannot specify both "datapoints" and "datapoints/include"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := readDocument(writeAssertionYAML(t, tt.body))
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// WriteAssertionFile only ever emits the default exact form, so a written file
// never contains an operator suffix.
func TestWriteDocument_EmitsDefaultExactCollections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, buildSampleMetrics()))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	require.Contains(t, string(raw), "resources:")
	require.Contains(t, string(raw), "scopes:")
	require.Contains(t, string(raw), "metrics:")
	require.NotContains(t, string(raw), "/include")
}
