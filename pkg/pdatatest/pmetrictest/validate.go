// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetrictest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// ValidateMetrics reports semantic errors in md (Metrics Data).
// Currently it checks:
//   - No two datapoints within the same metric share identical attribute sets
//     (duplicate datapoint identity).
//   - No two metrics with the same name under the same Resource + Scope have
//     conflicting identifying fields (type, unit, temporality, monotonicity).
//
// It returns nil if no violations are found.
func ValidateMetrics(md pmetric.Metrics) error {
	var errs error

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			ms := sm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if err := validateDatapointUniqueness(m); err != nil {
					errPrefix := fmt.Sprintf(`resource "%v": scope %q: metric %q`,
						rm.Resource().Attributes().AsRaw(), sm.Scope().Name(), m.Name())
					errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, err))
				}
			}

			// Check for conflicting identifying fields across metrics with the same name.
			if err := validateMetricIdentityConflicts(ms); err != nil {
				errPrefix := fmt.Sprintf(`resource "%v": scope %q`,
					rm.Resource().Attributes().AsRaw(), sm.Scope().Name())
				errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, err))
			}
		}
	}

	return errs
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

type metricIdentity struct {
	metricType  pmetric.MetricType
	unit        string
	temporality pmetric.AggregationTemporality
	monotonic   bool
}

// getMetricIdentity extracts the identifying fields from a metric.
// For metric types that do not support temporality or monotonicity
// (Gauge, Summary, Empty), those fields remain at their zero values.
func getMetricIdentity(m pmetric.Metric) metricIdentity {
	id := metricIdentity{
		metricType: m.Type(),
		unit:       m.Unit(),
	}
	//exhaustive:enforce
	switch m.Type() {
	case pmetric.MetricTypeSum:
		id.temporality = m.Sum().AggregationTemporality()
		id.monotonic = m.Sum().IsMonotonic()
	case pmetric.MetricTypeHistogram:
		id.temporality = m.Histogram().AggregationTemporality()
	case pmetric.MetricTypeExponentialHistogram:
		id.temporality = m.ExponentialHistogram().AggregationTemporality()
	case pmetric.MetricTypeGauge:
	case pmetric.MetricTypeSummary:
	case pmetric.MetricTypeEmpty:
	}
	return id
}

// describeIdentityConflict returns a description of which
// identifying fields differ between two metric identities.
func describeIdentityConflict(a, b metricIdentity) string {
	var parts []string
	if a.metricType != b.metricType {
		parts = append(parts, fmt.Sprintf("type: %s vs %s", a.metricType, b.metricType))
	}
	if a.unit != b.unit {
		parts = append(parts, fmt.Sprintf("unit: %q vs %q", a.unit, b.unit))
	}
	if a.temporality != b.temporality {
		parts = append(parts, fmt.Sprintf("temporality: %s vs %s", a.temporality, b.temporality))
	}
	if a.monotonic != b.monotonic {
		parts = append(parts, fmt.Sprintf("monotonic: %v vs %v", a.monotonic, b.monotonic))
	}
	return strings.Join(parts, ", ")
}

// validateMetricIdentityConflicts checks that no two metrics in ms with the
// same name have different identifying fields (type, unit, temporality,
// monotonicity).
func validateMetricIdentityConflicts(ms pmetric.MetricSlice) error {
	type entry struct {
		identity metricIdentity
		index    int
	}
	seen := make(map[string]entry) // metric name -> first-seen identity + index
	var errs error

	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		id := getMetricIdentity(m)
		name := m.Name()

		if prev, exists := seen[name]; exists {
			if prev.identity != id {
				errs = multierr.Append(errs, fmt.Errorf(
					"metric %q at index %d has conflicting identifying fields with metric at index %d: %s",
					name, i, prev.index, describeIdentityConflict(prev.identity, id),
				))
			}
		} else {
			seen[name] = entry{identity: id, index: i}
		}
	}

	return errs
}
