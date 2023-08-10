// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"

import (
	"sort"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

func normalizeTimestamps(metrics pmetric.Metrics) {
	zeroTime := time.Unix(0, 0)
	nanosec := pcommon.NewTimestampFromTime(zeroTime)
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		for j := 0; j < rms.At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < rms.At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				m := rms.At(i).ScopeMetrics().At(j).Metrics().At(k)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					for l := 0; l < m.Gauge().DataPoints().Len(); l++ {
						m.Gauge().DataPoints().At(l).SetStartTimestamp(nanosec)
					}
					normNumberDataPointSlice(m.Gauge().DataPoints())
				case pmetric.MetricTypeSum:
					for l := 0; l < m.Sum().DataPoints().Len(); l++ {
						m.Sum().DataPoints().At(l).SetStartTimestamp(nanosec)
					}
					normNumberDataPointSlice(m.Sum().DataPoints())
				case pmetric.MetricTypeHistogram:
					for l := 0; l < m.Histogram().DataPoints().Len(); l++ {
						m.Histogram().DataPoints().At(l).SetStartTimestamp(nanosec)
					}
					normHistogramDataPointSlice(m.Histogram().DataPoints())
				case pmetric.MetricTypeExponentialHistogram:
					for l := 0; l < m.ExponentialHistogram().DataPoints().Len(); l++ {
						m.ExponentialHistogram().DataPoints().At(l).SetStartTimestamp(nanosec)
					}
					normExponentialHistogramDataPointSlice(m.ExponentialHistogram().DataPoints())
				case pmetric.MetricTypeSummary:
					for l := 0; l < m.Summary().DataPoints().Len(); l++ {
						m.Summary().DataPoints().At(l).SetStartTimestamp(nanosec)
					}
					normSummaryDataPointSlice(m.Summary().DataPoints())
				}
			}
		}
	}
}

func normNumberDataPointSlice(ndps pmetric.NumberDataPointSlice) {
	attrCache := make(map[[16]byte]bool)
	for i := 0; i < ndps.Len(); i++ {
		attrHash := pdatautil.MapHash(ndps.At(i).Attributes())
		if attrCache[attrHash] {
			continue
		}
		timeSeries := []pcommon.Timestamp{ndps.At(i).Timestamp()}

		// Find any other data points in the time series
		for j := i + 1; j < ndps.Len(); j++ {
			if pdatautil.MapHash(ndps.At(j).Attributes()) != attrHash {
				continue
			}
			timeSeries = append(timeSeries, ndps.At(j).Timestamp())
		}

		normalizedTs := normalizeTimeSeries(timeSeries)
		for k := 0; k < ndps.Len(); k++ {
			if pdatautil.MapHash(ndps.At(k).Attributes()) != attrHash {
				continue
			}
			for key, value := range normalizedTs {
				if ndps.At(k).Timestamp() == key {
					ndps.At(k).SetTimestamp(value)
				}
			}
		}
		attrCache[attrHash] = true
	}
}

func normSummaryDataPointSlice(sdps pmetric.SummaryDataPointSlice) {
	attrCache := make(map[[16]byte]bool)
	for i := 0; i < sdps.Len(); i++ {
		attrHash := pdatautil.MapHash(sdps.At(i).Attributes())
		if attrCache[attrHash] {
			continue
		}
		timeSeries := []pcommon.Timestamp{sdps.At(i).Timestamp()}
		// Find any other data points in the time series
		for j := i + 1; j < sdps.Len(); j++ {
			if pdatautil.MapHash(sdps.At(j).Attributes()) != attrHash {
				continue
			}
			timeSeries = append(timeSeries, sdps.At(j).Timestamp())
		}

		normalizedTs := normalizeTimeSeries(timeSeries)
		for k := 0; k < sdps.Len(); k++ {
			if pdatautil.MapHash(sdps.At(k).Attributes()) != attrHash {
				continue
			}
			for key, value := range normalizedTs {
				if sdps.At(k).Timestamp() == key {
					sdps.At(k).SetTimestamp(value)
				}
			}
		}
		attrCache[attrHash] = true
	}
}

func normExponentialHistogramDataPointSlice(ehdps pmetric.ExponentialHistogramDataPointSlice) {
	attrCache := make(map[[16]byte]bool)
	for i := 0; i < ehdps.Len(); i++ {
		attrHash := pdatautil.MapHash(ehdps.At(i).Attributes())
		if attrCache[attrHash] {
			continue
		}
		timeSeries := []pcommon.Timestamp{ehdps.At(i).Timestamp()}
		// Find any other data points in the time series
		for j := i + 1; j < ehdps.Len(); j++ {
			if pdatautil.MapHash(ehdps.At(j).Attributes()) != attrHash {
				continue
			}
			timeSeries = append(timeSeries, ehdps.At(j).Timestamp())
		}

		normalizedTs := normalizeTimeSeries(timeSeries)
		for k := 0; k < ehdps.Len(); k++ {
			if pdatautil.MapHash(ehdps.At(k).Attributes()) != attrHash {
				continue
			}
			for key, value := range normalizedTs {
				if ehdps.At(k).Timestamp() == key {
					ehdps.At(k).SetTimestamp(value)
				}
			}
		}
		attrCache[attrHash] = true
	}
}

func normHistogramDataPointSlice(hdps pmetric.HistogramDataPointSlice) {
	attrCache := make(map[[16]byte]bool)
	for i := 0; i < hdps.Len(); i++ {
		attrHash := pdatautil.MapHash(hdps.At(i).Attributes())
		if attrCache[attrHash] {
			continue
		}
		timeSeries := []pcommon.Timestamp{hdps.At(i).Timestamp()}
		// Find any other data points in the time series
		for j := i + 1; j < hdps.Len(); j++ {
			if pdatautil.MapHash(hdps.At(j).Attributes()) != attrHash {
				continue
			}
			timeSeries = append(timeSeries, hdps.At(j).Timestamp())
		}

		normalizedTs := normalizeTimeSeries(timeSeries)
		for k := 0; k < hdps.Len(); k++ {
			if pdatautil.MapHash(hdps.At(k).Attributes()) != attrHash {
				continue
			}
			for key, value := range normalizedTs {
				if hdps.At(k).Timestamp() == key {
					hdps.At(k).SetTimestamp(value)
				}
			}
		}
		attrCache[attrHash] = true
	}
}

// returns a map of the original timestamps with their corresponding normalized values.
// normalization entails setting nonunique subsequent timestamps to the same value while incrementing unique timestamps by a set value of 1,000,000 ns
func normalizeTimeSeries(timeSeries []pcommon.Timestamp) map[pcommon.Timestamp]pcommon.Timestamp {
	normalizedTs := make(map[pcommon.Timestamp]pcommon.Timestamp)
	sort.Slice(timeSeries, func(i, j int) bool {
		return func(t1, t2 pcommon.Timestamp) int {
			if t1 < t2 {
				return -1
			} else if t1 > t2 {
				return 1
			}
			return 0
		}(timeSeries[i], timeSeries[j]) < 0
	})

	for i := range timeSeries {
		normalizedTs[timeSeries[i]] = normalTime(i)
	}
	return normalizedTs
}

func normalTime(timeSeriesIndex int) pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(time.Unix(0, 0).Add(time.Duration(timeSeriesIndex+1) * 1000000 * time.Nanosecond))
}
