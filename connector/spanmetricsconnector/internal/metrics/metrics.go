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

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type HistogramMetrics interface {
	GetOrCreate(key string, attributes pcommon.Map) Histogram
	Reset(onlyExemplars bool)
}

type Histogram interface {
	Observe(value float64)
	AddExemplar(traceID pcommon.TraceID, spanID pcommon.SpanID, value float64)
}

type ExplicitHistogramMetrics struct {
	Metrics map[string]*ExplicitHistogram
	Bounds  []float64
}

type ExponentialHistogramMetrics struct {
	Metrics map[string]*ExponentialHistogram
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

func (m *ExplicitHistogramMetrics) GetOrCreate(key string, attributes pcommon.Map) Histogram {
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

func (m *ExplicitHistogramMetrics) Reset(onlyExemplars bool) {
	if onlyExemplars {
		for _, h := range m.Metrics {
			h.Exemplars = pmetric.NewExemplarSlice()
		}
		return
	}

	m.Metrics = make(map[string]*ExplicitHistogram)
}

func (m *ExponentialHistogramMetrics) GetOrCreate(key string, attributes pcommon.Map) Histogram {
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

func (m *ExponentialHistogramMetrics) Reset(onlyExemplars bool) {
	if onlyExemplars {
		for _, h := range m.Metrics {
			h.Exemplars = pmetric.NewExemplarSlice()
		}
		return
	}

	m.Metrics = make(map[string]*ExponentialHistogram)
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
