// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

type metricFamily struct {
	mtype pmetric.MetricType
	// isMonotonic only applies to sums
	isMonotonic bool
	groups      map[uint64]*metricGroup
	name        string
	metadata    *scrape.MetricMetadata
	groupOrders []*metricGroup
}

// metricGroup, represents a single metric of a metric family. for example a histogram metric is usually represent by
// a couple data complexValue (buckets and count/sum), a group of a metric family always share a same set of tags. for
// simple types like counter and gauge, each data point is a group of itself
type metricGroup struct {
	mtype    pmetric.MetricType
	ts       int64
	ls       labels.Labels
	count    float64
	hasCount bool
	sum      float64
	hasSum   bool
	// This corresponds to the `_created` sample found from the metric parsing.
	// - https://github.com/prometheus/OpenMetrics/blob/main/specification/OpenMetrics.md#timestamps
	// - https://github.com/prometheus/OpenMetrics/blob/main/specification/OpenMetrics.md#counter-1
	createdSeconds float64
	value          float64
	hValue         *histogram.Histogram
	fhValue        *histogram.FloatHistogram
	complexValue   []*dataPoint
	exemplars      pmetric.ExemplarSlice
}

func newMetricFamily(metricName string, mc scrape.MetricMetadataStore, logger *zap.Logger) *metricFamily {
	metadata, familyName := metadataForMetric(metricName, mc)
	mtype, isMonotonic := convToMetricType(metadata.Type)
	if mtype == pmetric.MetricTypeEmpty {
		logger.Debug(fmt.Sprintf("Unknown-typed metric : %s %+v", metricName, metadata))
	}

	return &metricFamily{
		mtype:       mtype,
		isMonotonic: isMonotonic,
		groups:      make(map[uint64]*metricGroup),
		name:        familyName,
		metadata:    metadata,
	}
}

// includesMetric returns true if the metric is part of the family
func (mf *metricFamily) includesMetric(metricName string) bool {
	if mf.mtype != pmetric.MetricTypeGauge {
		// If it is a merged family type, then it should match the
		// family name when suffixes are trimmed.
		return normalizeMetricName(metricName) == mf.name
	}
	// If it isn't a merged type, the metricName and family name should match
	return metricName == mf.name
}

func (mg *metricGroup) sortPoints() {
	sort.Slice(mg.complexValue, func(i, j int) bool {
		return mg.complexValue[i].boundary < mg.complexValue[j].boundary
	})
}

func (mg *metricGroup) toDistributionPoint(dest pmetric.HistogramDataPointSlice) {
	if !mg.hasCount {
		return
	}

	mg.sortPoints()

	bucketCount := len(mg.complexValue) + 1
	// if the final bucket is +Inf, we ignore it
	if bucketCount > 1 && mg.complexValue[bucketCount-2].boundary == math.Inf(1) {
		bucketCount--
	}

	// for OTLP the bounds won't include +inf
	bounds := make([]float64, bucketCount-1)
	bucketCounts := make([]uint64, bucketCount)
	var adjustedCount float64

	pointIsStale := value.IsStaleNaN(mg.sum) || value.IsStaleNaN(mg.count)
	for i := 0; i < bucketCount-1; i++ {
		bounds[i] = mg.complexValue[i].boundary
		adjustedCount = mg.complexValue[i].value

		// Buckets still need to be sent to know to set them as stale,
		// but a staleness NaN converted to uint64 would be an extremely large number.
		// Setting to 0 instead.
		if pointIsStale {
			adjustedCount = 0
		} else if i != 0 {
			adjustedCount -= mg.complexValue[i-1].value
		}
		bucketCounts[i] = uint64(adjustedCount)
	}

	// Add the final bucket based on the total count
	adjustedCount = mg.count
	if pointIsStale {
		adjustedCount = 0
	} else if bucketCount > 1 {
		adjustedCount -= mg.complexValue[bucketCount-2].value
	}
	bucketCounts[bucketCount-1] = uint64(adjustedCount)

	point := dest.AppendEmpty()

	if pointIsStale {
		point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	} else {
		point.SetCount(uint64(mg.count))
		if mg.hasSum {
			point.SetSum(mg.sum)
		}
	}

	point.ExplicitBounds().FromRaw(bounds)
	point.BucketCounts().FromRaw(bucketCounts)

	// The timestamp MUST be in retrieved from milliseconds and converted to nanoseconds.
	tsNanos := timestampFromMs(mg.ts)
	if mg.createdSeconds != 0 {
		point.SetStartTimestamp(timestampFromFloat64(mg.createdSeconds))
	} else if !removeStartTimeAdjustment.IsEnabled() {
		// metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
		point.SetStartTimestamp(tsNanos)
	}
	point.SetTimestamp(tsNanos)
	populateAttributes(pmetric.MetricTypeHistogram, mg.ls, point.Attributes())
	mg.setExemplars(point.Exemplars())
}

