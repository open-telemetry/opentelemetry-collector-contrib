// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapointstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/datapointstorage"

import (
	"math"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/xpdata/xhash"
)

// AttributeHash is used to store a hash of attributes for a metric. See xhash.MapHash for more details.
type AttributeHash [16]byte

// TimeseriesInfo contains the information necessary to adjust from the initial point and to detect resets.
type TimeseriesInfo struct {
	Mark bool

	Number               NumberInfo
	Histogram            HistogramInfo
	ExponentialHistogram ExponentialHistogramInfo
	Summary              SummaryInfo
}

type NumberInfo struct {
	StartTime           pcommon.Timestamp
	PreviousDoubleValue float64
	PreviousIntValue    int64

	// These are the optional reference values for strategies that need to cache
	// additional data from the initial points.
	// For example - storing the initial point for the subtract_initial_point strategy.
	RefDoubleValue float64
	RefIntValue    int64
}

type HistogramInfo struct {
	StartTime            pcommon.Timestamp
	PreviousCount        uint64
	PreviousSum          float64
	PreviousBucketCounts []uint64
	ExplicitBounds       []float64

	// These are the optional reference values for strategies that need to cache
	// additional data from the initial points.
	// For example - storing the initial point for the subtract_initial_point strategy.
	RefCount        uint64
	RefSum          float64
	RefBucketCounts []uint64
}

type ExponentialHistogramInfo struct {
	StartTime             pcommon.Timestamp
	PreviousCount         uint64
	PreviousSum           float64
	PreviousZeroCount     uint64
	PreviousZeroThreshold float64
	Scale                 int32
	PreviousPositive      ExponentialHistogramBucketInfo
	PreviousNegative      ExponentialHistogramBucketInfo

	// These are the optional reference values for strategies that need to cache
	// additional data from the initial points.
	// For example - storing the initial point for the subtract_initial_point strategy.
	RefCount     uint64
	RefSum       float64
	RefZeroCount uint64
	RefPositive  ExponentialHistogramBucketInfo
	RefNegative  ExponentialHistogramBucketInfo
}

type ExponentialHistogramBucketInfo struct {
	Offset       int32
	BucketCounts []uint64
}

func NewExponentialHistogramBucketInfo(ehdpb pmetric.ExponentialHistogramDataPointBuckets) ExponentialHistogramBucketInfo {
	return ExponentialHistogramBucketInfo{
		Offset:       ehdpb.Offset(),
		BucketCounts: ehdpb.BucketCounts().AsRaw(),
	}
}

// Downscale reduces an exponential histogram's scale by bitsLost,
// which amounts to dividing bucket indices by 2**bitsLost and merging buckets with the same resulting index.
func (buckets ExponentialHistogramBucketInfo) Downscale(bitsLost uint32) (out ExponentialHistogramBucketInfo) {
	if len(buckets.BucketCounts) == 0 || bitsLost == 0 {
		return buckets
	}
	out.Offset = buckets.Offset >> bitsLost
	maxBucket := int64(buckets.Offset) + int64(len(buckets.BucketCounts)) - 1
	outLen := int((maxBucket >> bitsLost) - int64(out.Offset) + 1)
	out.BucketCounts = make([]uint64, outLen)
	for index, bucketCount := range buckets.BucketCounts {
		newIndex := int(((int64(buckets.Offset) + int64(index)) >> bitsLost) - int64(out.Offset))
		out.BucketCounts[newIndex] += bucketCount
	}
	return out
}

// TrimZeros removes buckets at or below thresholdBucket, returning the sum of their counts.
// This is used when increasing the zero threshold: the removed count is added to the zero count.
func (buckets *ExponentialHistogramBucketInfo) TrimZeros(thresholdBucket int32) uint64 {
	thresholdIndex := int64(thresholdBucket) - int64(buckets.Offset)
	if thresholdIndex < 0 {
		return 0
	}
	trimmed := min(thresholdIndex+1, int64(len(buckets.BucketCounts)))
	var zeroCount uint64
	for _, bucketCount := range buckets.BucketCounts[:trimmed] {
		zeroCount += bucketCount
	}
	buckets.BucketCounts = buckets.BucketCounts[trimmed:]
	if len(buckets.BucketCounts) == 0 {
		buckets.BucketCounts = nil
		buckets.Offset = 0
	} else {
		buckets.Offset = int32(int64(buckets.Offset) + trimmed)
	}
	return zeroCount
}

