// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

type ExplicitHistogramMetrics struct {
	Metrics map[Key]*ExplicitHistogram
	Bounds  []float64
}

type ExponentialHistogramMetrics struct {
	Metrics map[Key]*ExponentialHistogram
	MaxSize int32
}

type ExplicitHistogram struct {
	Attributes pcommon.Map
	Exemplars  pmetric.ExemplarSlice

	BucketCounts []uint64
	Count        uint64
	Sum          float64

	Bounds []float64
}

type ExponentialHistogram struct {
	Attributes pcommon.Map
	Exemplars  pmetric.ExemplarSlice

	Agg *structure.Histogram[float64]
}

func (m *ExplicitHistogramMetrics) GetOrCreate(key Key, attributes pcommon.Map) Histogram {
	h, ok := m.Metrics[key]
	if !ok {
		h = &ExplicitHistogram{
			Attributes:   attributes,
			Exemplars:    pmetric.NewExemplarSlice(),
			Bounds:       m.Bounds,
			BucketCounts: make([]uint64, len(m.Bounds)+1),
		}
		m.Metrics[key] = h
	}

	return h
}

func (m *ExplicitHistogramMetrics) BuildMetrics(
	metric pmetric.Metric,
	start pcommon.Timestamp,
	temporality pmetric.AggregationTemporality,
) {
	metric.SetEmptyHistogram().SetAggregationTemporality(temporality)
	dps := metric.Histogram().DataPoints()
	dps.EnsureCapacity(len(m.Metrics))
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for _, h := range m.Metrics {
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(timestamp)
		dp.ExplicitBounds().FromRaw(h.Bounds)
		dp.BucketCounts().FromRaw(h.BucketCounts)
		dp.SetCount(h.Count)
		dp.SetSum(h.Sum)
		for i := 0; i < dp.Exemplars().Len(); i++ {
			dp.Exemplars().At(i).SetTimestamp(timestamp)
		}
		h.Attributes.CopyTo(dp.Attributes())
	}
}

func (m *ExplicitHistogramMetrics) Reset(onlyExemplars bool) {
	if onlyExemplars {
		for _, h := range m.Metrics {
			h.Exemplars = pmetric.NewExemplarSlice()
		}
		return
	}

	m.Metrics = make(map[Key]*ExplicitHistogram)
}

func (m *ExponentialHistogramMetrics) GetOrCreate(key Key, attributes pcommon.Map) Histogram {
	h, ok := m.Metrics[key]
	if !ok {
		agg := new(structure.Histogram[float64])
		cfg := structure.NewConfig(
			structure.WithMaxSize(m.MaxSize),
		)
		agg.Init(cfg)

		h = &ExponentialHistogram{
			Agg:        agg,
			Attributes: attributes,
			Exemplars:  pmetric.NewExemplarSlice(),
		}
		m.Metrics[key] = h
	}

	return h
}

func (m *ExponentialHistogramMetrics) BuildMetrics(
	metric pmetric.Metric,
	start pcommon.Timestamp,
	temporality pmetric.AggregationTemporality,
) {
	metric.SetEmptyExponentialHistogram().SetAggregationTemporality(temporality)
	dps := metric.ExponentialHistogram().DataPoints()
	dps.EnsureCapacity(len(m.Metrics))
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for _, h := range m.Metrics {
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(timestamp)
		expoHistToExponentialDataPoint(h.Agg, dp)
		for i := 0; i < dp.Exemplars().Len(); i++ {
			dp.Exemplars().At(i).SetTimestamp(timestamp)
		}
		h.Attributes.CopyTo(dp.Attributes())
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

func (m *ExponentialHistogramMetrics) Reset(onlyExemplars bool) {
	if onlyExemplars {
		for _, h := range m.Metrics {
			h.Exemplars = pmetric.NewExemplarSlice()
		}
		return
	}

	m.Metrics = make(map[Key]*ExponentialHistogram)
}

func (h *ExplicitHistogram) Observe(value float64) {
	h.Sum += value
	h.Count++

	// Binary search to find the latencyMs bucket index.
	index := sort.SearchFloat64s(h.Bounds, value)
	h.BucketCounts[index]++
}

func (h *ExplicitHistogram) AddExemplar(traceID pcommon.TraceID, spanID pcommon.SpanID, value float64) {
	e := h.Exemplars.AppendEmpty()
	e.SetTraceID(traceID)
	e.SetSpanID(spanID)
	e.SetDoubleValue(value)
}

func (h *ExponentialHistogram) Observe(value float64) {
	h.Agg.Update(value)
}

func (h *ExponentialHistogram) AddExemplar(traceID pcommon.TraceID, spanID pcommon.SpanID, value float64) {
	e := h.Exemplars.AppendEmpty()
	e.SetTraceID(traceID)
	e.SetSpanID(spanID)
	e.SetDoubleValue(value)
}

type Sum struct {
	Attributes pcommon.Map
	Count      uint64
}

type SumMetrics struct {
	Metrics map[Key]*Sum
}

func (m *SumMetrics) BuildMetrics(
	metric pmetric.Metric,
	start pcommon.Timestamp,
	temporality pmetric.AggregationTemporality,
) {
	metric.SetEmptySum().SetIsMonotonic(true)
	metric.Sum().SetAggregationTemporality(temporality)

	dps := metric.Sum().DataPoints()
	dps.EnsureCapacity(len(m.Metrics))
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for _, s := range m.Metrics {
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(timestamp)
		dp.SetIntValue(int64(s.Count))
		s.Attributes.CopyTo(dp.Attributes())
	}
}

func (m *SumMetrics) Reset() {
	m.Metrics = make(map[Key]*Sum)
}
