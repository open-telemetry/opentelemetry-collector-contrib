// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

// Every key WriteAssertionFile emits must be accepted when the file is read.
func TestWriteAssertionFile_ReadsBackWithKnownKeys(t *testing.T) {
	for name, m := range map[string]pmetric.Metrics{
		"sample":    buildSampleMetrics(),
		"histogram": buildHistogramMetrics(),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
			require.NoError(t, WriteAssertionFile(t, path, m, IncludeValues()))
			require.NoError(t, AssertMetrics(path, m))
		})
	}
}

func TestReadDocument_UnknownKeys(t *testing.T) {
	tests := map[string]struct {
		body    string
		wantErr string
	}{
		"document": {
			body: `version: 1
signal: metrics
resource: []
`,
			wantErr: `unknown keys ["resource"], want ["version" "signal" "resources" "resources/include" "resources/count"]`,
		},
		"document operator suffix": {
			body: `version: 1
signal: metrics
resources/includes: []
`,
			wantErr: `unknown keys ["resources/includes"]`,
		},
		"resource": {
			body: `version: 1
signal: metrics
resources:
  - attributes: {}
    scopes/cuont:
      exact: 1
`,
			wantErr: `resource assertion: unknown keys ["scopes/cuont"]`,
		},
		"scope count": {
			body: `version: 1
signal: metrics
resources:
  - scopes:
      - name: scope-a
        metrics/include: []
        metrics/cuont:
          exact: 5
`,
			wantErr: `scope assertion: unknown keys ["metrics/cuont"]`,
		},
		"scope include": {
			body: `version: 1
signal: metrics
resources/include:
  - scopes/include:
      - name: scope-a
        metrics/includes:
          - name: svc.active
            type: gauge
`,
			wantErr: `scope assertion: unknown keys ["metrics/includes"]`,
		},
		"scope version operator": {
			body: `version: 1
signal: metrics
resources:
  - scopes:
      - name: scope-a
        version/exist: true
`,
			wantErr: `scope assertion: unknown keys ["version/exist"]`,
		},
		"metric": {
			body: `version: 1
signal: metrics
resources:
  - scopes:
      - name: scope-a
        metrics:
          - name: svc.active
            type: gauge
            unit/regex: "1"
`,
			wantErr: `metric "svc.active": unknown keys ["unit/regex"]`,
		},
		"datapoint": {
			body: `version: 1
signal: metrics
resources:
  - scopes:
      - name: scope-a
        metrics:
          - name: svc.active
            type: gauge
            datapoints:
              - value: 1
`,
			wantErr: `datapoint assertion: unknown keys ["value"]`,
		},
		"datapoint operator suffix": {
			body: `version: 1
signal: metrics
resources:
  - scopes:
      - name: scope-a
        metrics:
          - name: svc.active
            type: gauge
            datapoints:
              - double_value/precison3: 1.5
`,
			wantErr: `datapoint assertion: unknown keys ["double_value/precison3"]`,
		},
		"datapoint under include": {
			body: `version: 1
signal: metrics
resources/include:
  - scopes/include:
      - name: scope-a
        metrics/include:
          - name: svc.active
            type: gauge
            datapoints/include:
              - attributes/includes:
                  method: GET
`,
			wantErr: `datapoint assertion: unknown keys ["attributes/includes"]`,
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

func TestReadDocument_AttributeOperatorSchemaErrors(t *testing.T) {
	tests := map[string]struct {
		attrs   []string
		wantErr string
	}{
		"exists false": {
			attrs:   []string{`foo/exists: false`},
			wantErr: `attribute "foo"/exists must be true (the only supported value)`,
		},
		"exists string": {
			attrs:   []string{`foo/exists: "true"`},
			wantErr: `attribute "foo"/exists must be true (the only supported value)`,
		},
		"regex not a string": {
			attrs:   []string{`foo/regex: 42`},
			wantErr: `attribute "foo"/regex must be a string pattern`,
		},
		"regex invalid pattern": {
			attrs:   []string{`foo/regex: "["`},
			wantErr: `attribute "foo"/regex has invalid pattern "["`,
		},
		"literal and regex": {
			attrs:   []string{`foo: bar`, `foo/regex: b.*`},
			wantErr: `attribute "foo": cannot specify both "foo" and "foo/regex"`,
		},
		"literal and exists": {
			attrs:   []string{`foo: bar`, `foo/exists: true`},
			wantErr: `attribute "foo": cannot specify both "foo" and "foo/exists"`,
		},
		"exists and regex": {
			attrs:   []string{`foo/exists: true`, `foo/regex: b.*`},
			wantErr: `attribute "foo": cannot specify both "foo/exists" and "foo/regex"`,
		},
	}

	// Every case is checked in each attribute map a document can hold.
	placements := map[string]struct {
		body   func(key string, attrs []string) string
		prefix string
	}{
		"resource": {
			body: func(key string, attrs []string) string {
				return "version: 1\nsignal: metrics\nresources:\n  - " + key + ":\n      " +
					strings.Join(attrs, "\n      ") + "\n"
			},
			prefix: "resource assertion: ",
		},
		"datapoint": {
			body: func(key string, attrs []string) string {
				return "version: 1\nsignal: metrics\nresources:\n  - scopes:\n      - name: scope-a\n" +
					"        metrics:\n          - name: svc.active\n            type: gauge\n" +
					"            datapoints:\n              - " + key + ":\n                  " +
					strings.Join(attrs, "\n                  ") + "\n"
			},
			prefix: "datapoint assertion: ",
		},
	}

	for name, tt := range tests {
		for placement, p := range placements {
			for _, key := range []string{"attributes", "attributes/include"} {
				t.Run(name+"/"+placement+"/"+key, func(t *testing.T) {
					_, err := readDocument(writeAssertionYAML(t, p.body(key, tt.attrs)))
					require.Error(t, err)
					require.Contains(t, err.Error(), p.prefix+tt.wantErr)
				})
			}
		}
	}
}

// Attribute keys may contain `/`, so only the /exists and /regex suffixes are
// operators and a key with any other suffix is a literal attribute key.
func TestReadDocument_UnknownAttributeSuffixIsLiteral(t *testing.T) {
	path := writeAssertionYAML(t, `version: 1
signal: metrics
resources:
  - attributes:
      k8s/thing: node-1
    scopes:
      - name: scope-a
        metrics:
          - name: svc.requests
            type: gauge
            datapoints:
              - attributes:
                  method/gtee: GET
`)

	doc, err := readDocument(path)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"k8s/thing": "node-1"}, doc.Resources[0].Attributes)
	require.Equal(t, map[string]any{"method/gtee": "GET"}, doc.Resources[0].Scopes[0].Metrics[0].Datapoints[0].Attributes)

	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k8s/thing", "node-1")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("scope-a")
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("svc.requests")
	metric.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutStr("method/gtee", "GET")
	require.NoError(t, AssertMetrics(path, m))

	rm.Resource().Attributes().PutStr("k8s/thing", "node-2")
	require.ErrorContains(t, AssertMetrics(path, m), "missing expected resource")
}
