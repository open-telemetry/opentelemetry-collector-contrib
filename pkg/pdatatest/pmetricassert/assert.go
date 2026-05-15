// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"errors"
	"fmt"
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
	actualDoc := normalize(actual)
	return compareDocuments(expected, actualDoc)
}

func compareDocuments(expected, actual *document) error {
	var errs []error

	projection, err := newAttributeProjection(resourceAttributeMaps(expected.Resources))
	if err != nil {
		return err
	}
	expectedResources := sortResources(expected.Resources, projection, true)
	actualResources := sortResources(actual.Resources, projection, false)

	for ei, ai := 0, 0; ei < len(expectedResources) || ai < len(actualResources); {
		switch {
		case ai == len(actualResources):
			er := expectedResources[ei].resource
			errs = append(errs, fmt.Errorf("missing expected resource: %v", er.Attributes))
			ei++
		case ei == len(expectedResources):
			ar := actualResources[ai].resource
			errs = append(errs, fmt.Errorf("unexpected resource: %v", ar.Attributes))
			ai++
		case expectedResources[ei].key < actualResources[ai].key:
			er := expectedResources[ei].resource
			errs = append(errs, fmt.Errorf("missing expected resource: %v", er.Attributes))
			ei++
		case expectedResources[ei].key > actualResources[ai].key:
			ar := actualResources[ai].resource
			errs = append(errs, fmt.Errorf("unexpected resource: %v", ar.Attributes))
			ai++
		default:
			er := expectedResources[ei].resource
			ar := actualResources[ai].resource
			if err := compareAttributes(er.Attributes, ar.Attributes); err != nil {
				errs = append(errs, fmt.Errorf("resource %v attributes: %w", er.Attributes, err))
			}
			if err := compareResource(er, ar); err != nil {
				errs = append(errs, fmt.Errorf("resource %v: %w", er.Attributes, err))
			}
			ei++
			ai++
		}
	}
	return errors.Join(errs...)
}

type keyedResource struct {
	key      string
	resource resourceAssertion
}

func resourceAttributeMaps(resources []resourceAssertion) []map[string]any {
	out := make([]map[string]any, 0, len(resources))
	for _, r := range resources {
		out = append(out, r.Attributes)
	}
	return out
}

