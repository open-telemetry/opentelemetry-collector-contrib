// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"fmt"
	"os"

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
	Version   int                `yaml:"version"`
	Signal    string             `yaml:"signal"`
	Resources ResourcesAssertion `yaml:",inline"`
}

type resourceAssertion struct {
	Attributes map[string]any  `yaml:"attributes,omitempty"`
	Scopes     ScopesAssertion `yaml:",inline"`
}

type scopeAssertion struct {
	Name    string           `yaml:"name,omitempty"`
	Version string           `yaml:"version,omitempty"`
	Metrics MetricsAssertion `yaml:",inline"`
}

type metricAssertion struct {
	Name        string              `yaml:"name"`
	Type        string              `yaml:"type"`
	Unit        string              `yaml:"unit,omitempty"`
	Temporality string              `yaml:"temporality,omitempty"`
	Monotonic   *bool               `yaml:"monotonic,omitempty"`
	Datapoints  DatapointsAssertion `yaml:",inline"`
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

// --- MarshalYAML Methods ---

func (d document) MarshalYAML() (any, error) {
	out := map[string]any{
		"version": d.Version,
		"signal":  d.Signal,
	}
	if len(d.Resources.Exact) > 0 {
		out["resources"] = d.Resources.Exact
	}
	if len(d.Resources.Include) > 0 {
		out["resources/include"] = d.Resources.Include
	}
	return out, nil
}

func (r resourceAssertion) MarshalYAML() (any, error) {
	out := map[string]any{}
	if r.Attributes != nil {
		out["attributes"] = r.Attributes
	}
	if len(r.Scopes.Exact) > 0 {
		out["scopes"] = r.Scopes.Exact
	}
	if len(r.Scopes.Include) > 0 {
		out["scopes/include"] = r.Scopes.Include
	}
	return out, nil
}

func (s scopeAssertion) MarshalYAML() (any, error) {
	out := map[string]any{}
	if s.Name != "" {
		out["name"] = s.Name
	}
	if s.Version != "" {
		out["version"] = s.Version
	}
	if len(s.Metrics.Exact) > 0 {
		out["metrics"] = s.Metrics.Exact
	}
	if len(s.Metrics.Include) > 0 {
		out["metrics/include"] = s.Metrics.Include
	}
	return out, nil
}

func (m metricAssertion) MarshalYAML() (any, error) {
	out := map[string]any{
		"name": m.Name,
		"type": m.Type,
	}
	if m.Unit != "" {
		out["unit"] = m.Unit
	}
	if m.Temporality != "" {
		out["temporality"] = m.Temporality
	}
	if m.Monotonic != nil {
		out["monotonic"] = m.Monotonic
	}
	if len(m.Datapoints.Exact) > 0 {
		out["datapoints"] = m.Datapoints.Exact
	}
	if len(m.Datapoints.Include) > 0 {
		out["datapoints/include"] = m.Datapoints.Include
	}
	return out, nil
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
	for i := range doc.Resources.Exact {
		expandResourceShorthand(&doc.Resources.Exact[i])
	}
	for i := range doc.Resources.Include {
		expandResourceShorthand(&doc.Resources.Include[i])
	}
}

func expandResourceShorthand(r *resourceAssertion) {
	for i := range r.Scopes.Exact {
		expandScopeShorthand(&r.Scopes.Exact[i])
	}
	for i := range r.Scopes.Include {
		expandScopeShorthand(&r.Scopes.Include[i])
	}
}

func expandScopeShorthand(s *scopeAssertion) {
	for i := range s.Metrics.Exact {
		expandMetricShorthand(&s.Metrics.Exact[i])
	}
	for i := range s.Metrics.Include {
		expandMetricShorthand(&s.Metrics.Include[i])
	}
}

func expandMetricShorthand(m *metricAssertion) {
	if len(m.Datapoints.Exact) == 0 && len(m.Datapoints.Include) == 0 {
		m.Datapoints.Exact = []datapointAssertion{{}}
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
	for i := range doc.Resources.Exact {
		compactResourceShorthand(&doc.Resources.Exact[i])
	}
	for i := range doc.Resources.Include {
		compactResourceShorthand(&doc.Resources.Include[i])
	}
	if len(doc.Resources.Include) == 0 {
		doc.Resources = ResourcesAssertion{Exact: doc.Resources.Exact}
	}
}

func compactResourceShorthand(r *resourceAssertion) {
	for i := range r.Scopes.Exact {
		compactScopeShorthand(&r.Scopes.Exact[i])
	}
	for i := range r.Scopes.Include {
		compactScopeShorthand(&r.Scopes.Include[i])
	}
	if len(r.Scopes.Include) == 0 {
		r.Scopes = ScopesAssertion{Exact: r.Scopes.Exact}
	}
}

func compactScopeShorthand(s *scopeAssertion) {
	for i := range s.Metrics.Exact {
		compactMetricShorthand(&s.Metrics.Exact[i])
	}
	for i := range s.Metrics.Include {
		compactMetricShorthand(&s.Metrics.Include[i])
	}
	if len(s.Metrics.Include) == 0 {
		s.Metrics = MetricsAssertion{Exact: s.Metrics.Exact}
	}
}

func compactMetricShorthand(m *metricAssertion) {
	if len(m.Datapoints.Exact) == 1 && len(m.Datapoints.Exact[0].Attributes) == 0 && m.Datapoints.Exact[0].Value == nil && len(m.Datapoints.Include) == 0 {
		m.Datapoints = DatapointsAssertion{}
	}
}
