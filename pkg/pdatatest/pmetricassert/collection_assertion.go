// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v3"
)

// MetricAssertion is the public assertion shape for metric identity checks.
//
// The base type is intentionally shared with the internal document model so
// matcher behavior stays consistent between file assertions and in-memory
// collection assertions.
type (
	MetricAssertion    = metricAssertion
	ResourceAssertion  = resourceAssertion
	ScopeAssertion     = scopeAssertion
	DatapointAssertion = datapointAssertion
)

// CountAssertion constrains collection length.
//
// Exact requires one exact value. Min/Max can be used as an inclusive range.
// Exact cannot be combined with Min/Max.
type CountAssertion struct {
	Min   *int `yaml:"min,omitempty"`
	Max   *int `yaml:"max,omitempty"`
	Exact *int `yaml:"exact,omitempty"`
}

// Validate checks the actual collection length against the configured count
// operator.
func (c *CountAssertion) Validate(actualLength int) error {
	if c == nil {
		return nil
	}
	if c.Exact != nil {
		if c.Min != nil || c.Max != nil {
			return errors.New("count assertion is invalid: exact cannot be combined with min/max")
		}
		if *c.Exact < 0 {
			return fmt.Errorf("count assertion is invalid: exact must be >= 0, got %d", *c.Exact)
		}
		if actualLength != *c.Exact {
			return fmt.Errorf("count assertion failed: expected exactly %d items, got %d", *c.Exact, actualLength)
		}
		return nil
	}

	if c.Min != nil && *c.Min < 0 {
		return fmt.Errorf("count assertion is invalid: min must be >= 0, got %d", *c.Min)
	}
	if c.Max != nil && *c.Max < 0 {
		return fmt.Errorf("count assertion is invalid: max must be >= 0, got %d", *c.Max)
	}
	if c.Min != nil && c.Max != nil && *c.Min > *c.Max {
		return fmt.Errorf("count assertion is invalid: min (%d) cannot be greater than max (%d)", *c.Min, *c.Max)
	}
	if c.Min != nil && actualLength < *c.Min {
		return fmt.Errorf("count assertion failed: expected at least %d items, got %d", *c.Min, actualLength)
	}
	if c.Max != nil && actualLength > *c.Max {
		return fmt.Errorf("count assertion failed: expected at most %d items, got %d", *c.Max, actualLength)
	}
	return nil
}

// MetricsAssertion supports exact/count collection operators.
type MetricsAssertion struct {
	Exact []MetricAssertion `yaml:"-"`
	Count *CountAssertion   `yaml:"-"`
}

// ResourcesAssertion supports exact/count collection operators.
type ResourcesAssertion struct {
	Exact []ResourceAssertion `yaml:"-"`
	Count *CountAssertion     `yaml:"-"`
}

// ScopesAssertion supports exact/count collection operators.
type ScopesAssertion struct {
	Exact []ScopeAssertion `yaml:"-"`
	Count *CountAssertion  `yaml:"-"`
}

// DatapointsAssertion supports exact/count collection operators.
type DatapointsAssertion struct {
	Exact []DatapointAssertion `yaml:"-"`
	Count *CountAssertion      `yaml:"-"`
}

// UnmarshalYAML supports these forms:
// - sequence node (equivalent to metrics/exact)
// - map node with keys: metrics, metrics/exact, metrics/count
func (a *MetricsAssertion) UnmarshalYAML(value *yaml.Node) error {
	exact, count, err := unmarshalCollectionBySuffix[MetricAssertion](value, "metrics")
	if err != nil {
		return err
	}
	a.Exact = exact
	a.Count = count
	return nil
}

func (a MetricsAssertion) MarshalYAML() (any, error) {
	return marshalCollection(a.Exact, a.Count), nil
}

func (a *ResourcesAssertion) UnmarshalYAML(value *yaml.Node) error {
	exact, count, err := unmarshalCollectionBySuffix[ResourceAssertion](value, "resources")
	if err != nil {
		return err
	}
	a.Exact = exact
	a.Count = count
	return nil
}

func (a ResourcesAssertion) MarshalYAML() (any, error) {
	return marshalCollection(a.Exact, a.Count), nil
}

func (a *ScopesAssertion) UnmarshalYAML(value *yaml.Node) error {
	exact, count, err := unmarshalCollectionBySuffix[ScopeAssertion](value, "scopes")
	if err != nil {
		return err
	}
	a.Exact = exact
	a.Count = count
	return nil
}