// toExponentialHistogramDataPoints is based on
// https://opentelemetry.io/docs/specs/otel/compatibility/prometheus_and_openmetrics/#exponential-histograms
func (mg *metricGroup) toExponentialHistogramDataPoints(dest pmetric.ExponentialHistogramDataPointSlice) {
	if !mg.hasCount {
		return
	}
	point := dest.AppendEmpty()
	point.SetTimestamp(timestampFromMs(mg.ts))

	// We do not set Min or Max as native histograms don't have that information.
	switch {
	case mg.fhValue != nil:
		fh := mg.fhValue

		if value.IsStaleNaN(fh.Sum) {
			point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			// The count and sum are initialized to 0, so we don't need to set them.
		} else {
			point.SetScale(fh.Schema)
			// Input is a float native histogram. This conversion will lose
			// precision,but we don't actually expect float histograms in scrape,
			// since these are typically the result of operations on integer
			// native histograms in the database.
			point.SetCount(uint64(fh.Count))
			point.SetSum(fh.Sum)
			point.SetZeroThreshold(fh.ZeroThreshold)
			point.SetZeroCount(uint64(fh.ZeroCount))

			if len(fh.PositiveSpans) > 0 {
				point.Positive().SetOffset(fh.PositiveSpans[0].Offset - 1) // -1 because OTEL offset are for the lower bound, not the upper bound
				convertAbsoluteBuckets(fh.PositiveSpans, fh.PositiveBuckets, point.Positive().BucketCounts())
			}
			if len(fh.NegativeSpans) > 0 {
				point.Negative().SetOffset(fh.NegativeSpans[0].Offset - 1) // -1 because OTEL offset are for the lower bound, not the upper bound
				convertAbsoluteBuckets(fh.NegativeSpans, fh.NegativeBuckets, point.Negative().BucketCounts())
			}
		}

	case mg.hValue != nil:
		h := mg.hValue

		if value.IsStaleNaN(h.Sum) {
			point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			// The count and sum are initialized to 0, so we don't need to set them.
		} else {
			point.SetScale(h.Schema)
			point.SetCount(h.Count)
			point.SetSum(h.Sum)
			point.SetZeroThreshold(h.ZeroThreshold)
			point.SetZeroCount(h.ZeroCount)

			if len(h.PositiveSpans) > 0 {
				point.Positive().SetOffset(h.PositiveSpans[0].Offset - 1) // -1 because OTEL offset are for the lower bound, not the upper bound
				convertDeltaBuckets(h.PositiveSpans, h.PositiveBuckets, point.Positive().BucketCounts())
			}
			if len(h.NegativeSpans) > 0 {
				point.Negative().SetOffset(h.NegativeSpans[0].Offset - 1) // -1 because OTEL offset are for the lower bound, not the upper bound
				convertDeltaBuckets(h.NegativeSpans, h.NegativeBuckets, point.Negative().BucketCounts())
			}
		}

	default:
		// This should never happen.
		return
	}

	tsNanos := timestampFromMs(mg.ts)
	if mg.createdSeconds != 0 {
		point.SetStartTimestamp(timestampFromFloat64(mg.createdSeconds))
	} else if !removeStartTimeAdjustment.IsEnabled() {
		// metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
		point.SetStartTimestamp(tsNanos)
	}
	point.SetTimestamp(tsNanos)
	populateAttributes(pmetric.MetricTypeHistogram, mg.ls, point.Attributes())
	mg.setExemplars(point.Exemplars())
}

