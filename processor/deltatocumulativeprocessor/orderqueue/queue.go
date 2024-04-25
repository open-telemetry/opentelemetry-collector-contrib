// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package orderqueue // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/orderqueue"

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/orderqueue/internal/heap"
)

type Config struct {
	Hold time.Duration `mapstructure:"hold"`
	Max  int           `mapstructure:"max_streams"`
}

func Default() Config {
	return Config{
		Hold: 1 * time.Minute,
		Max:  0, // no limit
	}
}

func New(cfg Config, next consumer.Metrics, log *zap.Logger) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Processor{
		cfg:  cfg,
		next: next,
		log:  log,

		ctx:    ctx,
		cancel: cancel,

		mtx: sync.Mutex{},
		meta: Meta{
			resources: make(map[identity.Resource]pcommon.Resource),
			scopes:    make(map[identity.Scope]pcommon.InstrumentationScope),
			metrics:   make(map[identity.Metric]pmetric.Metric),
		},
		nums: make(map[identity.Stream]heap.Heap[pmetric.NumberDataPoint]),
		hist: make(map[identity.Stream]heap.Heap[pmetric.HistogramDataPoint]),
		expo: make(map[identity.Stream]heap.Heap[pmetric.ExponentialHistogramDataPoint]),
		summ: make(map[identity.Stream]heap.Heap[pmetric.SummaryDataPoint]),
	}
}

type Processor struct {
	cfg  Config
	next consumer.Metrics
	log  *zap.Logger

	ctx    context.Context
	cancel func()

	mtx  sync.Mutex
	meta Meta
	nums map[identity.Stream]heap.Heap[pmetric.NumberDataPoint]
	hist map[identity.Stream]heap.Heap[pmetric.HistogramDataPoint]
	expo map[identity.Stream]heap.Heap[pmetric.ExponentialHistogramDataPoint]
	summ map[identity.Stream]heap.Heap[pmetric.SummaryDataPoint]
}

func (p *Processor) size() int {
	return len(p.nums) + len(p.hist) + len(p.expo) + len(p.summ)
}

func (p *Processor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var errs error
	metrics.Each(md, func(m metrics.Metric) {
		mid := m.Ident()

		var ops Operations
		switch m.Type() {
		case pmetric.MetricTypeGauge:
			ops = use(mid, m.Gauge(), p.nums, pmetric.NewNumberDataPointSlice)
		case pmetric.MetricTypeSum:
			ops = use(mid, m.Sum(), p.nums, pmetric.NewNumberDataPointSlice)
		case pmetric.MetricTypeHistogram:
			ops = use(mid, m.Histogram(), p.hist, pmetric.NewHistogramDataPointSlice)
		case pmetric.MetricTypeExponentialHistogram:
			ops = use(mid, m.ExponentialHistogram(), p.expo, pmetric.NewExponentialHistogramDataPointSlice)
		case pmetric.MetricTypeSummary:
			ops = use(mid, m.Summary(), p.summ, pmetric.NewSummaryDataPointSlice)
		default:
			return
		}

		ops.dps(func(id identity.Stream, idx int) {
			if !ops.has(id) && p.cfg.Max > 0 && p.size() >= p.cfg.Max {
				errs = errors.Join(errs, streams.ErrLimit(p.cfg.Max))
				return
			}
			ops.push(idx)
		})
		ops.clear()

		// store metric metadata for future use
		p.meta.Store(m.Resource(), m.Scope(), m.Metric)
	})

	return errs
}

type Operations interface {
	dps(func(id identity.Stream, idx int))
	has(id identity.Stream) bool
	push(idx int)
	clear()
}

type (
	dtype[T any] interface {
		DataPoints() T
	}
	slice[Self any, E any] interface {
		Len() int
		At(i int) E

		MoveAndAppendTo(Self)
	}
	point interface {
		Attributes() pcommon.Map
		Timestamp() pcommon.Timestamp
	}
)

type dtops[P dtype[S], S slice[S, E], E point] struct {
	mid identity.Metric
	pty P

	heaps map[identity.Stream]heap.Heap[E]
	empty func() S
}

func (d dtops[P, S, E]) dps(fn func(identity.Stream, int)) {
	dps := d.pty.DataPoints()

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		fn(identity.OfStream(d.mid, dp), i)
	}
}

func (d dtops[P, S, E]) has(id identity.Stream) bool {
	_, ok := d.heaps[id]
	return ok
}

func (d dtops[P, S, E]) push(i int) {
	dp := d.pty.DataPoints().At(i)
	id := identity.OfStream(d.mid, dp)

	q, ok := d.heaps[id]
	if !ok {
		q = *heap.New(func(a, b E) bool {
			return a.Timestamp() < b.Timestamp()
		})
		d.heaps[id] = q
	}

	q.Push(dp)
}

func (d dtops[P, S, E]) clear() {
	d.pty.DataPoints().MoveAndAppendTo(d.empty())
}

func use[P dtype[S], S slice[S, E], E point](
	id identity.Metric,
	pty P,
	heaps map[identity.Stream]heap.Heap[E],
	empty func() S,
) dtops[P, S, E] {
	return dtops[P, S, E]{mid: id, pty: pty, heaps: heaps, empty: empty}
}
