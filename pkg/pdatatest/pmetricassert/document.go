// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// documentVersion is the schema version emitted by WriteAssertionFile.
// Readers accept this exact value; bumps must be backwards compatible or
// accompanied by a migration.
const documentVersion = 1

// document is the YAML-serializable form of a metrics assertion snapshot.
//
// The schema implements the identity-only subset of the grammar proposed in
// issue #48079: default-exact matching, order-insensitive collections,
// identity fields only. Operator-suffix extensions (/include, /count, ...)
// are tracked as follow-ups.
type document struct {
	Version   int                 `yaml:"version"`
	Signal    string              `yaml:"signal"`
	Resources []resourceAssertion `yaml:"resources"`
}

type resourceAssertion struct {
	Attributes map[string]any   `yaml:"attributes,omitempty"`
	Scopes     []scopeAssertion `yaml:"scopes"`
}

type scopeAssertion struct {
	// Name is always matched exactly: it is the stable instrumentation library
	// identity, so it has no /exists or /regex operator (unlike Version).
	Name    string
	Version versionMatcher
	Metrics []metricAssertion
}

type matcherOp int

const (
	matchExact  matcherOp = iota // `version:` — the only form WriteAssertionFile emits
	matchExists                  // `version/exists: true` — present, any value
	matchRegex                   // `version/regex: <pattern>` — full-string match
)

// versionMatcher matches a scope version. Its zero value is an exact match
// against "", so an omitted `version:` key still pins the empty scope version.
type versionMatcher struct {
	op    matcherOp
	value string
}

// scopeAssertionYAML is the on-disk shape of scopeAssertion: the operator keys
// must be distinct YAML fields because yaml.v3 cannot decode a `version/regex`
// suffix into the versionMatcher struct directly.
type scopeAssertionYAML struct {
	Name          string  `yaml:"name,omitempty"`
	Version       *string `yaml:"version,omitempty"`
	VersionExists *bool   `yaml:"version/exists,omitempty"`
	VersionRegex  *string `yaml:"version/regex,omitempty"`

	Metrics []metricAssertion `yaml:"metrics"`
}

// UnmarshalYAML decodes a scope, resolving the version operator keys into a versionMatcher.
func (s *scopeAssertion) UnmarshalYAML(value *yaml.Node) error {
	var raw scopeAssertionYAML
	if err := value.Decode(&raw); err != nil {
		return err
	}

	version, err := buildVersionMatcher(raw.Version, raw.VersionExists, raw.VersionRegex)
	if err != nil {
		return err
	}

	s.Name = raw.Name
	s.Version = version
	s.Metrics = raw.Metrics
	return nil
}

func buildVersionMatcher(exact *string, exists *bool, regex *string) (versionMatcher, error) {
	set := 0
	if exact != nil {
		set++
	}
	if exists != nil {
		set++
	}
	if regex != nil {
		set++
	}
	if set > 1 {
		return versionMatcher{}, errors.New(`scope must use at most one of "version", "version/exists", "version/regex"`)
	}

	switch {
	case exists != nil:
		if !*exists {
			return versionMatcher{}, errors.New("scope version/exists must be true (the only supported value)")
		}
		return versionMatcher{op: matchExists}, nil
	case regex != nil:
		if _, err := regexp.Compile("^(?:" + *regex + ")$"); err != nil {
			return versionMatcher{}, fmt.Errorf("scope version/regex has invalid pattern %q: %w", *regex, err)
		}
		return versionMatcher{op: matchRegex, value: *regex}, nil
	case exact != nil:
		return versionMatcher{op: matchExact, value: *exact}, nil
	default:
		return versionMatcher{op: matchExact}, nil
	}
}

// MarshalYAML encodes a scope, rendering an exact version as a plain `version:`
// scalar (not a suffixed key) so WriteAssertionFile output matches the pre-operator layout.
func (s scopeAssertion) MarshalYAML() (any, error) {
	out := scopeAssertionYAML{Name: s.Name, Metrics: s.Metrics}

	switch s.Version.op {
	case matchExists:
		t := true
		out.VersionExists = &t
	case matchRegex:
		v := s.Version.value
		out.VersionRegex = &v
	case matchExact:
		if s.Version.value != "" {
			v := s.Version.value
			out.Version = &v
		}
	}

	return out, nil
}

type metricAssertion struct {
	Name        string               `yaml:"name"`
	Type        string               `yaml:"type"`
	Unit        string               `yaml:"unit,omitempty"`
	Temporality string               `yaml:"temporality,omitempty"`
	Monotonic   *bool                `yaml:"monotonic,omitempty"`
	Datapoints  []datapointAssertion `yaml:"datapoints,omitempty"`
}

type datapointAssertion struct {
	Attributes     map[string]any `yaml:"attributes,omitempty"`
	Value          any            `yaml:"value,omitempty"`
	Count          *uint64        `yaml:"count,omitempty"`
	Sum            *float64       `yaml:"sum,omitempty"`
	ExplicitBounds []float64      `yaml:"explicit_bounds,omitempty"`
	BucketCounts   []uint64       `yaml:"bucket_counts,omitempty"`
	Min            *float64       `yaml:"min,omitempty"`
	Max            *float64       `yaml:"max,omitempty"`
}

func readDocument(path string) (*document, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read assertion file: %w", err)
	}
	var doc document
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse assertion file %s: %w", path, err)
	}
	if doc.Version != documentVersion {
		return nil, fmt.Errorf("assertion file %s: unsupported version %d (want %d)",
			path, doc.Version, documentVersion)
	}
	if doc.Signal != "metrics" {
		return nil, fmt.Errorf("assertion file %s: unsupported signal %q (want %q)",
			path, doc.Signal, "metrics")
	}
	expandShorthand(&doc)
	return &doc, nil
}

// expandShorthand fills in the implicit single-datapoint-with-no-attributes
// case. An omitted or empty `datapoints:` means "one datapoint with no
// attributes", which is the most common shape for single-series metrics.
//
// This relies on the invariant enforced by ValidateMetrics (#48106) that a
// Metric must have at least one datapoint. [AssertMetrics] expands shorthand
// before comparison, so pmetricassert does not accept the empty-metric
// encoding as a valid assertion either.
func expandShorthand(doc *document) {
	for i := range doc.Resources {
		for j := range doc.Resources[i].Scopes {
			for k := range doc.Resources[i].Scopes[j].Metrics {
				m := &doc.Resources[i].Scopes[j].Metrics[k]
				if len(m.Datapoints) == 0 {
					m.Datapoints = []datapointAssertion{{}}
				}
			}
		}
	}
}

func writeDocument(path string, doc *document) error {
	compactShorthand(doc)
	b, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// compactShorthand is the inverse of expandShorthand: it drops an explicit
// single empty-attribute datapoint so the emitted YAML reads as "metric with
// no dimensioning attributes" rather than "metric with one empty datapoint".
func compactShorthand(doc *document) {
	for i := range doc.Resources {
		for j := range doc.Resources[i].Scopes {
			for k := range doc.Resources[i].Scopes[j].Metrics {
				m := &doc.Resources[i].Scopes[j].Metrics[k]
				if len(m.Datapoints) == 1 && len(m.Datapoints[0].Attributes) == 0 && m.Datapoints[0].Value == nil {
					m.Datapoints = nil
				}
			}
		}
	}
}
