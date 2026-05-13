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
// identity fields only. Operator-suffix extensions (/include, /regex,
// /count, ...) are tracked as follow-ups.
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
	Attributes map[string]any `yaml:"attributes,omitempty"`
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
				if len(m.Datapoints) == 1 && len(m.Datapoints[0].Attributes) == 0 {
					m.Datapoints = nil
				}
			}
		}
	}
}
