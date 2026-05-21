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
	yamlContent := "version: 1\n" +
		"signal: metrics\n" +
		"resources/count:\n" +
		"  exact: 1\n" +
		"resources/exact:\n" +
		"  - attributes:\n" +
		"      service.name: svc\n" +
		"    scopes/exact:\n" +
		"      - name: scope-a\n" +
		"        metrics/exact:\n" +
		"          - name: svc.requests\n" +
		"            type: sum\n" +
		"            unit: \"{requests}\"\n" +
		"            temporality: cumulative\n" +
		"            monotonic: true\n" +
		"            datapoints/exact:\n" +
		"              - attributes:\n" +
		"                  method: GET\n"
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0o600))

	doc, err := readDocument(path)
	require.NoError(t, err)
	require.NotNil(t, doc.Resources.Count)
	require.NotNil(t, doc.Resources.Count.Exact)
	require.Equal(t, 1, *doc.Resources.Count.Exact)
	require.Len(t, doc.Resources.Exact, 1)

	res := doc.Resources.Exact[0]
	require.Len(t, res.Scopes.Exact, 1)
	scope := res.Scopes.Exact[0]
	require.Len(t, scope.Metrics.Exact, 1)
	metric := scope.Metrics.Exact[0]
	require.Len(t, metric.Datapoints.Exact, 1)
	require.Equal(t, "GET", metric.Datapoints.Exact[0].Attributes["method"])
}

func TestWriteDocument_DefaultCollectionsMarshalAsExact(t *testing.T) {
	doc := &document{
		Version: documentVersion,
		Signal:  "metrics",
		Resources: ResourcesAssertion{Exact: []resourceAssertion{
			{
				Attributes: map[string]any{"service.name": "svc"},
				Scopes: ScopesAssertion{Exact: []scopeAssertion{
					{
						Name: "scope",
						Metrics: MetricsAssertion{Exact: []metricAssertion{
							{Name: "svc.active", Type: "gauge", Unit: "1", Datapoints: DatapointsAssertion{Exact: []datapointAssertion{{}}}},
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
