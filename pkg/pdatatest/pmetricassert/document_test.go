// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	require.Equal(t, collectionModeInclude, doc.Resources.mode)
	require.Len(t, doc.Resources.items, 1)

	res := doc.Resources.items[0]
	require.Equal(t, collectionModeInclude, res.Scopes.mode)
	require.Len(t, res.Scopes.items, 1)

	scope := res.Scopes.items[0]
	require.Equal(t, collectionModeInclude, scope.Metrics.mode)
	require.Len(t, scope.Metrics.items, 1)

	metric := scope.Metrics.items[0]
	require.Equal(t, collectionModeInclude, metric.Datapoints.mode)
	require.Len(t, metric.Datapoints.items, 1)
	require.Equal(t, "GET", metric.Datapoints.items[0].Attributes["method"])
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

	require.Equal(t, collectionModeExact, doc.Resources.mode)
	require.Equal(t, collectionModeExact, doc.Resources.items[0].Scopes.mode)
	require.Equal(t, collectionModeExact, doc.Resources.items[0].Scopes.items[0].Metrics.mode)

	metric := doc.Resources.items[0].Scopes.items[0].Metrics.items[0]
	require.Equal(t, collectionModeExact, metric.Datapoints.mode)
	// The single empty-attribute datapoint shorthand still applies in an
	// exact collection.
	require.Equal(t, []datapointAssertion{{}}, metric.Datapoints.items)
}

// `<collection>/exact` is an alias of `<collection>` at every level.
func TestReadDocument_CollectionExactSuffixes(t *testing.T) {
	path := writeAssertionYAML(t, `version: 1
signal: metrics
resources/exact:
  - attributes:
      service.name: svc
    scopes/exact:
      - name: scope-a
        metrics/exact:
          - name: svc.requests
            type: sum
            datapoints/exact:
              - attributes:
                  method: GET
`)

	doc, err := readDocument(path)
	require.NoError(t, err)

	require.Equal(t, collectionModeExact, doc.Resources.mode)
	require.Len(t, doc.Resources.items, 1)

	res := doc.Resources.items[0]
	require.Equal(t, collectionModeExact, res.Scopes.mode)
	require.Len(t, res.Scopes.items, 1)

	scope := res.Scopes.items[0]
	require.Equal(t, collectionModeExact, scope.Metrics.mode)
	require.Len(t, scope.Metrics.items, 1)

	metric := scope.Metrics.items[0]
	require.Equal(t, collectionModeExact, metric.Datapoints.mode)
	require.Len(t, metric.Datapoints.items, 1)
	require.Equal(t, "GET", metric.Datapoints.items[0].Attributes["method"])
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

	res := doc.Resources.items[0]
	require.Equal(t, collectionModeInclude, res.Scopes.mode)
	require.Empty(t, res.Scopes.items)
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

	metric := doc.Resources.items[0].Scopes.items[0].Metrics.items[0]
	require.Equal(t, collectionModeInclude, metric.Datapoints.mode)
	require.Empty(t, metric.Datapoints.items, "no implicit datapoint may be injected under /include")
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

func TestReadDocument_CollectionExactConflicts(t *testing.T) {
	// nest places keys under the collection named by level, with every outer
	// collection holding a single item.
	nest := func(level, keys string) string {
		body := "version: 1\nsignal: metrics\n"
		indent := ""
		for _, outer := range []struct{ name, item string }{
			{"resources", "attributes: {}"},
			{"scopes", "name: scope-a"},
			{"metrics", "name: svc.active\n  type: gauge"},
		} {
			if outer.name == level {
				break
			}
			body += indent + outer.name + ":\n" + indent + "  - " + strings.ReplaceAll(outer.item, "\n", "\n"+indent+"  ") + "\n"
			indent += "    "
		}
		return body + indent + strings.ReplaceAll(keys, "\n", "\n"+indent) + "\n"
	}

	for _, key := range []string{"resources", "scopes", "metrics", "datapoints"} {
		tests := map[string]struct {
			keys    string
			wantErr string
		}{
			"plain and exact": {
				keys:    key + ": []\n" + key + "/exact: []",
				wantErr: fmt.Sprintf("cannot specify both %q and %q", key, key+"/exact"),
			},
			"exact and include": {
				keys:    key + "/exact: []\n" + key + "/include: []",
				wantErr: fmt.Sprintf("cannot specify both %q and %q", key+"/exact", key+"/include"),
			},
			"exact and count": {
				keys:    key + "/exact: []\n" + key + "/count:\n  exact: 1",
				wantErr: fmt.Sprintf("cannot specify both %q and %q, an exact collection already fixes its size", key+"/exact", key+"/count"),
			},
		}
		for name, tt := range tests {
			t.Run(key+"/"+name, func(t *testing.T) {
				_, err := readDocument(writeAssertionYAML(t, nest(key, tt.keys)))
				require.ErrorContains(t, err, tt.wantErr)
			})
		}
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