func convertDeltaBuckets(spans []histogram.Span, deltas []int64, buckets pcommon.UInt64Slice) {
	buckets.EnsureCapacity(len(deltas))
	bucketIdx := 0
	bucketCount := int64(0)
	for spanIdx, span := range spans {
		if spanIdx > 0 {
			for i := int32(0); i < span.Offset; i++ {
				buckets.Append(uint64(0))
			}
		}
		for i := uint32(0); i < span.Length; i++ {
			bucketCount += deltas[bucketIdx]
			bucketIdx++
			buckets.Append(uint64(bucketCount))
		}
	}
}

func convertAbsoluteBuckets(spans []histogram.Span, counts []float64, buckets pcommon.UInt64Slice) {
	buckets.EnsureCapacity(len(counts))
	bucketIdx := 0
	for spanIdx, span := range spans {
		if spanIdx > 0 {
			for i := int32(0); i < span.Offset; i++ {
				buckets.Append(uint64(0))
			}
		}
		for i := uint32(0); i < span.Length; i++ {
			buckets.Append(uint64(counts[bucketIdx]))
			bucketIdx++
		}
	}
}

func (mg *metricGroup) setExemplars(exemplars pmetric.ExemplarSlice) {
	if mg == nil {
		return
	}
	if mg.exemplars.Len() > 0 {
		mg.exemplars.MoveAndAppendTo(exemplars)
	}
}

func (mg *metricGroup) toSummaryPoint(dest pmetric.SummaryDataPointSlice) {
	// expecting count to be provided, however, in the following two cases, they can be missed.
	// 1. data is corrupted
	// 2. ignored by startValue evaluation
	if !mg.hasCount {
		return
	}

	mg.sortPoints()

	point := dest.AppendEmpty()
	pointIsStale := value.IsStaleNaN(mg.sum) || value.IsStaleNaN(mg.count)
	if pointIsStale {
		point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	} else {
		if mg.hasSum {
			point.SetSum(mg.sum)
		}
		point.SetCount(uint64(mg.count))
	}

	quantileValues := point.QuantileValues()
	for _, p := range mg.complexValue {
		quantile := quantileValues.AppendEmpty()
		// Quantiles still need to be sent to know to set them as stale,
		// but a staleness NaN converted to uint64 would be an extremely large number.
		// By not setting the quantile value, it will default to 0.
		if !pointIsStale {
			quantile.SetValue(p.value)
		}
		quantile.SetQuantile(p.boundary)
	}

	// Based on the summary description from https://prometheus.io/docs/concepts/metric_types/#summary
	// the quantiles are calculated over a sliding time window, however, the count is the total count of
	// observations and the corresponding sum is a sum of all observed values, thus the sum and count used
	// at the global level of the metricspb.SummaryValue
	// The timestamp MUST be in retrieved from milliseconds and converted to nanoseconds.
	tsNanos := timestampFromMs(mg.ts)
	point.SetTimestamp(tsNanos)
	if mg.createdSeconds != 0 {
		point.SetStartTimestamp(timestampFromFloat64(mg.createdSeconds))
	} else if !removeStartTimeAdjustment.IsEnabled() {
		// metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
		point.SetStartTimestamp(tsNanos)
	}
	populateAttributes(pmetric.MetricTypeSummary, mg.ls, point.Attributes())
}

func (mg *metricGroup) toNumberDataPoint(dest pmetric.NumberDataPointSlice) {
	tsNanos := timestampFromMs(mg.ts)
	point := dest.AppendEmpty()
	// gauge/undefined types have no start time.
	if mg.mtype == pmetric.MetricTypeSum {
		if mg.createdSeconds != 0 {
			point.SetStartTimestamp(timestampFromFloat64(mg.createdSeconds))
		} else if !removeStartTimeAdjustment.IsEnabled() {
			// metrics_adjuster adjusts the startTimestamp to the initial scrape timestamp
			point.SetStartTimestamp(tsNanos)
		}
	}
	point.SetTimestamp(tsNanos)
	if value.IsStaleNaN(mg.value) {
		point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	} else {
		point.SetDoubleValue(mg.value)
	}
	populateAttributes(pmetric.MetricTypeGauge, mg.ls, point.Attributes())
	mg.setExemplars(point.Exemplars())
}

func populateAttributes(mType pmetric.MetricType, ls labels.Labels, dest pcommon.Map) {
	dest.EnsureCapacity(ls.Len())
	names := getSortedNotUsefulLabels(mType)
	j := 0
	ls.Range(func(l labels.Label) {
		for j < len(names) && names[j] < l.Name {
			j++
		}
		if j < len(names) && l.Name == names[j] {
			return
		}
		if l.Value == "" {
			// empty label values should be omitted
			return
		}
		dest.PutStr(l.Name, l.Value)
	})
}

