// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
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

// MetricsAssertion supports exact/include collection operators.
type ResourcesAssertion struct {
	Values   []resourceAssertion `yaml:"resources,omitempty"`
	Includes []resourceAssertion `yaml:"resources/include,omitempty"`
}

type ScopesAssertion struct {
	Values   []scopeAssertion `yaml:"scopes,omitempty"`
	Includes []scopeAssertion `yaml:"scopes/include,omitempty"`
}

type MetricsAssertion struct {
	Values   []metricAssertion `yaml:"metrics,omitempty"`
	Includes []metricAssertion `yaml:"metrics/include,omitempty"`
}

type DatapointsAssertion struct {
	Values   []datapointAssertion `yaml:"datapoints,omitempty"`
	Includes []datapointAssertion `yaml:"datapoints/include,omitempty"`
}

// Validate checks actual metrics against exact/include operators.
func (a MetricsAssertion) Validate(actualMetrics pmetric.MetricSlice) error {
	if a.Values != nil {
		if err := validateMetricExact(a.Values, actualMetrics); err != nil {
			return fmt.Errorf("metrics exact assertion failed: %w", err)
		}
	}

	if len(a.Includes) > 0 {
		if err := validateMetricInclude(a.Includes, actualMetrics); err != nil {
			return fmt.Errorf("metrics include assertion failed: %w", err)
		}
	}

	return nil
}

func (a MetricsAssertion) ValidateAssertions(actual []metricAssertion) error {
	if len(a.Values) > 0 {
		if err := validateExactCollection("metric", a.Values, actual, func(expected, got metricAssertion) error {
			return expected.MatchesAssertion(got)
		}); err != nil {
			return fmt.Errorf("metrics exact assertion failed: %w", err)
		}
	}

	if len(a.Includes) > 0 {
		if err := validateIncludeCollection("metric", a.Includes, actual, func(expected, got metricAssertion) error {
			return expected.MatchesAssertion(got)
		}); err != nil {
			return fmt.Errorf("metrics include assertion failed: %w", err)
		}
	}

	return nil
}

func (a ResourcesAssertion) Validate(actual []resourceAssertion) error {
	if a.Values != nil {
		if err := validateExactCollection("resource", a.Values, actual, func(expected, got resourceAssertion) error {
			return expected.Matches(got)
		}); err != nil {
			return fmt.Errorf("resources exact assertion failed: %w", err)
		}
	}
	if len(a.Includes) > 0 {
		if err := validateIncludeCollection("resource", a.Includes, actual, func(expected, got resourceAssertion) error {
			return expected.Matches(got)
		}); err != nil {
			return fmt.Errorf("resources include assertion failed: %w", err)
		}
	}
	return nil
}

func (a ScopesAssertion) Validate(actual []scopeAssertion) error {
	if a.Values != nil {
		if err := validateExactCollection("scope", a.Values, actual, func(expected, got scopeAssertion) error {
			return expected.Matches(got)
		}); err != nil {
			return fmt.Errorf("scopes exact assertion failed: %w", err)
		}
	}
	if len(a.Includes) > 0 {
		if err := validateIncludeCollection("scope", a.Includes, actual, func(expected, got scopeAssertion) error {
			return expected.Matches(got)
		}); err != nil {
			return fmt.Errorf("scopes include assertion failed: %w", err)
		}
	}
	return nil
}

func (a DatapointsAssertion) Validate(actual []datapointAssertion) error {
	if a.Values != nil {
		if err := validateExactCollection("datapoint", a.Values, actual, func(expected, got datapointAssertion) error {
			return expected.Matches(got)
		}); err != nil {
			return fmt.Errorf("datapoints exact assertion failed: %w", err)
		}
	}
	if len(a.Includes) > 0 {
		if err := validateIncludeCollection("datapoint", a.Includes, actual, func(expected, got datapointAssertion) error {
			return expected.Matches(got)
		}); err != nil {
			return fmt.Errorf("datapoints include assertion failed: %w", err)
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

func validateMetricInclude(expected []MetricAssertion, actual pmetric.MetricSlice) error {
	for i := range expected {
		em := expected[i]
		matched := false
		for j := 0; j < actual.Len(); j++ {
			if err := em.Matches(actual.At(j)); err == nil {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("included metric[%d] (%q) was not found in actual metrics", i, em.Name)
		}
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

func validateIncludeCollection[T any](itemName string, expected, actual []T, matches func(T, T) error) error {
	for i, e := range expected {
		matched := false
		var candidateErrs []error
		for _, a := range actual {
			err := matches(e, a)
			if err == nil {
				matched = true
				break
			}
			candidateErrs = append(candidateErrs, err)
		}
		if !matched {
			if len(candidateErrs) > 0 {
				return fmt.Errorf("included %s[%d] was not found in actual %ss: %w", itemName, i, itemName, errors.Join(candidateErrs...))
			}
			return fmt.Errorf("included %s[%d] was not found in actual %ss", itemName, i, itemName)
		}
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
	if len(expected.Datapoints.Values) == 0 && len(expected.Datapoints.Includes) == 0 {
		expected.Datapoints.Values = []datapointAssertion{{}}
	}

	actualAssertion := buildMetricAssertion(actual)
	actualAssertion.Datapoints.Values = make([]datapointAssertion, 0, len(extractDatapoints(actual)))
	for _, ed := range extractDatapoints(actual) {
		actualAssertion.Datapoints.Values = append(actualAssertion.Datapoints.Values, datapointAssertion{
			Attributes: attrMapToRaw(ed.attributes),
			Value:      ed.value,
		})
	}
	return compareMetric(expected, actualAssertion)
}

func (m metricAssertion) MatchesAssertion(actual metricAssertion) error {
	expected := m
	if len(expected.Datapoints.Values) == 0 && len(expected.Datapoints.Includes) == 0 {
		expected.Datapoints.Values = []datapointAssertion{{}}
	}
	return compareMetric(expected, actual)
}
