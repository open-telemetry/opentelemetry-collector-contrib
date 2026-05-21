// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"fmt"
	"os"
	"strings"

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
// identity fields only. Operator-suffix extensions (/include, /regex,
// /count, ...) are tracked as follow-ups.
type document struct {
	Version   int                `yaml:"version"`
	Signal    string             `yaml:"signal"`
	Resources ResourcesAssertion `yaml:"resources"`
}

type resourceAssertion struct {
	Attributes map[string]any  `yaml:"attributes,omitempty"`
	Scopes     ScopesAssertion `yaml:"scopes"`
}

type scopeAssertion struct {
	Name    string           `yaml:"name,omitempty"`
	Version string           `yaml:"version,omitempty"`
	Metrics MetricsAssertion `yaml:"metrics"`
}

type metricAssertion struct {
	Name        string              `yaml:"name"`
	Type        string              `yaml:"type"`
	Unit        string              `yaml:"unit,omitempty"`
	Temporality string              `yaml:"temporality,omitempty"`
	Monotonic   *bool               `yaml:"monotonic,omitempty"`
	Datapoints  DatapointsAssertion `yaml:"datapoints,omitempty"`
}

type datapointAssertion struct {
	Attributes map[string]any `yaml:"attributes,omitempty"`
}

func (d *document) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("document must be a map, got YAML kind %d", value.Kind)
	}

	resourcesNode, err := extractCollectionNodeFromParent(value, "resources")
	if err != nil {
		return err
	}

	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i].Value
		val := value.Content[i+1]
		switch {
		case key == "version":
			if err := val.Decode(&d.Version); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "signal":
			if err := val.Decode(&d.Signal); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "resources" || strings.HasPrefix(key, "resources/"):
			// Handled through ResourcesAssertion below.
		default:
			return fmt.Errorf("unsupported document key %q", key)
		}
	}

	if resourcesNode != nil {
		if err := d.Resources.UnmarshalYAML(resourcesNode); err != nil {
			return err
		}
	}

	return nil
}

func (r *resourceAssertion) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("resource assertion must be a map, got YAML kind %d", value.Kind)
	}

	scopesNode, err := extractCollectionNodeFromParent(value, "scopes")
	if err != nil {
		return err
	}

	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i].Value
		val := value.Content[i+1]
		switch {
		case key == "attributes":
			if err := val.Decode(&r.Attributes); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "scopes" || strings.HasPrefix(key, "scopes/"):
			// Handled through ScopesAssertion below.
		default:
			return fmt.Errorf("unsupported resource assertion key %q", key)
		}
	}

	if scopesNode != nil {
		if err := r.Scopes.UnmarshalYAML(scopesNode); err != nil {
			return err
		}
	}

	return nil
}

func (s *scopeAssertion) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("scope assertion must be a map, got YAML kind %d", value.Kind)
	}

	metricsNode, err := extractCollectionNodeFromParent(value, "metrics")
	if err != nil {
		return err
	}

	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i].Value
		val := value.Content[i+1]
		switch {
		case key == "name":
			if err := val.Decode(&s.Name); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "version":
			if err := val.Decode(&s.Version); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "metrics" || strings.HasPrefix(key, "metrics/"):
			// Handled through MetricsAssertion below.
		default:
			return fmt.Errorf("unsupported scope assertion key %q", key)
		}
	}

	if metricsNode != nil {
		if err := s.Metrics.UnmarshalYAML(metricsNode); err != nil {
			return err
		}
	}

	return nil
}

func (m *metricAssertion) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("metric assertion must be a map, got YAML kind %d", value.Kind)
	}

	datapointsNode, err := extractCollectionNodeFromParent(value, "datapoints")
	if err != nil {
		return err
	}

	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i].Value
		val := value.Content[i+1]
		switch {
		case key == "name":
			if err := val.Decode(&m.Name); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "type":
			if err := val.Decode(&m.Type); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "unit":
			if err := val.Decode(&m.Unit); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "temporality":
			if err := val.Decode(&m.Temporality); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "monotonic":
			if err := val.Decode(&m.Monotonic); err != nil {
				return fmt.Errorf("decode %q: %w", key, err)
			}
		case key == "datapoints" || strings.HasPrefix(key, "datapoints/"):
			// Handled through DatapointsAssertion below.
		default:
			return fmt.Errorf("unsupported metric assertion key %q", key)
		}
	}

	if datapointsNode != nil {
		if err := m.Datapoints.UnmarshalYAML(datapointsNode); err != nil {
			return err
		}
	}

	return nil
}

func extractCollectionNodeFromParent(parent *yaml.Node, baseKey string) (*yaml.Node, error) {
	var direct *yaml.Node
	var withSuffix []struct {
		key string
		val *yaml.Node
	}

	for i := 0; i < len(parent.Content); i += 2 {
		key := parent.Content[i].Value
		val := parent.Content[i+1]
		switch {
		case key == baseKey:
			if direct != nil {
				return nil, fmt.Errorf("duplicate %s assertion key %q", baseKey, key)
			}
			direct = val
		case strings.HasPrefix(key, baseKey+"/"):
			withSuffix = append(withSuffix, struct {
				key string
				val *yaml.Node
			}{key: key, val: val})
		}
	}

	if direct == nil && len(withSuffix) == 0 {
		return nil, nil
	}

	if len(withSuffix) == 0 {
		return direct, nil
	}

	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if direct != nil {
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: baseKey}, direct)
	}
	for _, entry := range withSuffix {
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: entry.key}, entry.val)
	}
	return m, nil
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
}

func expandResourceShorthand(r *resourceAssertion) {
	for i := range r.Scopes.Exact {
		expandScopeShorthand(&r.Scopes.Exact[i])
	}
}

func expandScopeShorthand(s *scopeAssertion) {
	for i := range s.Metrics.Exact {
		expandMetricShorthand(&s.Metrics.Exact[i])
	}
}

func expandMetricShorthand(m *metricAssertion) {
	if len(m.Datapoints.Exact) == 0 && m.Datapoints.Count == nil {
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
	if doc.Resources.Count == nil {
		doc.Resources = ResourcesAssertion{Exact: doc.Resources.Exact}
	}
}

func compactResourceShorthand(r *resourceAssertion) {
	for i := range r.Scopes.Exact {
		compactScopeShorthand(&r.Scopes.Exact[i])
	}
	if r.Scopes.Count == nil {
		r.Scopes = ScopesAssertion{Exact: r.Scopes.Exact}
	}
}

func compactScopeShorthand(s *scopeAssertion) {
	for i := range s.Metrics.Exact {
		compactMetricShorthand(&s.Metrics.Exact[i])
	}
	if s.Metrics.Count == nil {
		s.Metrics = MetricsAssertion{Exact: s.Metrics.Exact}
	}
}

func compactMetricShorthand(m *metricAssertion) {
	if len(m.Datapoints.Exact) == 1 && len(m.Datapoints.Exact[0].Attributes) == 0 && m.Datapoints.Count == nil {
		m.Datapoints = DatapointsAssertion{}
	}
}
