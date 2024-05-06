// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/utils"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// removeAccessToken removes the SFX access token label if found in the give resource metric as a resource attribute
func removeAccessToken(dest pmetric.ResourceMetrics) {
	dest.Resource().Attributes().RemoveIf(func(k string, _ pcommon.Value) bool {
		return k == splunk.SFxAccessTokenLabel
	})
}

// matchedHistogramResourceMetrics returns a map with the keys set to the index of resource Metrics containing
// Histogram metric type.
// The value is another map consisting of the ScopeMetric indices in the RM which contain Histogram metric type as keys
// and the value as an int slice with indices of the Histogram metric within the given scope.
// Example output {1: {1: [0,2]}}.
// The above output can be interpreted as Resource at index 1 contains Histogram metrics.
// Within this resource specifically the scope metric at index 1 contain Histograms.
// Lastly, the scope metric at index 1 has two Histogram type metric which can be found at index 0 and 2.
func matchedHistogramResourceMetrics(md pmetric.Metrics) (matchedRMIdx map[int]map[int][]int) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		matchedSMIdx := matchedHistogramScopeMetrics(rm)
		if len(matchedSMIdx) > 0 {
			if matchedRMIdx == nil {
				matchedRMIdx = map[int]map[int][]int{}
			}
			matchedRMIdx[i] = matchedSMIdx
		}
	}
	return
}

// matchedHistogramScopeMetrics returns a map with keys equal to the ScopeMetric indices in the input resource metric
// which contain Histogram metric type.
// And the value is an int slice with indices of the Histogram metric within the keyed scope metric.
// Example output {1: [0,2]}.
// The above output can be interpreted as scope metrics at index 1 contains Histogram metrics.
// And that the scope metric at index 1 has two Histogram type metric which can be found at index 0 and 2.
func matchedHistogramScopeMetrics(rm pmetric.ResourceMetrics) (matchedSMIdx map[int][]int) {
	ilms := rm.ScopeMetrics()
	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		matchedMetricsIdx := matchedHistogramMetrics(ilm)
		if len(matchedMetricsIdx) > 0 {
			if matchedSMIdx == nil {
				matchedSMIdx = map[int][]int{}
			}
			matchedSMIdx[i] = matchedMetricsIdx
		}
	}
	return
}

// matchedHistogramMetrics returns an int slice with indices of metrics which are of Histogram type
// within the input scope metric.
// Example output [0,2].
// The above output can be interpreted as input scope metric has Histogram type metric at index 0 and 2.
func matchedHistogramMetrics(ilm pmetric.ScopeMetrics) (matchedMetricsIdx []int) {
	ms := ilm.Metrics()
	for i := 0; i < ms.Len(); i++ {
		metric := ms.At(i)
		if metric.Type() == pmetric.MetricTypeHistogram {
			matchedMetricsIdx = append(matchedMetricsIdx, i)
		}
	}
	return
}

// GetHistograms returns new Metrics slice containing only Histogram metrics found in the input
// and the count of histogram metrics
func GetHistograms(md pmetric.Metrics) (pmetric.Metrics, int) {
	matchedMetricsIdxes := matchedHistogramResourceMetrics(md)
	matchedRmCount := len(matchedMetricsIdxes)
	if matchedRmCount == 0 {
		return pmetric.Metrics{}, 0
	}

	metricCount := 0
	srcRms := md.ResourceMetrics()
	dest := pmetric.NewMetrics()
	dstRms := dest.ResourceMetrics()
	dstRms.EnsureCapacity(matchedRmCount)

	// Iterate over those ResourceMetrics which were found to contain histograms
	for rmIdx, ilmMap := range matchedMetricsIdxes {
		srcRm := srcRms.At(rmIdx)

		// Copy resource metric properties to dest
		destRm := dstRms.AppendEmpty()
		srcRm.Resource().CopyTo(destRm.Resource())
		destRm.SetSchemaUrl(srcRm.SchemaUrl())

		// Remove Sfx access token
		removeAccessToken(destRm)

		matchedIlmCount := len(ilmMap)
		destIlms := destRm.ScopeMetrics()
		destIlms.EnsureCapacity(matchedIlmCount)
		srcIlms := srcRm.ScopeMetrics()

		// Iterate over ScopeMetrics which were found to contain histograms
		for ilmIdx, metricIdxes := range ilmMap {
			srcIlm := srcIlms.At(ilmIdx)

			// Copy scope properties to dest
			destIlm := destIlms.AppendEmpty()
			srcIlm.Scope().CopyTo(destIlm.Scope())
			destIlm.SetSchemaUrl(srcIlm.SchemaUrl())

			matchedMetricCount := len(metricIdxes)
			destMs := destIlm.Metrics()
			destMs.EnsureCapacity(matchedMetricCount)
			srcMs := srcIlm.Metrics()

			// Iterate over Metrics which contain histograms
			for _, srcIdx := range metricIdxes {
				srcMetric := srcMs.At(srcIdx)

				// Copy metric properties to dest
				destMetric := destMs.AppendEmpty()
				srcMetric.CopyTo(destMetric)
				metricCount++
			}
		}
	}

	return dest, metricCount
}
