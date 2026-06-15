// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetrictest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// ValidateMetrics reports semantic errors in md (Metrics Data).
// Currently it checks:
//   - No ResourceMetrics entry has an empty ScopeMetrics set.
//   - No ScopeMetrics entry has an empty Metrics set.
//   - No typed Metric has an empty datapoint set.
//   - No two datapoints within the same metric share identical attribute sets.
//   - No two metrics share the same name under the same scope.
//   - No two ScopeMetrics share the same scope (name + version) under the same resource.
//   - No two ResourceMetrics share the same resource attributes.
//
// It returns nil if no violations are found.
func ValidateMetrics(md pmetric.Metrics) error {
	var errs error

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		if sms.Len() == 0 {
			errs = multierr.Append(errs, fmt.Errorf(
				`resource metrics at index %d with resource "%v" has no scope metrics`,
				i, rm.Resource().Attributes().AsRaw(),
			))
		}
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			ms := sm.Metrics()
			if ms.Len() == 0 {
				errPrefix := fmt.Sprintf(`resource "%v": scope %q`,
					rm.Resource().Attributes().AsRaw(), sm.Scope().Name())
				errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, fmt.Errorf(
					"scope metrics at index %d has no metrics", j,
				)))
			}
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if err := validateDatapointPresence(m); err != nil {
					errPrefix := fmt.Sprintf(`resource "%v": scope %q: metric %q`,
						rm.Resource().Attributes().AsRaw(), sm.Scope().Name(), m.Name())
					errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, err))
				}
				if err := validateDatapointUniqueness(m); err != nil {
					errPrefix := fmt.Sprintf(`resource "%v": scope %q: metric %q`,
						rm.Resource().Attributes().AsRaw(), sm.Scope().Name(), m.Name())
					errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, err))
				}
			}

			// Check for duplicate metric names within this scope.
			if err := validateDuplicateMetricNames(ms); err != nil {
				errPrefix := fmt.Sprintf(`resource "%v": scope %q`,
					rm.Resource().Attributes().AsRaw(), sm.Scope().Name())
				errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, err))
			}
		}

		// Check for duplicate scope names within this resource.
		if err := validateDuplicateScopes(sms); err != nil {
			errPrefix := fmt.Sprintf(`resource "%v"`,
				rm.Resource().Attributes().AsRaw())
			errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, err))
		}
	}

	// Check for multiple ResourceMetrics with equal resource attributes.
	if err := validateDuplicateResources(rms); err != nil {
		errs = multierr.Append(errs, err)
	}

	return errs
}

// validateDatapointPresence checks that typed metrics have at least one datapoint.
func validateDatapointPresence(m pmetric.Metric) error {
	//exhaustive:enforce
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return checkDatapointPresence("gauge", m.Gauge().DataPoints().Len())
	case pmetric.MetricTypeSum:
		return checkDatapointPresence("sum", m.Sum().DataPoints().Len())
	case pmetric.MetricTypeHistogram:
		return checkDatapointPresence("histogram", m.Histogram().DataPoints().Len())
	case pmetric.MetricTypeExponentialHistogram:
		return checkDatapointPresence("exponential histogram", m.ExponentialHistogram().DataPoints().Len())
	case pmetric.MetricTypeSummary:
		return checkDatapointPresence("summary", m.Summary().DataPoints().Len())
	case pmetric.MetricTypeEmpty:
	}
	return nil
}

func checkDatapointPresence(metricType string, count int) error {
	if count == 0 {
		return fmt.Errorf("%s metric has no datapoints", metricType)
	}
	return nil
}