func (mf *metricFamily) loadMetricGroupOrCreate(groupKey uint64, ls labels.Labels, ts int64) *metricGroup {
	mg, ok := mf.groups[groupKey]
	if !ok {
		mg = &metricGroup{
			mtype:     mf.mtype,
			ts:        ts,
			ls:        ls,
			exemplars: pmetric.NewExemplarSlice(),
		}
		mf.groups[groupKey] = mg
		// maintaining data insertion order is helpful to generate stable/reproducible metric output
		mf.groupOrders = append(mf.groupOrders, mg)
	}
	return mg
}

func (mf *metricFamily) addSeries(seriesRef uint64, metricName string, ls labels.Labels, t int64, v float64) error {
	mg := mf.loadMetricGroupOrCreate(seriesRef, ls, t)
	if mg.ts != t {
		return fmt.Errorf("inconsistent timestamps on metric points for metric %v", metricName)
	}
	switch mf.mtype {
	case pmetric.MetricTypeHistogram, pmetric.MetricTypeSummary:
		switch {
		case strings.HasSuffix(metricName, metricsSuffixSum):
			mg.sum = v
			mg.hasSum = true
		case strings.HasSuffix(metricName, metricsSuffixCount):
			// always use the timestamp from count, because is the only required field for histograms and summaries.
			mg.ts = t
			mg.count = v
			mg.hasCount = true
		case metricName == mf.metadata.MetricFamily+metricSuffixCreated:
			mg.createdSeconds = v
		default:
			boundary, err := getBoundary(mf.mtype, ls)
			if err != nil {
				return err
			}
			mg.complexValue = append(mg.complexValue, &dataPoint{value: v, boundary: boundary})
		}
	case pmetric.MetricTypeExponentialHistogram:
		if metricName == mf.metadata.MetricFamily+metricSuffixCreated {
			mg.createdSeconds = v
		}
	case pmetric.MetricTypeSum:
		if metricName == mf.metadata.MetricFamily+metricSuffixCreated {
			mg.createdSeconds = v
		} else {
			mg.value = v
		}
	case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge:
		fallthrough
	default:
		mg.value = v
	}

	return nil
}

// addCreationTimestamp updates the metric group cache with the created timestamp for the group.
// The parser gets the created time in ms however and we must convert it to seconds here.
// - https://github.com/prometheus/prometheus/blob/2bf6f4c9dcbb1ad2e8fef70c6a48d8fc44a7f57c/model/textparse/interface.go#L77-L80
func (mf *metricFamily) addCreationTimestamp(seriesRef uint64, ls labels.Labels, atMs, ctMs int64) {
	mg := mf.loadMetricGroupOrCreate(seriesRef, ls, atMs)
	mg.createdSeconds = float64(ctMs) / 1000.0
}

func (mf *metricFamily) addExponentialHistogramSeries(seriesRef uint64, metricName string, ls labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) error {
	mg := mf.loadMetricGroupOrCreate(seriesRef, ls, t)
	if mg.ts != t {
		return fmt.Errorf("inconsistent timestamps on metric points for metric %v", metricName)
	}
	if mg.mtype != pmetric.MetricTypeExponentialHistogram {
		return fmt.Errorf("metric type mismatch for exponential histogram metric %v type %s", metricName, mg.mtype.String())
	}
	switch {
	case fh != nil:
		if mg.hValue != nil {
			return fmt.Errorf("exponential histogram %v already has float counts", metricName)
		}
		mg.count = fh.Count
		mg.sum = fh.Sum
		mg.hasCount = true
		mg.hasSum = true
		mg.fhValue = fh
	case h != nil:
		if mg.fhValue != nil {
			return fmt.Errorf("exponential histogram %v already has integer counts", metricName)
		}
		mg.count = float64(h.Count)
		mg.sum = h.Sum
		mg.hasCount = true
		mg.hasSum = true
		mg.hValue = h
	}
	return nil
}

