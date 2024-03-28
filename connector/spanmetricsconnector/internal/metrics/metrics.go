// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"

import (
	"sort"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Key string

type HistogramMetrics interface {
	GetOrCreate(key Key, attributes pcommon.Map) Histogram
	BuildMetrics(pmetric.Metric, pcommon.Timestamp, pmetric.AggregationTemporality)
	Reset(onlyExemplars bool)
}

type Histogram interface {
	Observe(value float64)
	AddExemplar(traceID pcommon.TraceID, spanID pcommon.SpanID, value float64)
}

type explicitHistogramMetrics struct {
	metrics          map[Key]*explicitHistogram
	bounds           []float64
	maxExemplarCount *int
}

type exponentialHistogramMetrics struct {
	metrics          map[Key]*exponentialHistogram
	maxSize          int32
	maxExemplarCount *int
}

type explicitHistogram struct {
	attributes pcommon.Map
	exemplars  pmetric.ExemplarSlice

	bucketCounts []uint64
	count        uint64
	sum          float64

	bounds []float64

	maxExemplarCount *int
}

type exponentialHistogram struct {
	attributes pcommon.Map
	exemplars  pmetric.ExemplarSlice

	histogram *structure.Histogram[float64]

	maxExemplarCount *int
}

func NewExponentialHistogramMetrics(maxSize int32, maxExemplarCount *int) HistogramMetrics {
	return &exponentialHistogramMetrics{
		metrics:          make(map[Key]*exponentialHistogram),
		maxSize:          maxSize,
		maxExemplarCount: maxExemplarCount,
	}
}

func NewExplicitHistogramMetrics(bounds []float64, maxExemplarCount *int) HistogramMetrics {
	return &explicitHistogramMetrics{
		metrics:          make(map[Key]*explicitHistogram),
		bounds:           bounds,
		maxExemplarCount: maxExemplarCount,
	}
}

func (m *explicitHistogramMetrics) GetOrCreate(key Key, attributes pcommon.Map) Histogram {
	h, ok := m.metrics[key]
	if !ok {
		h = &explicitHistogram{
			attributes:       attributes,
			exemplars:        pmetric.NewExemplarSlice(),
			bounds:           m.bounds,
			bucketCounts:     make([]uint64, len(m.bounds)+1),
			maxExemplarCount: m.maxExemplarCount,
		}
		m.metrics[key] = h
	}

	return h
}

func (m *explicitHistogramMetrics) BuildMetrics(
	metric pmetric.Metric,
	start pcommon.Timestamp,
	temporality pmetric.AggregationTemporality,
) {
	metric.SetEmptyHistogram().SetAggregationTemporality(temporality)
	dps := metric.Histogram().DataPoints()
	dps.EnsureCapacity(len(m.metrics))
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for _, h := range m.metrics {
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(start)
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

func (m *explicitHistogramMetrics) Reset(onlyExemplars bool) {
	if onlyExemplars {
		for _, h := range m.metrics {
			h.exemplars = pmetric.NewExemplarSlice()
		}
		return
	}

	m.metrics = make(map[Key]*explicitHistogram)
}

func (m *exponentialHistogramMetrics) GetOrCreate(key Key, attributes pcommon.Map) Histogram {
	h, ok := m.metrics[key]
	if !ok {
		histogram := new(structure.Histogram[float64])
		cfg := structure.NewConfig(
			structure.WithMaxSize(m.maxSize),
		)
		histogram.Init(cfg)

		h = &exponentialHistogram{
			histogram:        histogram,
			attributes:       attributes,
			exemplars:        pmetric.NewExemplarSlice(),
			maxExemplarCount: m.maxExemplarCount,
		}
		m.metrics[key] = h

	}

	return h
}

func (m *exponentialHistogramMetrics) BuildMetrics(
	metric pmetric.Metric,
	start pcommon.Timestamp,
	temporality pmetric.AggregationTemporality,
) {
	metric.SetEmptyExponentialHistogram().SetAggregationTemporality(temporality)
	dps := metric.ExponentialHistogram().DataPoints()
	dps.EnsureCapacity(len(m.metrics))
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for _, m := range m.metrics {
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(timestamp)
		expoHistToExponentialDataPoint(m.histogram, dp)
		for i := 0; i < m.exemplars.Len(); i++ {
			m.exemplars.At(i).SetTimestamp(timestamp)
		}
		m.exemplars.CopyTo(dp.Exemplars())
		m.attributes.CopyTo(dp.Attributes())
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

func (m *exponentialHistogramMetrics) Reset(onlyExemplars bool) {
	if onlyExemplars {
		for _, m := range m.metrics {
			m.exemplars = pmetric.NewExemplarSlice()
		}
		return
	}

	m.metrics = make(map[Key]*exponentialHistogram)
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
	attributes       pcommon.Map
	count            uint64
	exemplars        pmetric.ExemplarSlice
	maxExemplarCount *int
}

func (s *Sum) Add(value uint64) {
	s.count += value
}

func NewSumMetrics(maxExemplarCount *int) SumMetrics {
	return SumMetrics{
		metrics:          make(map[Key]*Sum),
		maxExemplarCount: maxExemplarCount,
	}
}

type SumMetrics struct {
	metrics          map[Key]*Sum
	maxExemplarCount *int
}

func (m *SumMetrics) GetOrCreate(key Key, attributes pcommon.Map) *Sum {
	s, ok := m.metrics[key]
	if !ok {
		s = &Sum{
			attributes:       attributes,
			exemplars:        pmetric.NewExemplarSlice(),
			maxExemplarCount: m.maxExemplarCount,
		}
		m.metrics[key] = s
	}
	return s
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
	start pcommon.Timestamp,
	temporality pmetric.AggregationTemporality,
) {
	metric.SetEmptySum().SetIsMonotonic(true)
	metric.Sum().SetAggregationTemporality(temporality)

	dps := metric.Sum().DataPoints()
	dps.EnsureCapacity(len(m.metrics))
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for _, s := range m.metrics {
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(timestamp)
		dp.SetIntValue(int64(s.count))
		for i := 0; i < s.exemplars.Len(); i++ {
			s.exemplars.At(i).SetTimestamp(timestamp)
		}
		s.exemplars.CopyTo(dp.Exemplars())
		s.attributes.CopyTo(dp.Attributes())
	}
}

func (m *SumMetrics) Reset() {
	m.metrics = make(map[Key]*Sum)
}