func sortResources(resources []resourceAssertion, projection attributeProjection, expected bool) []keyedResource {
	out := make([]keyedResource, 0, len(resources))
	for _, r := range resources {
		out = append(out, keyedResource{
			key:      projectedAttributeKey(r.Attributes, projection, expected),
			resource: r,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].key < out[j].key
	})
	return out
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
	projection, err := newAttributeProjection(datapointAttributeMaps(expected))
	if err != nil {
		return err
	}
	expectedDatapoints := sortDatapoints(expected, projection, true)
	actualDatapoints := sortDatapoints(actual, projection, false)
	var missing, unexpected []string
	var errs []error

	for ei, ai := 0, 0; ei < len(expectedDatapoints) || ai < len(actualDatapoints); {
		switch {
		case ai == len(actualDatapoints):
			edp := expectedDatapoints[ei].datapoint
			missing = append(missing, canonKey(edp.Attributes))
			ei++
		case ei == len(expectedDatapoints):
			adp := actualDatapoints[ai].datapoint
			unexpected = append(unexpected, canonKey(adp.Attributes))
			ai++
		case expectedDatapoints[ei].key < actualDatapoints[ai].key:
			edp := expectedDatapoints[ei].datapoint
			missing = append(missing, canonKey(edp.Attributes))
			ei++
		case expectedDatapoints[ei].key > actualDatapoints[ai].key:
			adp := actualDatapoints[ai].datapoint
			unexpected = append(unexpected, canonKey(adp.Attributes))
			ai++
		default:
			edp := expectedDatapoints[ei].datapoint
			adp := actualDatapoints[ai].datapoint
			if err := compareAttributes(edp.Attributes, adp.Attributes); err != nil {
				errs = append(errs, fmt.Errorf("datapoint attributes %s: %w", canonKey(edp.Attributes), err))
			}
			ei++
			ai++
		}
	}
	if len(missing) == 0 && len(unexpected) == 0 {
		return errors.Join(errs...)
	}
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

type keyedDatapoint struct {
	key       string
	datapoint datapointAssertion
}

func datapointAttributeMaps(datapoints []datapointAssertion) []map[string]any {
	out := make([]map[string]any, 0, len(datapoints))
	for _, dp := range datapoints {
		out = append(out, dp.Attributes)
	}
	return out
}

func sortDatapoints(datapoints []datapointAssertion, projection attributeProjection, expected bool) []keyedDatapoint {
	out := make([]keyedDatapoint, 0, len(datapoints))
	for _, dp := range datapoints {
		out = append(out, keyedDatapoint{
			key:       projectedAttributeKey(dp.Attributes, projection, expected),
			datapoint: dp,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].key < out[j].key
	})
	return out
}

func compareAttributes(expected, actual map[string]any) error {
	var errs []error
	expectedKeys := map[string]struct{}{}
	for rawKey, expectedValue := range expected {
		key, operator := splitAttributeOperator(rawKey)
		expectedKeys[key] = struct{}{}
		switch operator {
		case "":
			actualValue, ok := actual[key]
			if !ok {
				errs = append(errs, fmt.Errorf("missing attribute %q", key))
				continue
			}
			if canonKey(expectedValue) != canonKey(actualValue) {
				errs = append(errs, fmt.Errorf("attribute %q mismatch: expected %v, got %v", key, expectedValue, actualValue))
			}
		case "exists":
			shouldExist, ok := expectedValue.(bool)
			if !ok {
				errs = append(errs, fmt.Errorf("attribute %q/exists must be a boolean", key))
				continue
			}
			_, exists := actual[key]
			if shouldExist && !exists {
				errs = append(errs, fmt.Errorf("missing attribute %q required by /exists", key))
			}
			if !shouldExist && exists {
				errs = append(errs, fmt.Errorf("attribute %q is present but /exists is false", key))
			}
		}
	}
	for key := range actual {
		if _, ok := expectedKeys[key]; !ok {
			errs = append(errs, fmt.Errorf("unexpected attribute %q", key))
		}
	}
	return errors.Join(errs...)
}

type attributeProjection struct {
	operators map[string]string
}

func newAttributeProjection(attrsList []map[string]any) (attributeProjection, error) {
	operators := map[string]string{}
	exactKeys := map[string]struct{}{}
	for _, attrs := range attrsList {
		for rawKey := range attrs {
			key, operator := splitAttributeOperator(rawKey)
			if operator == "" {
				exactKeys[key] = struct{}{}
				continue
			}
			if existing, ok := operators[key]; ok && existing != operator {
				return attributeProjection{}, fmt.Errorf("attribute %q uses conflicting operators %q and %q", key, existing, operator)
			}
			operators[key] = operator
		}
	}
	for key := range exactKeys {
		if operator, ok := operators[key]; ok {
			return attributeProjection{}, fmt.Errorf("attribute %q mixes exact matching and /%s matching in the same collection", key, operator)
		}
	}
	return attributeProjection{operators: operators}, nil
}

func projectedAttributeKey(attrs map[string]any, projection attributeProjection, expected bool) string {
	projected := map[string]any{}
	for key, operator := range projection.operators {
		rawKey := key + "/" + operator
		if expected {
			if value, ok := attrs[rawKey]; ok {
				projected[rawKey] = value
				continue
			}
			projected[rawKey] = defaultOperatorProjectionValue(operator)
			continue
		}
		projected[rawKey] = actualOperatorProjectionValue(key, operator, attrs)
	}
	for rawKey, value := range attrs {
		key, operator := splitAttributeOperator(rawKey)
		if operator != "" {
			continue
		}
		if _, ok := projection.operators[key]; ok {
			continue
		}
		projected[rawKey] = value
	}
	return canonKey(projected)
}

func defaultOperatorProjectionValue(operator string) any {
	switch operator {
	case "exists":
		return false
	default:
		return nil
	}
}

func actualOperatorProjectionValue(key, operator string, attrs map[string]any) any {
	switch operator {
	case "exists":
		_, ok := attrs[key]
		return ok
	default:
		return nil
	}
}

func splitAttributeOperator(key string) (string, string) {
	const existsSuffix = "/exists"
	if name, ok := strings.CutSuffix(key, existsSuffix); ok {
		return name, "exists"
	}
	return key, ""
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
