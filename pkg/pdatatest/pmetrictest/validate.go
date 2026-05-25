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
// Currently it checks that no two datapoints within the same metric share
// identical attribute sets (duplicate datapoint identity).
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