// UpdatePrevious updates the previous-point tracking fields from dp.
func (eh *ExponentialHistogramInfo) UpdatePrevious(dp pmetric.ExponentialHistogramDataPoint) {
	eh.Scale = dp.Scale()
	eh.PreviousZeroThreshold = dp.ZeroThreshold()
	eh.PreviousPositive = NewExponentialHistogramBucketInfo(dp.Positive())
	eh.PreviousNegative = NewExponentialHistogramBucketInfo(dp.Negative())
	eh.PreviousCount, eh.PreviousSum, eh.PreviousZeroCount = dp.Count(), dp.Sum(), dp.ZeroCount()
}

// DownscaleRef adjusts the cached reference buckets and zero count when the
// incoming datapoint has a lower scale or a higher zero threshold.
func (eh *ExponentialHistogramInfo) DownscaleRef(newScale int32, newZeroThreshold float64) {
	eh.RefPositive, eh.RefNegative, eh.RefZeroCount = downscaleBuckets(
		eh.RefPositive,
		eh.RefNegative,
		eh.RefZeroCount,
		eh.Scale,
		eh.PreviousZeroThreshold,
		newScale,
		newZeroThreshold,
	)
}

func thresholdBucketForZeroThreshold(zeroThreshold float64, scale int32) int32 {
	val := math.Ceil(math.Ldexp(math.Log2(zeroThreshold), int(scale))) - 1
	if math.IsNaN(val) || val <= float64(math.MinInt32) {
		return math.MinInt32
	}
	if val >= float64(math.MaxInt32) {
		return math.MaxInt32
	}
	return int32(val)
}

func downscaleBuckets(
	pos, neg ExponentialHistogramBucketInfo,
	zeroCount uint64,
	fromScale int32,
	fromZeroThreshold float64,
	toScale int32,
	toZeroThreshold float64,
) (ExponentialHistogramBucketInfo, ExponentialHistogramBucketInfo, uint64) {
	if toZeroThreshold > fromZeroThreshold && toZeroThreshold > 0 {
		thresholdBucket := thresholdBucketForZeroThreshold(toZeroThreshold, fromScale)
		zeroCount += pos.TrimZeros(thresholdBucket)
		zeroCount += neg.TrimZeros(thresholdBucket)
	}
	if toScale < fromScale {
		bitsLost := uint32(int64(fromScale) - int64(toScale))
		pos = pos.Downscale(bitsLost)
		neg = neg.Downscale(bitsLost)
	}
	return pos, neg, zeroCount
}

