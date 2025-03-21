// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapointstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/datapointstorage"

import (
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// AttributeHash is used to store a hash of attributes for a metric. See pdatautil.MapHash for more details.
type AttributeHash [16]byte

// TimeseriesInfo contains the information necessary to adjust from the initial point and to detect resets.
type TimeseriesInfo struct {
	Mark bool

	Number               pmetric.NumberDataPoint
	Histogram            pmetric.HistogramDataPoint
	ExponentialHistogram pmetric.ExponentialHistogramDataPoint
	Summary              pmetric.SummaryDataPoint
}

type TimeseriesKey struct {
	Name           string
	Attributes     [16]byte
	AggTemporality pmetric.AggregationTemporality
}

// TimeseriesMap maps from a timeseries instance (metric * label values) to the timeseries info for
// the instance.
type TimeseriesMap struct {
	sync.RWMutex
	// The mutex is used to protect access to the member fields. It is acquired for the entirety of
	// AdjustMetricSlice() and also acquired by gc().

	Mark   bool
	TsiMap map[TimeseriesKey]*TimeseriesInfo
}

// Get the TimeseriesInfo for the timeseries associated with the metric and label values.
func (tsm *TimeseriesMap) Get(metric pmetric.Metric, kv pcommon.Map) (*TimeseriesInfo, bool) {
	// This should only be invoked be functions called (directly or indirectly) by AdjustMetricSlice().
	// The lock protecting tsm.tsiMap is acquired there.
	name := metric.Name()
	key := TimeseriesKey{
		Name:       name,
		Attributes: getAttributesSignature(kv),
	}
	switch metric.Type() {
	case pmetric.MetricTypeHistogram:
		// There are 2 types of Histograms whose aggregation temporality needs distinguishing:
		// * CumulativeHistogram
		// * GaugeHistogram
		key.AggTemporality = metric.Histogram().AggregationTemporality()
	case pmetric.MetricTypeExponentialHistogram:
		// There are 2 types of ExponentialHistograms whose aggregation temporality needs distinguishing:
		// * CumulativeHistogram
		// * GaugeHistogram
		key.AggTemporality = metric.ExponentialHistogram().AggregationTemporality()
	}

	tsm.Mark = true
	tsi, ok := tsm.TsiMap[key]
	if !ok {
		tsi = &TimeseriesInfo{}
		tsm.TsiMap[key] = tsi
	}
	tsi.Mark = true
	return tsi, ok
}

// Create a unique string signature for attributes values sorted by attribute keys.
func getAttributesSignature(m pcommon.Map) AttributeHash {
	// TODO(#38621): Investigate whether we should treat empty labels differently.
	clearedMap := pcommon.NewMap()
	for k, attrValue := range m.All() {
		value := attrValue.Str()
		if value != "" {
			clearedMap.PutStr(k, value)
		}
	}
	return pdatautil.MapHash(clearedMap)
}

// Remove timeseries that have aged out.
func (tsm *TimeseriesMap) GC() {
	tsm.Lock()
	defer tsm.Unlock()
	for ts, tsi := range tsm.TsiMap {
		if !tsi.Mark {
			delete(tsm.TsiMap, ts)
		} else {
			tsi.Mark = false
		}
	}
	tsm.Mark = false
}

// IsResetHistogram compares the given histogram datapoint h, to tsi.Histogram
// and determines whether the metric has been reset based on the values.  It is
// a reset if any of the bucket boundaries have changed, if any of the bucket
// counts have decreased or if the total sum or count have decreased.
func (tsi *TimeseriesInfo) IsResetHistogram(h pmetric.HistogramDataPoint) bool {
	if h.Count() < tsi.Histogram.Count() {
		return true
	}
	if h.Sum() < tsi.Histogram.Sum() {
		return true
	}

	// Guard against bucket boundaries changes.
	refBounds := tsi.Histogram.ExplicitBounds().AsRaw()
	hBounds := h.ExplicitBounds().AsRaw()
	if len(refBounds) != len(hBounds) {
		return true
	}
	for i := range len(refBounds) {
		if hBounds[i] != refBounds[i] {
			return true
		}
	}

	// We need to check individual buckets to make sure the counts are all increasing.
	if tsi.Histogram.BucketCounts().Len() != h.BucketCounts().Len() {
		return true
	}
	for i := range tsi.Histogram.BucketCounts().Len() {
		if h.BucketCounts().At(i) < tsi.Histogram.BucketCounts().At(i) {
			return true
		}
	}
	return false
}

// IsResetExponentialHistogram compares the given exponential histogram
// datapoint eh, to tsi.ExponentialHistogram and determines whether the metric
// has been reset based on the values.  It is a reset if any of the bucket
// boundaries have changed, if any of the bucket counts have decreased or if the
// total sum or count have decreased.
func (tsi *TimeseriesInfo) IsResetExponentialHistogram(eh pmetric.ExponentialHistogramDataPoint) bool {
	// Same as the histogram implementation
	if eh.Count() < tsi.ExponentialHistogram.Count() {
		return true
	}
	if eh.Sum() < tsi.ExponentialHistogram.Sum() {
		return true
	}

	// Guard against bucket boundaries changes.
	if tsi.ExponentialHistogram.Scale() != eh.Scale() {
		return true
	}

	// We need to check individual buckets to make sure the counts are all increasing.
	if tsi.ExponentialHistogram.Positive().BucketCounts().Len() != eh.Positive().BucketCounts().Len() {
		return true
	}
	for i := range tsi.ExponentialHistogram.Positive().BucketCounts().Len() {
		if eh.Positive().BucketCounts().At(i) < tsi.ExponentialHistogram.Positive().BucketCounts().At(i) {
			return true
		}
	}
	if tsi.ExponentialHistogram.Negative().BucketCounts().Len() != eh.Negative().BucketCounts().Len() {
		return true
	}
	for i := range tsi.ExponentialHistogram.Negative().BucketCounts().Len() {
		if eh.Negative().BucketCounts().At(i) < tsi.ExponentialHistogram.Negative().BucketCounts().At(i) {
			return true
		}
	}

	return false
}

// IsResetSummary compares the given summary datapoint s to tsi.Summary and
// determines whether the metric has been reset based on the values.  It is a
// reset if the count or sum has decreased.
func (tsi *TimeseriesInfo) IsResetSummary(s pmetric.SummaryDataPoint) bool {
	return s.Count() < tsi.Summary.Count() || s.Sum() < tsi.Summary.Sum()
}

// IsResetSum compares the given number datapoint s to tsi.Number and determines
// whether the metric has been reset based on the values.  It is a reset if the
// value has decreased.
func (tsi *TimeseriesInfo) IsResetSum(s pmetric.NumberDataPoint) bool {
	return s.DoubleValue() < tsi.Number.DoubleValue()
}

func newTimeseriesMap() *TimeseriesMap {
	return &TimeseriesMap{Mark: true, TsiMap: map[TimeseriesKey]*TimeseriesInfo{}}
}
