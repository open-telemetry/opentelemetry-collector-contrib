// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

// Merge will merge the metrics data in mdB into mdA, then return mdA.
// mdB will not be modified. The function will attempt to merge the data in mdB into
// existing ResourceMetrics / ScopeMetrics / Metrics in mdA if possible. If they don't
// exist, new entries will be created as needed.
//
// NOTE: Any "unnecessary" duplicate entries in mdA will *not* be combined. For example if
// mdA contains two ResourcMetric entries with identical Resource values, they will not be
// combined. If you wish to have this behavior, you could call this function twice:
//
//	cleanedMetrics := Merge(pmetric.NewMetrics(), mdA)
//	Merge(cleanedMetrics, mdB)
//
// That said, this will do a large amount of memory copying
func Merge(mdA pmetric.Metrics, mdB pmetric.Metrics) pmetric.Metrics {
outer:
	for i := 0; i < mdB.ResourceMetrics().Len(); i++ {
		rmB := mdB.ResourceMetrics().At(i)
		resourceIDB := identity.OfResource(rmB.Resource())

		for j := 0; j < mdA.ResourceMetrics().Len(); j++ {
			rmA := mdA.ResourceMetrics().At(j)
			resourceIDA := identity.OfResource(rmA.Resource())

			if resourceIDA == resourceIDB {
				mergeResourceMetrics(resourceIDA, rmA, rmB)
				continue outer
			}
		}

		// We didn't find a match
		// Add it to mdA
		newRM := mdA.ResourceMetrics().AppendEmpty()
		rmB.CopyTo(newRM)
	}

	return mdA
}

func mergeResourceMetrics(resourceID identity.Resource, rmA pmetric.ResourceMetrics, rmB pmetric.ResourceMetrics) pmetric.ResourceMetrics {
outer:
	for i := 0; i < rmB.ScopeMetrics().Len(); i++ {
		smB := rmB.ScopeMetrics().At(i)
		scopeIDB := identity.OfScope(resourceID, smB.Scope())

		for j := 0; j < rmA.ScopeMetrics().Len(); j++ {
			smA := rmA.ScopeMetrics().At(j)
			scopeIDA := identity.OfScope(resourceID, smA.Scope())

			if scopeIDA == scopeIDB {
				mergeScopeMetrics(scopeIDA, smA, smB)
				continue outer
			}
		}

		// We didn't find a match
		// Add it to rmA
		newSM := rmA.ScopeMetrics().AppendEmpty()
		smB.CopyTo(newSM)
	}

	return rmA
}

func mergeScopeMetrics(scopeID identity.Scope, smA pmetric.ScopeMetrics, smB pmetric.ScopeMetrics) pmetric.ScopeMetrics {
outer:
	for i := 0; i < smB.Metrics().Len(); i++ {
		mB := smB.Metrics().At(i)
		metricIDB := identity.OfMetric(scopeID, mB)

		for j := 0; j < smA.Metrics().Len(); j++ {
			mA := smA.Metrics().At(j)
			metricIDA := identity.OfMetric(scopeID, mA)

			if metricIDA == metricIDB {
				//exhaustive:enforce
				switch mA.Type() {
				case pmetric.MetricTypeGauge:
					mergeDataPoints(mA.Gauge().DataPoints(), mB.Gauge().DataPoints())
				case pmetric.MetricTypeSum:
					mergeDataPoints(mA.Sum().DataPoints(), mB.Sum().DataPoints())
				case pmetric.MetricTypeHistogram:
					mergeDataPoints(mA.Histogram().DataPoints(), mB.Histogram().DataPoints())
				case pmetric.MetricTypeExponentialHistogram:
					mergeDataPoints(mA.ExponentialHistogram().DataPoints(), mB.ExponentialHistogram().DataPoints())
				case pmetric.MetricTypeSummary:
					mergeDataPoints(mA.Summary().DataPoints(), mB.Summary().DataPoints())
				}

				continue outer
			}
		}

		// We didn't find a match
		// Add it to smA
		newM := smA.Metrics().AppendEmpty()
		mB.CopyTo(newM)
	}

	return smA
}

func mergeDataPoints[DPS dataPointSlice[DP], DP dataPoint[DP]](dataPointsA DPS, dataPointsB DPS) DPS {
	// Append all the datapoints from B to A
	for i := 0; i < dataPointsB.Len(); i++ {
		dpB := dataPointsB.At(i)

		newDP := dataPointsA.AppendEmpty()
		dpB.CopyTo(newDP)
	}

	return dataPointsA
}

type dataPointSlice[DP dataPoint[DP]] interface {
	Len() int
	At(i int) DP
	AppendEmpty() DP
}

type dataPoint[Self any] interface {
	Timestamp() pcommon.Timestamp
	Attributes() pcommon.Map
	CopyTo(dest Self)
}