type SummaryInfo struct {
	StartTime     pcommon.Timestamp
	PreviousCount uint64
	PreviousSum   float64

	// These are the optional reference values for strategies that need to cache
	// additional data from the initial points.
	// For example - storing the initial point for the subtract_initial_point strategy.
	RefCount uint64
	RefSum   float64
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
		Attributes: xhash.MapHash(kv),
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

// IsResetHistogram compares the given histogram datapoint h, to ref
// and determines whether the metric has been reset based on the values.  It is
// a reset if any of the bucket boundaries have changed, if any of the bucket
// counts have decreased or if the total sum or count have decreased.
func (ref *TimeseriesInfo) IsResetHistogram(h pmetric.HistogramDataPoint) bool {
	if h.Count() < ref.Histogram.PreviousCount {
		return true
	}
	if h.Sum() < ref.Histogram.PreviousSum {
		return true
	}

	// Guard against bucket boundaries changes.
	refBounds := ref.Histogram.ExplicitBounds
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
	if len(ref.Histogram.PreviousBucketCounts) != h.BucketCounts().Len() {
		return true
	}
	for i := range len(ref.Histogram.PreviousBucketCounts) {
		if h.BucketCounts().At(i) < ref.Histogram.PreviousBucketCounts[i] {
			return true
		}
	}
	return false
}

// IsResetExponentialHistogram compares the given exponential histogram
// datapoint eh to ref and determines whether the metric has been reset. It is
// a reset if total count, sum, zero count, or zero threshold has decreased, if
// scale has increased, or if any bucket count has decreased after accounting for
// zero-threshold trimming, scale downscaling, and bucket offsets.
func (ref *TimeseriesInfo) IsResetExponentialHistogram(eh pmetric.ExponentialHistogramDataPoint) bool {
	if eh.Count() < ref.ExponentialHistogram.PreviousCount {
		return true
	}
	if eh.Sum() < ref.ExponentialHistogram.PreviousSum {
		return true
	}
	if eh.ZeroCount() < ref.ExponentialHistogram.PreviousZeroCount {
		return true
	}
	if eh.ZeroThreshold() < ref.ExponentialHistogram.PreviousZeroThreshold {
		return true
	}
	if eh.Scale() > ref.ExponentialHistogram.Scale {
		return true
	}

	prevPos, prevNeg, prevZeroCount := downscaleBuckets(
		ref.ExponentialHistogram.PreviousPositive,
		ref.ExponentialHistogram.PreviousNegative,
		ref.ExponentialHistogram.PreviousZeroCount,
		ref.ExponentialHistogram.Scale,
		ref.ExponentialHistogram.PreviousZeroThreshold,
		eh.Scale(),
		eh.ZeroThreshold(),
	)
	if eh.ZeroCount() < prevZeroCount {
		return true
	}

	return isResetExponentialBuckets(eh.Positive(), prevPos) || isResetExponentialBuckets(eh.Negative(), prevNeg)
}

func isResetExponentialBuckets(cur pmetric.ExponentialHistogramDataPointBuckets, prev ExponentialHistogramBucketInfo) bool {
	curOffset := int64(cur.Offset())
	curLen := int64(cur.BucketCounts().Len())
	prevOffset := int64(prev.Offset)
	prevLen := int64(len(prev.BucketCounts))

	if curOffset > prevOffset {
		preShiftLen := min(curOffset-prevOffset, prevLen)
		for i := range preShiftLen {
			if prev.BucketCounts[i] > 0 {
				return true
			}
		}
	}

	for i := range curLen {
		prevIndex := (curOffset + i) - prevOffset
		if prevIndex >= 0 && prevIndex < prevLen {
			if cur.BucketCounts().At(int(i)) < prev.BucketCounts[prevIndex] {
				return true
			}
		}
	}

	curEnd := curOffset + curLen
	prevEnd := prevOffset + prevLen
	if prevEnd > curEnd {
		postStart := max(int64(0), curEnd-prevOffset)
		for i := postStart; i < prevLen; i++ {
			if prev.BucketCounts[i] > 0 {
				return true
			}
		}
	}

	return false
}

// IsResetSummary compares the given summary datapoint s to ref and
// determines whether the metric has been reset based on the values.  It is a
// reset if the count or sum has decreased.
func (ref *TimeseriesInfo) IsResetSummary(s pmetric.SummaryDataPoint) bool {
	return s.Count() < ref.Summary.PreviousCount || s.Sum() < ref.Summary.PreviousSum
}

// IsResetSum compares the given number datapoint s to ref and determines
// whether the metric has been reset based on the values.  It is a reset if the
// value has decreased.
func (ref *TimeseriesInfo) IsResetSum(s pmetric.NumberDataPoint) bool {
	return s.DoubleValue() < ref.Number.PreviousDoubleValue || s.IntValue() < ref.Number.PreviousIntValue
}

func newTimeseriesMap() *TimeseriesMap {
	return &TimeseriesMap{Mark: true, TsiMap: map[TimeseriesKey]*TimeseriesInfo{}}
}