func (a ScopesAssertion) MarshalYAML() (any, error) {
	return marshalCollection(a.Exact, a.Count), nil
}

func (a *DatapointsAssertion) UnmarshalYAML(value *yaml.Node) error {
	exact, count, err := unmarshalCollectionBySuffix[DatapointAssertion](value, "datapoints")
	if err != nil {
		return err
	}
	a.Exact = exact
	a.Count = count
	return nil
}

func (a DatapointsAssertion) MarshalYAML() (any, error) {
	return marshalCollection(a.Exact, a.Count), nil
}

func marshalCollection[T any](exact []T, count *CountAssertion) any {
	if count == nil {
		return exact
	}
	out := map[string]any{}
	if len(exact) > 0 {
		out["exact"] = exact
	}
	out["count"] = count
	return out
}

func unmarshalCollectionBySuffix[T any](value *yaml.Node, baseKey string) (exact []T, count *CountAssertion, err error) {
	if value.Kind == yaml.SequenceNode {
		if err := value.Decode(&exact); err != nil {
			return nil, nil, fmt.Errorf("decode %s exact assertions: %w", baseKey, err)
		}
		return exact, nil, nil
	}

	if value.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("%s assertion must be a sequence or map, got YAML kind %d", baseKey, value.Kind)
	}

	for i := 0; i < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]
		key := keyNode.Value
		switch key {
		case baseKey, baseKey + "/exact", "exact":
			if len(exact) > 0 {
				return nil, nil, fmt.Errorf("duplicate %s exact assertion key %q", baseKey, key)
			}
			if err := valNode.Decode(&exact); err != nil {
				return nil, nil, fmt.Errorf("decode %q: %w", key, err)
			}
		case baseKey + "/count", "count":
			if count != nil {
				return nil, nil, fmt.Errorf("duplicate %s count assertion key %q", baseKey, key)
			}
			count = &CountAssertion{}
			if err := valNode.Decode(count); err != nil {
				return nil, nil, fmt.Errorf("decode %q: %w", key, err)
			}
		default:
			return nil, nil, fmt.Errorf("unsupported %s assertion key %q", baseKey, key)
		}
	}

	return exact, count, nil
}

// Validate checks actual metrics against exact/count operators.
func (a MetricsAssertion) Validate(actualMetrics pmetric.MetricSlice) error {
	if a.Count == nil && a.Exact == nil {
		if actualMetrics.Len() != 0 {
			return fmt.Errorf("metrics assertion failed: expected 0 metrics, got %d", actualMetrics.Len())
		}
		return nil
	}

	if err := a.Count.Validate(actualMetrics.Len()); err != nil {
		return fmt.Errorf("metrics/count: %w", err)
	}

	if a.Exact != nil {
		if err := validateMetricExact(a.Exact, actualMetrics); err != nil {
			return fmt.Errorf("metrics exact assertion failed: %w", err)
		}
	}

	return nil
}

func (a MetricsAssertion) ValidateAssertions(actual []metricAssertion) error {
	if a.Count == nil && a.Exact == nil {
		if len(actual) != 0 {
			return fmt.Errorf("metrics assertion failed: expected 0 metrics, got %d", len(actual))
		}
		return nil
	}

	if err := a.Count.Validate(len(actual)); err != nil {
		return fmt.Errorf("metrics/count: %w", err)
	}

	if a.Exact != nil {
		if err := validateExactCollection("metric", a.Exact, actual, func(expected, got metricAssertion) error {
			return expected.MatchesAssertion(got)
		}); err != nil {
			return fmt.Errorf("metrics exact assertion failed: %w", err)
		}
	}

	return nil
}

func (a ResourcesAssertion) Validate(actual []resourceAssertion) error {
	if a.Count == nil && a.Exact == nil {
		if len(actual) != 0 {
			return fmt.Errorf("resources assertion failed: expected 0 resources, got %d", len(actual))
		}
		return nil
	}

	if err := a.Count.Validate(len(actual)); err != nil {
		return fmt.Errorf("resources/count: %w", err)
	}
	if a.Exact != nil {
		if err := validateExactCollection("resource", a.Exact, actual, func(expected, got resourceAssertion) error {
			return expected.Matches(got)
		}); err != nil {
			return fmt.Errorf("resources exact assertion failed: %w", err)
		}
	}
	return nil
}

