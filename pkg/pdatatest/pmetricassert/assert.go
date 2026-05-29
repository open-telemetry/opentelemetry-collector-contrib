// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// AssertMetrics compares actual against the assertion file at expectedPath.
//
// The comparison is order-insensitive across resources, scopes, metrics, and
// datapoints. Resource attributes, scope identity, metric metadata, and
// datapoint attribute permutations must match exactly. Values, timestamps,
// and exemplars are ignored.
func AssertMetrics(expectedPath string, actual pmetric.Metrics) error {
	expected, err := readDocument(expectedPath)
	if err != nil {
		return err
	}
	actualDoc := normalize(actual, writeOptions{includeValues: true})
	return compareDocuments(expected, actualDoc)
}

func compareDocuments(expected, actual *document) error {
	var errs []error
	matched := make([]bool, len(actual.Resources))

	for _, er := range expected.Resources {
		idx := findMatchingAttributes(er.Attributes, matched, len(actual.Resources), func(i int) map[string]any {
			return actual.Resources[i].Attributes
		})
		if idx < 0 {
			errs = append(errs, fmt.Errorf("missing expected resource: %v", er.Attributes))
			continue
		}
		matched[idx] = true
		if err := compareResource(er, actual.Resources[idx]); err != nil {
			errs = append(errs, fmt.Errorf("resource %v: %w", er.Attributes, err))
		}
	}
	for i, ar := range actual.Resources {
		if !matched[i] {
			errs = append(errs, fmt.Errorf("unexpected resource: %v", ar.Attributes))
		}
	}
	return errors.Join(errs...)
}

func compareResource(expected, actual resourceAssertion) error {
	var errs []error
	expScopes := indexScopes(expected.Scopes)
	actScopes := indexScopes(actual.Scopes)

	for key, es := range expScopes {
		as, ok := actScopes[key]
		if !ok {
			errs = append(errs, fmt.Errorf("missing expected scope name=%q version=%q", es.Name, es.Version))
			continue
		}
		if err := compareScope(es, as); err != nil {
			errs = append(errs, fmt.Errorf("scope name=%q: %w", es.Name, err))
		}
	}
	for key, as := range actScopes {
		if _, ok := expScopes[key]; !ok {
			errs = append(errs, fmt.Errorf("unexpected scope name=%q version=%q", as.Name, as.Version))
		}
	}
	return errors.Join(errs...)
}

func indexScopes(ss []scopeAssertion) map[string]scopeAssertion {
	out := make(map[string]scopeAssertion, len(ss))
	for _, s := range ss {
		out[s.Name+"|"+s.Version] = s
	}
	return out
}

func compareScope(expected, actual scopeAssertion) error {
	var errs []error
	expMetrics := indexMetrics(expected.Metrics)
	actMetrics := indexMetrics(actual.Metrics)

	for name, em := range expMetrics {
		am, ok := actMetrics[name]
		if !ok {
			errs = append(errs, fmt.Errorf("missing expected metric %q", name))
			continue
		}
		if err := compareMetric(em, am); err != nil {
			errs = append(errs, fmt.Errorf("metric %q: %w", name, err))
		}
	}
	for name := range actMetrics {
		if _, ok := expMetrics[name]; !ok {
			errs = append(errs, fmt.Errorf("unexpected metric %q", name))
		}
	}
	return errors.Join(errs...)
}

func indexMetrics(ms []metricAssertion) map[string]metricAssertion {
	out := make(map[string]metricAssertion, len(ms))
	for _, m := range ms {
		out[m.Name] = m
	}
	return out
}

