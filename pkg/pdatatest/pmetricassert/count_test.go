// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// buildSampleMetrics has one resource, one scope, two metrics, and two
// datapoints on svc.requests, so every level has a size worth asserting.

// A count-only collection asserts cardinality and nothing about identity, so
// the resources it does not list are not reported as unexpected.
func TestAssertMetrics_CountOnlyIgnoresIdentity(t *testing.T) {
	require.NoError(t, assertSample(t, `version: 1
signal: metrics
resources/count:
  min: 1
`))

	err := assertSample(t, `version: 1
signal: metrics
resources/count:
  min: 2
`)
	require.ErrorContains(t, err, "resources: expected at least 2, got 1")
}

// Every bound form, asserted against a two-resource payload.
func TestAssertMetrics_CountBounds(t *testing.T) {
	tests := map[string]struct {
		count   string
		wantErr string
	}{
		"exact": {count: "resources/count:\n  exact: 2"},
		"exact wrong": {
			count:   "resources/count:\n  exact: 3",
			wantErr: "resources: expected 3, got 2",
		},
		"min only": {count: "resources/count:\n  min: 1"},
		"min too high": {
			count:   "resources/count:\n  min: 5",
			wantErr: "resources: expected at least 5, got 2",
		},
		"max only": {count: "resources/count:\n  max: 4"},
		"max too low": {
			count:   "resources/count:\n  max: 1",
			wantErr: "resources: expected at most 1, got 2",
		},
		"between": {count: "resources/count:\n  min: 1\n  max: 4"},
		"below range": {
			count:   "resources/count:\n  min: 3\n  max: 4",
			wantErr: "resources: expected between 3 and 4, got 2",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			m := buildSampleMetrics()
			appendResourceWithKindAndID(m, "id-1", "pod")

			path := writeAssertionYAML(t, "version: 1\nsignal: metrics\n"+tt.count+"\n")
			err := AssertMetrics(path, m)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// A metric that asserts only its datapoint count must not pick up the implicit
// single empty-attribute datapoint, which would pin the count to 1.
func TestAssertMetrics_DatapointsCountSkipsShorthand(t *testing.T) {
	require.NoError(t, assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes/include:
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
            datapoints/count:
              min: 2
`))

	err := assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes/include:
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
            datapoints/count:
              exact: 3
`)
	require.ErrorContains(t, err, "datapoints: expected 3, got 2")
}

// /count composes with /include: the listed items must be present and the
// collection must also have the asserted size.
func TestAssertMetrics_CountWithInclude(t *testing.T) {
	body := func(count string) string {
		return `version: 1
signal: metrics
resources/include:
  - attributes/include:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics/include:
          - name: svc.active
            type: gauge
            unit: "1"
        metrics/count:
          ` + count + "\n"
	}

	require.NoError(t, assertSample(t, body("min: 2")))

	// The include still holds, but the size does not.
	err := assertSample(t, body("min: 3"))
	require.ErrorContains(t, err, "metrics: expected at least 3, got 2")

	// The size holds, but the include does not.
	err = assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes/include:
      service.name: svc
    scopes/include:
      - name: github.com/example/receiver
        version/exists: true
        metrics/include:
          - name: svc.absent
            type: gauge
        metrics/count:
          min: 2
`)
	require.ErrorContains(t, err, `missing expected metric "svc.absent"`)
}

func TestAssertMetrics_ScopesCount(t *testing.T) {
	require.NoError(t, assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes/include:
      service.name: svc
    scopes/count:
      exact: 1
`))

	// A counted collection inside an exact parent still asserts only its size:
	// the scope this document never names is not reported as unexpected.
	require.NoError(t, assertSample(t, `version: 1
signal: metrics
resources:
  - attributes:
      service.name: svc
    scopes/count:
      exact: 1
`))

	err := assertSample(t, `version: 1
signal: metrics
resources/include:
  - attributes/include:
      service.name: svc
    scopes/count:
      exact: 2
`)
	require.ErrorContains(t, err, "scopes: expected 2, got 1")
}

func TestReadDocument_CountSchemaErrors(t *testing.T) {
	tests := map[string]struct {
		body    string
		wantErr string
	}{
		"exact list and count": {
			body: `version: 1
signal: metrics
resources: []
resources/count:
  exact: 1
`,
			wantErr: `cannot specify both "resources" and "resources/count"`,
		},
		"no bounds": {
			body: `version: 1
signal: metrics
resources/count: {}
`,
			wantErr: `resources/count must set one of "exact", "min" or "max"`,
		},
		"unknown key": {
			body: `version: 1
signal: metrics
resources/count:
  atLeast: 2
  upTo: 3
`,
			wantErr: `resources/count has unknown keys [atLeast upTo], want "exact", "min" or "max"`,
		},
		"exact with min": {
			body: `version: 1
signal: metrics
resources/count:
  exact: 1
  min: 2
`,
			wantErr: `resources/count cannot combine "exact" with "min" or "max"`,
		},
		"min above max": {
			body: `version: 1
signal: metrics
resources/count:
  min: 5
  max: 2
`,
			wantErr: "resources/count min 5 is greater than max 2",
		},
		"negative exact": {
			body: `version: 1
signal: metrics
resources/count:
  exact: -1
`,
			wantErr: "resources/count exact must not be negative, got -1",
		},
		"bare scalar": {
			body: `version: 1
signal: metrics
resources/count: 3
`,
			wantErr: `resources/count must be a mapping, write "exact: 3" for an exact size`,
		},
		// A null scalar decodes into an int as zero, so an empty bound would
		// otherwise be accepted as a constraint that asserts nothing.
		"empty min": {
			body: `version: 1
signal: metrics
resources/count:
  min:
`,
			wantErr: "resources/count min has no value",
		},
		"null exact": {
			body: `version: 1
signal: metrics
resources/count:
  exact: null
`,
			wantErr: "resources/count exact has no value",
		},
		"empty count": {
			body: `version: 1
signal: metrics
resources/count:
`,
			wantErr: `resources/count has no value, want a mapping with "exact", "min" or "max"`,
		},
		"negative min": {
			body: `version: 1
signal: metrics
resources/count:
  min: -3
`,
			wantErr: "resources/count min must not be negative, got -3",
		},
		"nested count on exact list": {
			body: `version: 1
signal: metrics
resources:
  - attributes: {}
    scopes: []
    scopes/count:
      exact: 1
`,
			wantErr: `cannot specify both "scopes" and "scopes/count"`,
		},
		"datapoints count on exact list": {
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
            datapoints/count:
              exact: 1
`,
			wantErr: `cannot specify both "datapoints" and "datapoints/count"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := readDocument(writeAssertionYAML(t, tt.body))
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// WriteAssertionFile never emits /count, so a round trip through the writer
// still produces the default exact form.
func TestWriteDocument_OmitsCount(t *testing.T) {
	path := writeAssertionYAML(t, `version: 1
signal: metrics
resources/count:
  min: 1
`)
	doc, err := readDocument(path)
	require.NoError(t, err)
	require.NotNil(t, doc.ResourcesCount)

	out := writeAssertionYAML(t, "")
	require.NoError(t, writeDocument(out, doc))

	roundTripped, err := readDocument(out)
	require.NoError(t, err)
	require.Nil(t, roundTripped.ResourcesCount)
}
