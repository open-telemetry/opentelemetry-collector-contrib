// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"

import (
	"sort"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

func normalizeTimestamps(metrics pmetric.Metrics) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		for j := 0; j < rms.At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < rms.At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				m := rms.At(i).ScopeMetrics().At(j).Metrics().At(k)
				//exhaustive:enforce
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					normalizeDataPointSlice(dataPointSlice[pmetric.NumberDataPoint](m.Gauge().DataPoints()))
				case pmetric.MetricTypeSum:
					normalizeDataPointSlice(dataPointSlice[pmetric.NumberDataPoint](m.Sum().DataPoints()))
				case pmetric.MetricTypeHistogram:
					normalizeDataPointSlice(dataPointSlice[pmetric.HistogramDataPoint](m.Histogram().DataPoints()))
				case pmetric.MetricTypeExponentialHistogram:
					normalizeDataPointSlice(dataPointSlice[pmetric.ExponentialHistogramDataPoint](m.ExponentialHistogram().DataPoints()))
				case pmetric.MetricTypeSummary:
					normalizeDataPointSlice(dataPointSlice[pmetric.SummaryDataPoint](m.Summary().DataPoints()))
				}
			}
		}
	}
}

// returns a map of the original timestamps with their corresponding normalized values.
// normalization entails setting nonunique subsequent timestamps to the same value while incrementing unique timestamps by a set value of 1,000,000 ns
func normalizeTimeSeries(timeSeries []pcommon.Timestamp) map[pcommon.Timestamp]pcommon.Timestamp {
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

	// normalize values
	normalizedTs := make(map[pcommon.Timestamp]pcommon.Timestamp)
	count := 0
	for _, v := range timeSeries {
		if v == 0 {
			continue
		}
		if _, ok := normalizedTs[v]; !ok {
			normalizedTs[v] = normalTime(count)
			count++
		}
	}

	return normalizedTs
}

func normalTime(timeSeriesIndex int) pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(time.Unix(0, 0).Add(time.Duration(timeSeriesIndex+1) * 1000000 * time.Nanosecond))
}

type dataPointSlice[T dataPoint] interface {
	Len() int
	At(i int) T
}

type dataPoint interface {
	pmetric.NumberDataPoint | pmetric.HistogramDataPoint | pmetric.ExponentialHistogramDataPoint | pmetric.SummaryDataPoint
	Attributes() pcommon.Map
	StartTimestamp() pcommon.Timestamp
	SetStartTimestamp(pcommon.Timestamp)
	Timestamp() pcommon.Timestamp
	SetTimestamp(pcommon.Timestamp)
}

func normalizeDataPointSlice[T dataPoint](dps dataPointSlice[T]) {
	attrCache := make(map[[16]byte]bool)
	for i := 0; i < dps.Len(); i++ {
		attrHash := pdatautil.MapHash(dps.At(i).Attributes())
		if attrCache[attrHash] {
			continue
		}
		timeSeries := []pcommon.Timestamp{dps.At(i).StartTimestamp(), dps.At(i).Timestamp()}

		// Find any other data points in the time series
		for j := i + 1; j < dps.Len(); j++ {
			if pdatautil.MapHash(dps.At(j).Attributes()) != attrHash {
				continue
			}
			timeSeries = append(timeSeries, dps.At(j).StartTimestamp(), dps.At(j).Timestamp())
		}

		normalizedTs := normalizeTimeSeries(timeSeries)
		for k := 0; k < dps.Len(); k++ {
			if pdatautil.MapHash(dps.At(k).Attributes()) != attrHash {
				continue
			}
			dps.At(k).SetTimestamp(normalizedTs[dps.At(k).Timestamp()])
			dps.At(k).SetStartTimestamp(normalizedTs[dps.At(k).StartTimestamp()])
		}
		attrCache[attrHash] = true
	}
}