func (mf *metricFamily) appendMetric(metrics pmetric.MetricSlice, trimSuffixes bool) {
	metric := pmetric.NewMetric()
	// Trims type and unit suffixes from metric name
	name := mf.name
	if trimSuffixes {
		name = prometheus.TrimPromSuffixes(name, mf.mtype, mf.metadata.Unit)
	}
	metric.SetName(name)
	metric.SetDescription(mf.metadata.Help)
	metric.SetUnit(prometheus.UnitWordToUCUM(mf.metadata.Unit))
	metric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, string(mf.metadata.Type))

	var pointCount int

	switch mf.mtype {
	case pmetric.MetricTypeHistogram:
		histogram := metric.SetEmptyHistogram()
		histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		hdpL := histogram.DataPoints()
		for _, mg := range mf.groupOrders {
			mg.toDistributionPoint(hdpL)
		}
		pointCount = hdpL.Len()

	case pmetric.MetricTypeSummary:
		summary := metric.SetEmptySummary()
		sdpL := summary.DataPoints()
		for _, mg := range mf.groupOrders {
			mg.toSummaryPoint(sdpL)
		}
		pointCount = sdpL.Len()

	case pmetric.MetricTypeSum:
		sum := metric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(mf.isMonotonic)
		sdpL := sum.DataPoints()
		for _, mg := range mf.groupOrders {
			mg.toNumberDataPoint(sdpL)
		}
		pointCount = sdpL.Len()

	case pmetric.MetricTypeExponentialHistogram:
		histogram := metric.SetEmptyExponentialHistogram()
		histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		hdpL := histogram.DataPoints()
		for _, mg := range mf.groupOrders {
			mg.toExponentialHistogramDataPoints(hdpL)
		}
		pointCount = hdpL.Len()

	case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge:
		fallthrough
	default: // Everything else should be set to a Gauge.
		gauge := metric.SetEmptyGauge()
		gdpL := gauge.DataPoints()
		for _, mg := range mf.groupOrders {
			mg.toNumberDataPoint(gdpL)
		}
		pointCount = gdpL.Len()
	}

	if pointCount == 0 {
		return
	}

	metric.MoveTo(metrics.AppendEmpty())
}

func (mf *metricFamily) addExemplar(seriesRef uint64, e exemplar.Exemplar) {
	mg := mf.groups[seriesRef]
	if mg == nil {
		return
	}
	es := mg.exemplars
	convertExemplar(e, es.AppendEmpty())
}

func convertExemplar(pe exemplar.Exemplar, e pmetric.Exemplar) {
	e.SetTimestamp(timestampFromMs(pe.Ts))
	e.SetDoubleValue(pe.Value)
	e.FilteredAttributes().EnsureCapacity(pe.Labels.Len())
	pe.Labels.Range(func(lb labels.Label) {
		switch strings.ToLower(lb.Name) {
		case prometheus.ExemplarTraceIDKey:
			var tid [16]byte
			err := decodeAndCopyToLowerBytes(tid[:], []byte(lb.Value))
			if err == nil {
				e.SetTraceID(tid)
			} else {
				e.FilteredAttributes().PutStr(lb.Name, lb.Value)
			}
		case prometheus.ExemplarSpanIDKey:
			var sid [8]byte
			err := decodeAndCopyToLowerBytes(sid[:], []byte(lb.Value))
			if err == nil {
				e.SetSpanID(sid)
			} else {
				e.FilteredAttributes().PutStr(lb.Name, lb.Value)
			}
		default:
			e.FilteredAttributes().PutStr(lb.Name, lb.Value)
		}
	})
}

/*
	decodeAndCopyToLowerBytes copies src to dst on lower bytes instead of higher

1. If len(src) > len(dst) -> copy first len(dst) bytes as it is. Example -> src = []byte{0xab,0xcd,0xef,0xgh,0xij}, dst = [2]byte, result dst = [2]byte{0xab, 0xcd}
2. If len(src) = len(dst) -> copy src to dst as it is
3. If len(src) < len(dst) -> prepend required 0s and then add src to dst. Example -> src = []byte{0xab, 0xcd}, dst = [8]byte, result dst = [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xab, 0xcd}
*/
func decodeAndCopyToLowerBytes(dst []byte, src []byte) error {
	var err error
	decodedLen := hex.DecodedLen(len(src))
	if decodedLen >= len(dst) {
		_, err = hex.Decode(dst, src[:hex.EncodedLen(len(dst))])
	} else {
		_, err = hex.Decode(dst[len(dst)-decodedLen:], src)
	}
	return err
}
