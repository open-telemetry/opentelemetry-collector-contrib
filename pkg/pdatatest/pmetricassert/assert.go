// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"errors"
	"fmt"
	"sort"

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

	expRes := indexResources(expected.Resources)
	actRes := indexResources(actual.Resources)

	for key, er := range expRes {
		ar, ok := actRes[key]
		if !ok {
			errs = append(errs, fmt.Errorf("missing expected resource: %v", er.Attributes))
			continue
		}
		if err := compareResource(er, ar); err != nil {
			errs = append(errs, fmt.Errorf("resource %v: %w", er.Attributes, err))
		}
	}
	for key, ar := range actRes {
		if _, ok := expRes[key]; !ok {
			errs = append(errs, fmt.Errorf("unexpected resource: %v", ar.Attributes))
		}
	}
	return errors.Join(errs...)
}

func indexResources(rs []resourceAssertion) map[string]resourceAssertion {
	out := make(map[string]resourceAssertion, len(rs))
	for _, r := range rs {
		out[canonKey(r.Attributes)] = r
	}
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
	expIdx := indexDatapoints(expected)
	actIdx := indexDatapoints(actual)

	var missing, unexpected []string
	for k := range expIdx {
		if _, ok := actIdx[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range actIdx {
		if _, ok := expIdx[k]; !ok {
			unexpected = append(unexpected, k)
		}
	}
	if len(missing) == 0 && len(unexpected) == 0 {
		return nil
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	var errs []error
	for _, k := range missing {
		errs = append(errs, fmt.Errorf("missing datapoint with attributes %s", k))
	}
	for _, k := range unexpected {
		errs = append(errs, fmt.Errorf("unexpected datapoint with attributes %s", k))
	}
	return errors.Join(errs...)
}

func indexDatapoints(dps []datapointAssertion) map[string]datapointAssertion {
	out := make(map[string]datapointAssertion, len(dps))
	for _, dp := range dps {
		out[canonKey(dp.Attributes)] = dp
	}
	return out
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
