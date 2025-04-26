// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"

import (
	"sort"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// https://github.com/open-telemetry/opentelemetry-go/blob/3ae002c3caf3e44387f0554dfcbbde2c5aab7909/sdk/metric/internal/aggregate/limit.go#L11C36-L11C50
const overflowKey = "otel.metric.overflow"

type Key string

type HistogramMetrics interface {
	GetOrCreate(key Key, attributesFun BuildAttributesFun, startTimestamp pcommon.Timestamp) (Histogram, bool)
	BuildMetrics(pmetric.Metric, pcommon.Timestamp, func(Key, pcommon.Timestamp) pcommon.Timestamp, pmetric.AggregationTemporality)
	ClearExemplars()
}

type Histogram interface {
	Observe(value float64)
	AddExemplar(traceID pcommon.TraceID, spanID pcommon.SpanID, value float64)
}

type explicitHistogramMetrics struct {
	metrics          map[Key]*explicitHistogram
	bounds           []float64
	maxExemplarCount *int
	cardinalityLimit int
}

type exponentialHistogramMetrics struct {
	metrics          map[Key]*exponentialHistogram
	maxSize          int32
	maxExemplarCount *int
	cardinalityLimit int
}

type explicitHistogram struct {
	attributes pcommon.Map
	exemplars  pmetric.ExemplarSlice

	bucketCounts []uint64
	count        uint64
	sum          float64

	bounds []float64

	maxExemplarCount *int

	startTimestamp pcommon.Timestamp
}

type exponentialHistogram struct {
	attributes pcommon.Map
	exemplars  pmetric.ExemplarSlice

	histogram *structure.Histogram[float64]

	maxExemplarCount *int

	startTimestamp pcommon.Timestamp
}

type BuildAttributesFun func() pcommon.Map

func NewExponentialHistogramMetrics(maxSize int32, maxExemplarCount *int, cardinalityLimit int) HistogramMetrics {
	return &exponentialHistogramMetrics{
		metrics:          make(map[Key]*exponentialHistogram),
		maxSize:          maxSize,
		maxExemplarCount: maxExemplarCount,
		cardinalityLimit: cardinalityLimit,
	}
}

func NewExplicitHistogramMetrics(bounds []float64, maxExemplarCount *int, cardinalityLimit int) HistogramMetrics {
	return &explicitHistogramMetrics{
		metrics:          make(map[Key]*explicitHistogram),
		bounds:           bounds,
		maxExemplarCount: maxExemplarCount,
		cardinalityLimit: cardinalityLimit,
	}
}

func (m *explicitHistogramMetrics) IsCardinalityLimitReached() bool {
	return m.cardinalityLimit > 0 && len(m.metrics) >= m.cardinalityLimit
}

func (m *explicitHistogramMetrics) GetOrCreate(key Key, attributesFun BuildAttributesFun, startTimestamp pcommon.Timestamp) (Histogram, bool) {
	limitReached := false
	h, ok := m.metrics[key]
	if !ok {
		var attributes pcommon.Map
		if m.IsCardinalityLimitReached() {
			limitReached = true
			key = overflowKey

			// check if overflowKey already exists
			h, ok = m.metrics[key]
			if ok {
				return h, limitReached
			}

			attributes = pcommon.NewMap()
			attributes.PutBool(overflowKey, true)
		} else {
			attributes = attributesFun()
		}

		h = &explicitHistogram{
			attributes:       attributes,
			exemplars:        pmetric.NewExemplarSlice(),
			bounds:           m.bounds,
			bucketCounts:     make([]uint64, len(m.bounds)+1),
			maxExemplarCount: m.maxExemplarCount,
			startTimestamp:   startTimestamp,
		}
		m.metrics[key] = h
	}
	return h, limitReached
}

func (m *explicitHistogramMetrics) BuildMetrics(
	metric pmetric.Metric,
	timestamp pcommon.Timestamp,
	startTimeStampGenerator func(Key, pcommon.Timestamp) pcommon.Timestamp,
	temporality pmetric.AggregationTemporality,
) {
	metric.SetEmptyHistogram().SetAggregationTemporality(temporality)
	dps := metric.Histogram().DataPoints()
	dps.EnsureCapacity(len(m.metrics))
	for k, h := range m.metrics {
		dp := dps.AppendEmpty()
		startTimestamp := startTimeStampGenerator(k, h.startTimestamp)
		dp.SetStartTimestamp(startTimestamp)
		dp.SetTimestamp(timestamp)
		dp.ExplicitBounds().FromRaw(h.bounds)
		dp.BucketCounts().FromRaw(h.bucketCounts)
		dp.SetCount(h.count)
		dp.SetSum(h.sum)
		for i := 0; i < h.exemplars.Len(); i++ {
			h.exemplars.At(i).SetTimestamp(timestamp)
		}
		h.exemplars.CopyTo(dp.Exemplars())
		h.attributes.CopyTo(dp.Attributes())
	}
}

func (m *explicitHistogramMetrics) ClearExemplars() {
	for _, h := range m.metrics {
		h.exemplars = pmetric.NewExemplarSlice()
	}
}

func (m *exponentialHistogramMetrics) IsCardinalityLimitReached() bool {
	return m.cardinalityLimit > 0 && len(m.metrics) >= m.cardinalityLimit
}

func (m *exponentialHistogramMetrics) GetOrCreate(key Key, attributesFun BuildAttributesFun, startTimeStamp pcommon.Timestamp) (Histogram, bool) {
	limitReached := false
	h, ok := m.metrics[key]
	if !ok {
		histogram := new(structure.Histogram[float64])
		cfg := structure.NewConfig(
			structure.WithMaxSize(m.maxSize),
		)
		histogram.Init(cfg)

		var attributes pcommon.Map
		if m.IsCardinalityLimitReached() {
			limitReached = true
			key = overflowKey

			// check if overflowKey already exists
			h, ok = m.metrics[key]
			if ok {
				return h, limitReached
			}

			attributes = pcommon.NewMap()
			attributes.PutBool(overflowKey, true)
		} else {
			attributes = attributesFun()
		}

		h = &exponentialHistogram{
			histogram:        histogram,
			attributes:       attributes,
			exemplars:        pmetric.NewExemplarSlice(),
			maxExemplarCount: m.maxExemplarCount,
			startTimestamp:   startTimeStamp,
		}
		m.metrics[key] = h
	}
	return h, limitReached
}

func (m *exponentialHistogramMetrics) BuildMetrics(
	metric pmetric.Metric,
	timestamp pcommon.Timestamp,
	startTimeStampGenerator func(Key, pcommon.Timestamp) pcommon.Timestamp,
	temporality pmetric.AggregationTemporality,
) {
	metric.SetEmptyExponentialHistogram().SetAggregationTemporality(temporality)
	dps := metric.ExponentialHistogram().DataPoints()
	dps.EnsureCapacity(len(m.metrics))
	for k, e := range m.metrics {
		dp := dps.AppendEmpty()
		startTimestamp := startTimeStampGenerator(k, e.startTimestamp)
		dp.SetStartTimestamp(startTimestamp)
		dp.SetTimestamp(timestamp)
		expoHistToExponentialDataPoint(e.histogram, dp)
		for i := 0; i < e.exemplars.Len(); i++ {
			e.exemplars.At(i).SetTimestamp(timestamp)
		}
		e.exemplars.CopyTo(dp.Exemplars())
		e.attributes.CopyTo(dp.Attributes())
	}
}

// expoHistToExponentialDataPoint copies `lightstep/go-expohisto` structure.Histogram to
// pmetric.ExponentialHistogramDataPoint
func expoHistToExponentialDataPoint(agg *structure.Histogram[float64], dp pmetric.ExponentialHistogramDataPoint) {
	dp.SetCount(agg.Count())
	dp.SetSum(agg.Sum())
	if agg.Count() != 0 {
		dp.SetMin(agg.Min())
		dp.SetMax(agg.Max())
	}

	dp.SetZeroCount(agg.ZeroCount())
	dp.SetScale(agg.Scale())

	for _, half := range []struct {
		inFunc  func() *structure.Buckets
		outFunc func() pmetric.ExponentialHistogramDataPointBuckets
	}{
		{agg.Positive, dp.Positive},
		{agg.Negative, dp.Negative},
	} {
		in := half.inFunc()
		out := half.outFunc()
		out.SetOffset(in.Offset())
		out.BucketCounts().EnsureCapacity(int(in.Len()))

		for i := uint32(0); i < in.Len(); i++ {
			out.BucketCounts().Append(in.At(i))
		}
	}
}

func (m *exponentialHistogramMetrics) ClearExemplars() {
	for _, m := range m.metrics {
		m.exemplars = pmetric.NewExemplarSlice()
	}
}

func (h *explicitHistogram) Observe(value float64) {
	h.sum += value
	h.count++

	// Binary search to find the value bucket index.
	index := sort.SearchFloat64s(h.bounds, value)
	h.bucketCounts[index]++
}

func (h *explicitHistogram) AddExemplar(traceID pcommon.TraceID, spanID pcommon.SpanID, value float64) {
	if h.maxExemplarCount != nil && h.exemplars.Len() >= *h.maxExemplarCount {
		return
	}
	e := h.exemplars.AppendEmpty()
	e.SetTraceID(traceID)
	e.SetSpanID(spanID)
	e.SetDoubleValue(value)
}

func (h *exponentialHistogram) Observe(value float64) {
	h.histogram.Update(value)
}

func (h *exponentialHistogram) AddExemplar(traceID pcommon.TraceID, spanID pcommon.SpanID, value float64) {
	if h.maxExemplarCount != nil && h.exemplars.Len() >= *h.maxExemplarCount {
		return
	}
	e := h.exemplars.AppendEmpty()
	e.SetTraceID(traceID)
	e.SetSpanID(spanID)
	e.SetDoubleValue(value)
}

type Sum struct {
	attributes pcommon.Map
	count      uint64

	exemplars        pmetric.ExemplarSlice
	maxExemplarCount *int

	startTimestamp pcommon.Timestamp
	// isFirst is used to track if this datapoint is new to the Sum. This
	// is used to ensure that new Sum metrics being with 0, and then are incremented
	// to the desired value.  This avoids Prometheus throwing away the first
	// value in the series, due to the transition from null -> x.
	isFirst bool
}

func (s *Sum) Add(value uint64) {
	s.count += value
}

func NewSumMetrics(maxExemplarCount *int, cardinalityLimit int) SumMetrics {
	return SumMetrics{
		metrics:          make(map[Key]*Sum),
		maxExemplarCount: maxExemplarCount,
		cardinalityLimit: cardinalityLimit,
	}
}

type SumMetrics struct {
	metrics          map[Key]*Sum
	maxExemplarCount *int
	cardinalityLimit int
}

func (m *SumMetrics) IsCardinalityLimitReached() bool {
	return m.cardinalityLimit > 0 && len(m.metrics) >= m.cardinalityLimit
}

func (m *SumMetrics) GetOrCreate(key Key, attributesFun BuildAttributesFun, startTimestamp pcommon.Timestamp) (*Sum, bool) {
	limitReached := false
	s, ok := m.metrics[key]
	if !ok {
		var attributes pcommon.Map
		// check when new key coming
		if m.IsCardinalityLimitReached() {
			limitReached = true
			key = overflowKey

			// check if overflowKey already exists
			s, ok = m.metrics[key]
			if ok {
				return s, limitReached
			}

			attributes = pcommon.NewMap()
			attributes.PutBool(overflowKey, true)
		} else {
			attributes = attributesFun()
		}

		s = &Sum{
			attributes:       attributes,
			exemplars:        pmetric.NewExemplarSlice(),
			maxExemplarCount: m.maxExemplarCount,
			startTimestamp:   startTimestamp,
			isFirst:          true,
		}
		m.metrics[key] = s
	}

	return s, limitReached
}

func (s *Sum) AddExemplar(traceID pcommon.TraceID, spanID pcommon.SpanID, value float64) {
	if s.maxExemplarCount != nil && s.exemplars.Len() >= *s.maxExemplarCount {
		return
	}
	e := s.exemplars.AppendEmpty()
	e.SetTraceID(traceID)
	e.SetSpanID(spanID)
	e.SetDoubleValue(value)
}

func (m *SumMetrics) BuildMetrics(
	metric pmetric.Metric,
	timestamp pcommon.Timestamp,
	startTimeStampGenerator func(Key, pcommon.Timestamp) pcommon.Timestamp,
	temporality pmetric.AggregationTemporality,
) {
	metric.SetEmptySum().SetIsMonotonic(true)
	metric.Sum().SetAggregationTemporality(temporality)

	dps := metric.Sum().DataPoints()
	dps.EnsureCapacity(len(m.metrics))
	for k, s := range m.metrics {
		dp := dps.AppendEmpty()
		startTimeStamp := startTimeStampGenerator(k, s.startTimestamp)
		dp.SetStartTimestamp(startTimeStamp)
		dp.SetTimestamp(timestamp)
		if s.isFirst {
			dp.SetIntValue(0)
			s.isFirst = false
		} else {
			dp.SetIntValue(int64(s.count))
		}
		for i := 0; i < s.exemplars.Len(); i++ {
			s.exemplars.At(i).SetTimestamp(timestamp)
		}
		s.exemplars.CopyTo(dp.Exemplars())
		s.attributes.CopyTo(dp.Attributes())
	}
}

func (m *SumMetrics) ClearExemplars() {
	for _, sum := range m.metrics {
		sum.exemplars = pmetric.NewExemplarSlice()
	}
}