// validateDatapointUniqueness checks that no two datapoints within the given
// metric have identical attribute sets.
func validateDatapointUniqueness(m pmetric.Metric) error {
	//exhaustive:enforce
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		return checkDuplicateDatapointAttrs(extractAttributeMaps(dps.Len(), func(i int) pcommon.Map {
			return dps.At(i).Attributes()
		}))
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		return checkDuplicateDatapointAttrs(extractAttributeMaps(dps.Len(), func(i int) pcommon.Map {
			return dps.At(i).Attributes()
		}))
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		return checkDuplicateDatapointAttrs(extractAttributeMaps(dps.Len(), func(i int) pcommon.Map {
			return dps.At(i).Attributes()
		}))
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		return checkDuplicateDatapointAttrs(extractAttributeMaps(dps.Len(), func(i int) pcommon.Map {
			return dps.At(i).Attributes()
		}))
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		return checkDuplicateDatapointAttrs(extractAttributeMaps(dps.Len(), func(i int) pcommon.Map {
			return dps.At(i).Attributes()
		}))
	case pmetric.MetricTypeEmpty:
	}
	return nil
}

// extractAttributeMaps builds a slice of attribute maps from any datapoint slice
// using a callback, since the various datapoint slice types share no common interface.
func extractAttributeMaps(count int, getAttrs func(int) pcommon.Map) []pcommon.Map {
	attrs := make([]pcommon.Map, count)
	for i := range count {
		attrs[i] = getAttrs(i)
	}
	return attrs
}

// checkDuplicateDatapointAttrs reports an error for every pair of datapoints
// whose attribute maps hash to the same value.
func checkDuplicateDatapointAttrs(attrs []pcommon.Map) error {
	seen := make(map[[16]byte]int, len(attrs)) // hash to first index
	var errs error

	for i, a := range attrs {
		h := pdatautil.MapHash(a)
		if firstIdx, exists := seen[h]; exists {
			errs = multierr.Append(errs, fmt.Errorf(
				"datapoint at index %d has duplicate attributes with datapoint at index %d, attributes: %v",
				i, firstIdx, a.AsRaw()))
		} else {
			seen[h] = i
		}
	}

	return errs
}

// validateDuplicateMetricNames checks that no two metrics in ms share the
// same name. Each metric name must be unique within a single scope.
func validateDuplicateMetricNames(ms pmetric.MetricSlice) error {
	seen := make(map[string]int, ms.Len()) // metric name to first-seen index
	var errs error

	for i := 0; i < ms.Len(); i++ {
		name := ms.At(i).Name()
		if firstIdx, exists := seen[name]; exists {
			errs = multierr.Append(errs, fmt.Errorf(
				"metric %q at index %d is a duplicate of metric at index %d",
				name, i, firstIdx,
			))
		} else {
			seen[name] = i
		}
	}

	return errs
}

// validateDuplicateResources checks that no two ResourceMetrics in rms share
// the same resource attributes. Each resource should appear at most once.
func validateDuplicateResources(rms pmetric.ResourceMetricsSlice) error {
	seen := make(map[[16]byte]int, rms.Len()) // resource attributes hash to first-seen index
	var errs error

	for i := 0; i < rms.Len(); i++ {
		h := pdatautil.MapHash(rms.At(i).Resource().Attributes())
		if firstIdx, exists := seen[h]; exists {
			errs = multierr.Append(errs, fmt.Errorf(
				"resource %v at index %d is a duplicate of resource at index %d",
				rms.At(i).Resource().Attributes().AsRaw(), i, firstIdx,
			))
		} else {
			seen[h] = i
		}
	}

	return errs
}

// validateDuplicateScopes checks that no two ScopeMetrics in sms share the
// same scope identity (name + version). Each scope should appear at most once
// per resource.
func validateDuplicateScopes(sms pmetric.ScopeMetricsSlice) error {
	type scopeKey struct {
		name, version string
	}
	seen := make(map[scopeKey]int, sms.Len()) // scope key to first-seen index
	var errs error

	for i := 0; i < sms.Len(); i++ {
		sc := sms.At(i).Scope()
		key := scopeKey{name: sc.Name(), version: sc.Version()}
		if firstIdx, exists := seen[key]; exists {
			errs = multierr.Append(errs, fmt.Errorf(
				"scope %q (version %q) at index %d is a duplicate of scope at index %d",
				sc.Name(), sc.Version(), i, firstIdx,
			))
		} else {
			seen[key] = i
		}
	}

	return errs
}
