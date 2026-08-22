// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadDocument_CollectionOperatorSuffixes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
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
`), 0o600))

	doc, err := readDocument(path)
	require.NoError(t, err)
	require.Len(t, doc.Resources.Includes, 1)

	res := doc.Resources.Includes[0]
	require.Len(t, res.Scopes.Includes, 1)
	scope := res.Scopes.Includes[0]
	require.Len(t, scope.Metrics.Includes, 1)
	metric := scope.Metrics.Includes[0]
	require.Len(t, metric.Datapoints.Includes, 1)
	require.Equal(t, "GET", metric.Datapoints.Includes[0].Attributes["method"])
}

func TestWriteDocument_DefaultCollectionsMarshalAsExact(t *testing.T) {
	doc := &document{
		Version: documentVersion,
		Signal:  "metrics",
		Resources: ResourcesAssertion{Values: []resourceAssertion{
			{
				Attributes: map[string]any{"service.name": "svc"},
				Scopes: ScopesAssertion{Values: []scopeAssertion{
					{
						Name: "scope",
						Metrics: MetricsAssertion{Values: []metricAssertion{
							{Name: "svc.active", Type: "gauge", Unit: "1", Datapoints: DatapointsAssertion{Values: []datapointAssertion{{}}}},
						}},
					},
				}},
			},
		}},
	}

	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, writeDocument(path, doc))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(raw), "resources:")
	require.Contains(t, string(raw), "scopes:")
	require.Contains(t, string(raw), "metrics:")
	require.NotContains(t, string(raw), "resources/exact")
	require.NotContains(t, string(raw), "scopes/exact")
	require.NotContains(t, string(raw), "metrics/exact")
}