func (a ScopesAssertion) Validate(actual []scopeAssertion) error {
	if a.Count == nil && a.Exact == nil {
		if len(actual) != 0 {
			return fmt.Errorf("scopes assertion failed: expected 0 scopes, got %d", len(actual))
		}
		return nil
	}

	if err := a.Count.Validate(len(actual)); err != nil {
		return fmt.Errorf("scopes/count: %w", err)
	}
	if a.Exact != nil {
		if err := validateExactCollection("scope", a.Exact, actual, func(expected, got scopeAssertion) error {
			return expected.Matches(got)
		}); err != nil {
			return fmt.Errorf("scopes exact assertion failed: %w", err)
		}
	}
	return nil
}

func (a DatapointsAssertion) Validate(actual []datapointAssertion) error {
	if a.Count == nil && a.Exact == nil {
		if len(actual) != 0 {
			return fmt.Errorf("datapoints assertion failed: expected 0 datapoints, got %d", len(actual))
		}
		return nil
	}

	if err := a.Count.Validate(len(actual)); err != nil {
		return fmt.Errorf("datapoints/count: %w", err)
	}
	if a.Exact != nil {
		if err := validateExactCollection("datapoint", a.Exact, actual, func(expected, got datapointAssertion) error {
			return expected.Matches(got)
		}); err != nil {
			return fmt.Errorf("datapoints exact assertion failed: %w", err)
		}
	}
	return nil
}

func validateMetricExact(expected []MetricAssertion, actual pmetric.MetricSlice) error {
	if len(expected) != actual.Len() {
		return fmt.Errorf("expected %d metrics, got %d", len(expected), actual.Len())
	}

	used := make([]bool, actual.Len())
	for i := range expected {
		em := expected[i]
		found := -1
		for j := 0; j < actual.Len(); j++ {
			if used[j] {
				continue
			}
			if err := em.Matches(actual.At(j)); err == nil {
				found = j
				break
			}
		}
		if found < 0 {
			return fmt.Errorf("expected metric[%d] (%q) was not found in actual metrics", i, em.Name)
		}
		used[found] = true
	}
	return nil
}

func validateExactCollection[T any](itemName string, expected, actual []T, matches func(T, T) error) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("expected %d %ss, got %d", len(expected), itemName, len(actual))
	}

	used := make([]bool, len(actual))
	for i, e := range expected {
		found := -1
		var candidateErrs []error
		for j, a := range actual {
			if used[j] {
				continue
			}
			err := matches(e, a)
			if err == nil {
				found = j
				break
			}
			candidateErrs = append(candidateErrs, err)
		}
		if found < 0 {
			if len(candidateErrs) > 0 {
				return fmt.Errorf("expected %s[%d] was not found in actual %ss: %w", itemName, i, itemName, errors.Join(candidateErrs...))
			}
			return fmt.Errorf("expected %s[%d] was not found in actual %ss", itemName, i, itemName)
		}
		used[found] = true
	}

	return nil
}

func (r resourceAssertion) Matches(actual resourceAssertion) error {
	if err := compareAttributes(r.Attributes, actual.Attributes); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}
	return compareResource(r, actual)
}

func (s scopeAssertion) Matches(actual scopeAssertion) error {
	if s.Name != actual.Name {
		return fmt.Errorf("name mismatch: expected %q, got %q", s.Name, actual.Name)
	}
	if s.Version != actual.Version {
		return fmt.Errorf("version mismatch: expected %q, got %q", s.Version, actual.Version)
	}
	return compareScope(s, actual)
}

func (d datapointAssertion) Matches(actual datapointAssertion) error {
	return compareAttributes(d.Attributes, actual.Attributes)
}

// Matches validates one actual metric against this expected metric assertion.
func (m metricAssertion) Matches(actual pmetric.Metric) error {
	expected := m
	if len(expected.Datapoints.Exact) == 0 && expected.Datapoints.Count == nil {
		expected.Datapoints.Exact = []datapointAssertion{{}}
	}

	actualAssertion := buildMetricAssertion(actual)
	actualAssertion.Datapoints.Exact = make([]datapointAssertion, 0, len(extractDatapointAttributes(actual)))
	for _, attrs := range extractDatapointAttributes(actual) {
		actualAssertion.Datapoints.Exact = append(actualAssertion.Datapoints.Exact, datapointAssertion{Attributes: attrMapToRaw(attrs)})
	}
	return compareMetric(expected, actualAssertion)
}

func (m metricAssertion) MatchesAssertion(actual metricAssertion) error {
	expected := m
	if len(expected.Datapoints.Exact) == 0 && expected.Datapoints.Count == nil {
		expected.Datapoints.Exact = []datapointAssertion{{}}
	}
	return compareMetric(expected, actual)
}
