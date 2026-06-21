// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// documentVersion is the schema version emitted by WriteAssertionFile.
// Readers accept this exact value; bumps must be backwards compatible or
// accompanied by a migration.
const documentVersion = 1

// attributeMode controls how an attribute map assertion is evaluated.
type attributeMode int

const (
	// attributeModeExact requires actual attributes to match expected
	// attributes exactly — no missing and no extra keys.
	attributeModeExact attributeMode = iota
	// attributeModeInclude requires every expected key/value to be present
	// in the actual attributes but allows additional keys.
	attributeModeInclude
)

// document is the YAML-serializable form of a metrics assertion snapshot.
//
// The schema implements the identity-only subset of the grammar proposed in
// issue #48079: default-exact matching, order-insensitive collections,
// identity fields only. Attribute maps support /include mode.
// Operator-suffix extensions (/exclude, /count, /approx, ...) are tracked
// as follow-ups.
type document struct {
	Version   int                 `yaml:"version"`
	Signal    string              `yaml:"signal"`
	Resources []resourceAssertion `yaml:"resources"`
}

type resourceAssertion struct {
	Attributes    map[string]any   `yaml:"attributes,omitempty"`
	AttributeMode attributeMode    `yaml:"-"`
	Scopes        []scopeAssertion `yaml:"scopes"`
}

// UnmarshalYAML implements custom unmarshaling to support `attributes/include`
// as an alternative to `attributes`. When `attributes/include` is used the
// AttributeMode is set to attributeModeInclude; specifying both keys is an error.
func (r *resourceAssertion) UnmarshalYAML(node *yaml.Node) error {
	// Decode into a raw map to detect operator-suffixed keys.
	var raw map[string]yaml.Node
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if inc, ok := raw["attributes/include"]; ok {
		if _, dup := raw["attributes"]; dup {
			return errors.New("resource assertion: cannot specify both 'attributes' and 'attributes/include'")
		}
		var attrs map[string]any
		if err := inc.Decode(&attrs); err != nil {
			return fmt.Errorf("resource assertion: decode attributes/include: %w", err)
		}
		r.Attributes = attrs
		r.AttributeMode = attributeModeInclude
	} else if exact, ok := raw["attributes"]; ok {
		var attrs map[string]any
		if err := exact.Decode(&attrs); err != nil {
			return fmt.Errorf("resource assertion: decode attributes: %w", err)
		}
		r.Attributes = attrs
		r.AttributeMode = attributeModeExact
	}
	if scopesNode, ok := raw["scopes"]; ok {
		if err := scopesNode.Decode(&r.Scopes); err != nil {
			return fmt.Errorf("resource assertion: decode scopes: %w", err)
		}
	}
	return nil
}

type scopeAssertion struct {
	Name    string            `yaml:"name,omitempty"`
	Version string            `yaml:"version,omitempty"`
	Metrics []metricAssertion `yaml:"metrics"`
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
	AttributeMode  attributeMode  `yaml:"-"`
	Value          any            `yaml:"value,omitempty"`
	Count          *uint64        `yaml:"count,omitempty"`
	Sum            *float64       `yaml:"sum,omitempty"`
	ExplicitBounds []float64      `yaml:"explicit_bounds,omitempty"`
	BucketCounts   []uint64       `yaml:"bucket_counts,omitempty"`
	Min            *float64       `yaml:"min,omitempty"`
	Max            *float64       `yaml:"max,omitempty"`
}

// UnmarshalYAML implements custom unmarshaling to support `attributes/include`
// as an alternative to `attributes` on datapoint assertions.
func (d *datapointAssertion) UnmarshalYAML(node *yaml.Node) error {
	var raw map[string]yaml.Node
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if inc, ok := raw["attributes/include"]; ok {
		if _, dup := raw["attributes"]; dup {
			return errors.New("datapoint assertion: cannot specify both 'attributes' and 'attributes/include'")
		}
		var attrs map[string]any
		if err := inc.Decode(&attrs); err != nil {
			return fmt.Errorf("datapoint assertion: decode attributes/include: %w", err)
		}
		d.Attributes = attrs
		d.AttributeMode = attributeModeInclude
	} else if exact, ok := raw["attributes"]; ok {
		var attrs map[string]any
		if err := exact.Decode(&attrs); err != nil {
			return fmt.Errorf("datapoint assertion: decode attributes: %w", err)
		}
		d.Attributes = attrs
		d.AttributeMode = attributeModeExact
	}
	// Decode optional value fields.
	if v, ok := raw["value"]; ok {
		var val any
		if err := v.Decode(&val); err != nil {
			return fmt.Errorf("datapoint assertion: decode value: %w", err)
		}
		d.Value = val
	}
	if v, ok := raw["count"]; ok {
		var c uint64
		if err := v.Decode(&c); err != nil {
			return fmt.Errorf("datapoint assertion: decode count: %w", err)
		}
		d.Count = &c
	}
	if v, ok := raw["sum"]; ok {
		var s float64
		if err := v.Decode(&s); err != nil {
			return fmt.Errorf("datapoint assertion: decode sum: %w", err)
		}
		d.Sum = &s
	}
	if v, ok := raw["explicit_bounds"]; ok {
		var eb []float64
		if err := v.Decode(&eb); err != nil {
			return fmt.Errorf("datapoint assertion: decode explicit_bounds: %w", err)
		}
		d.ExplicitBounds = eb
	}
	if v, ok := raw["bucket_counts"]; ok {
		var bc []uint64
		if err := v.Decode(&bc); err != nil {
			return fmt.Errorf("datapoint assertion: decode bucket_counts: %w", err)
		}
		d.BucketCounts = bc
	}
	if v, ok := raw["min"]; ok {
		var min float64
		if err := v.Decode(&min); err != nil {
			return fmt.Errorf("datapoint assertion: decode min: %w", err)
		}
		d.Min = &min
	}
	if v, ok := raw["max"]; ok {
		var max float64
		if err := v.Decode(&max); err != nil {
			return fmt.Errorf("datapoint assertion: decode max: %w", err)
		}
		d.Max = &max
	}
	return nil
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