func compareMetric(expected, actual metricAssertion) error {
	var errs []error
	if expected.Type != actual.Type {
		errs = append(errs, fmt.Errorf("type mismatch: expected %q, got %q", expected.Type, actual.Type))
	}
	if expected.Unit != actual.Unit {
		errs = append(errs, fmt.Errorf("unit mismatch: expected %q, got %q", expected.Unit, actual.Unit))
	}
	if expected.Temporality != actual.Temporality {
		errs = append(errs, fmt.Errorf("temporality mismatch: expected %q, got %q", expected.Temporality, actual.Temporality))
	}
	if !boolPtrEqual(expected.Monotonic, actual.Monotonic) {
		errs = append(errs, fmt.Errorf("monotonic mismatch: expected %v, got %v",
			boolPtrString(expected.Monotonic), boolPtrString(actual.Monotonic)))
	}
	if err := compareDatapoints(expected.Datapoints, actual.Datapoints); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func compareDatapoints(expected, actual []datapointAssertion) error {
	matched := make([]bool, len(actual))
	var missing, unexpected []string
	var valErrs []error

	for _, edp := range expected {
		idx := findMatchingAttributes(edp.Attributes, matched, len(actual), func(i int) map[string]any {
			return actual[i].Attributes
		})
		if idx < 0 {
			missing = append(missing, canonKey(edp.Attributes))
			continue
		}
		matched[idx] = true

		if edp.Value != nil {
			if err := compareValue(edp.Value, actual[idx].Value); err != nil {
				valErrs = append(valErrs, fmt.Errorf("datapoint %s: %w", canonKey(edp.Attributes), err))
			}
		}
	}
	for i, adp := range actual {
		if !matched[i] {
			unexpected = append(unexpected, canonKey(adp.Attributes))
		}
	}
	if len(missing) == 0 && len(unexpected) == 0 && len(valErrs) == 0 {
		return nil
	}
	var errs []error
	errs = append(errs, valErrs...)
	sort.Strings(missing)
	sort.Strings(unexpected)
	for _, k := range missing {
		errs = append(errs, fmt.Errorf("missing datapoint with attributes %s", k))
	}
	for _, k := range unexpected {
		errs = append(errs, fmt.Errorf("unexpected datapoint with attributes %s", k))
	}
	return errors.Join(errs...)
}

func compareValue(expected, actual any) error {
	if expected == actual {
		return nil
	}

	expFloat, expIsFloat := toFloat64(expected)
	actFloat, actIsFloat := toFloat64(actual)

	expInt, expIsInt := toInt64(expected)
	actInt, actIsInt := toInt64(actual)

	if expIsInt && actIsInt && expInt == actInt {
		return nil
	}

	if expIsFloat && actIsFloat && expFloat == actFloat {
		return nil
	}

	return fmt.Errorf("value mismatch: expected %v, got %v", expected, actual)
}

func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint64:
		return float64(x), true
	default:
		return 0, false
	}
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case uint64:
		return int64(x), true
	case float64:
		if float64(int64(x)) == x {
			return int64(x), true
		}
	}
	return 0, false
}

// findMatchingAttributes returns the first unmatched index whose attributes
// satisfy the expected attribute map, or -1 if none do.
func findMatchingAttributes(expected map[string]any, matched []bool, n int, attrsAt func(int) map[string]any) int {
	for i := range n {
		if matched[i] {
			continue
		}
		if compareAttributes(expected, attrsAt(i)) == nil {
			return i
		}
	}
	return -1
}

func compareAttributes(expected, actual map[string]any) error {
	var errs []error
	seen := make(map[string]struct{}, len(expected))
	for rawKey, expectedValue := range expected {
		if key, ok := strings.CutSuffix(rawKey, "/exists"); ok {
			seen[key] = struct{}{}
			if expectedValue != true {
				errs = append(errs, fmt.Errorf("attribute %q/exists must be true (the only supported value)", key))
				continue
			}
			if _, exists := actual[key]; !exists {
				errs = append(errs, fmt.Errorf("missing attribute %q required by /exists", key))
			}
			continue
		}
		if key, ok := strings.CutSuffix(rawKey, "/regex"); ok {
			seen[key] = struct{}{}
			actualValue, exists := actual[key]
			if !exists {
				errs = append(errs, fmt.Errorf("missing attribute %q required by /regex", key))
				continue
			}
			if err := compareRegexAttribute(key, expectedValue, actualValue); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		seen[rawKey] = struct{}{}
		actualValue, ok := actual[rawKey]
		if !ok {
			errs = append(errs, fmt.Errorf("missing attribute %q", rawKey))
			continue
		}
		if canonKey(expectedValue) != canonKey(actualValue) {
			errs = append(errs, fmt.Errorf("attribute %q mismatch: expected %v, got %v", rawKey, expectedValue, actualValue))
		}
	}
	for key := range actual {
		if _, ok := seen[key]; !ok {
			errs = append(errs, fmt.Errorf("unexpected attribute %q", key))
		}
	}
	return errors.Join(errs...)
}

func compareRegexAttribute(key string, expectedValue, actualValue any) error {
	pattern, ok := expectedValue.(string)
	if !ok {
		return fmt.Errorf("attribute %q/regex must be a string pattern", key)
	}
	actualStr, ok := actualValue.(string)
	if !ok {
		return fmt.Errorf("attribute %q must be a string to match /regex (got %T)", key, actualValue)
	}
	re, err := regexp.Compile("^(?:" + pattern + ")$")
	if err != nil {
		return fmt.Errorf("attribute %q/regex has invalid pattern %q: %w", key, pattern, err)
	}
	if !re.MatchString(actualStr) {
		return fmt.Errorf("attribute %q value %q does not match regex %q", key, actualStr, pattern)
	}
	return nil
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func boolPtrString(p *bool) string {
	if p == nil {
		return "<unset>"
	}
	return fmt.Sprintf("%v", *p)
}
