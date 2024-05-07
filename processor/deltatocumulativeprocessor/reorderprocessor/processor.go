// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reorderprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor/internal/gmetric"
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

type Processor struct {
	cfg  Config
	next consumer.Metrics
	log  *zap.Logger

	ctx    context.Context
	cancel func()

	mtx  sync.Mutex
	meta Meta

	nums Queue[pmetric.NumberDataPoint, pmetric.NumberDataPointSlice]
	expo Queue[pmetric.ExponentialHistogramDataPoint, pmetric.ExponentialHistogramDataPointSlice]
	hist Queue[pmetric.HistogramDataPoint, pmetric.HistogramDataPointSlice]
	summ Queue[pmetric.SummaryDataPoint, pmetric.SummaryDataPointSlice]
}

func New(cfg *Config, next consumer.Metrics, log *zap.Logger) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	proc := Processor{
		cfg:  *cfg,
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

		nums: make(Queue[pmetric.NumberDataPoint, pmetric.NumberDataPointSlice]),
		hist: make(Queue[pmetric.HistogramDataPoint, pmetric.HistogramDataPointSlice]),
		expo: make(Queue[pmetric.ExponentialHistogramDataPoint, pmetric.ExponentialHistogramDataPointSlice]),
		summ: make(Queue[pmetric.SummaryDataPoint, pmetric.SummaryDataPointSlice]),
	}

	return &proc
}

func (p *Processor) size() int {
	return len(p.nums) + len(p.expo) + len(p.hist) + len(p.summ)
}

func (p *Processor) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var errs error
	// for rm := range gmetric.ResourceMetrics(md)
	gmetric.ResourceMetrics(md)(func(rm gmetric.ResourceMetric) bool {
		id := rm.Ident()
		// move data to queues
		switch m := rm.Typed().(type) {
		case gmetric.Gauge:
			moveToQueue(p, id, m, p.nums)
		case gmetric.Sum:
			moveToQueue(p, id, m, p.nums)
		case gmetric.Hist:
			moveToQueue(p, id, m, p.hist)
		case gmetric.Expo:
			moveToQueue(p, id, m, p.expo)
		case gmetric.Summ:
			moveToQueue(p, id, m, p.summ)
		}

		// store metadata for later reconstruction
		p.meta.Store(rm)
		return true
	})

	return errs
}

// moveToQueue moves all datapoints out of the individual metrics and stores
// them on their respective priority queue.
//
// once finished, all metrics have empty datapoint slices, but all metadata is
// left intact.
func moveToQueue[M gmetric.Metric[T, S], S gmetric.Slice[S, T], T gmetric.Point[T]](p *Processor, id identity.Metric, m M, q Queue[T, S]) {
	// for _, dp := range gmetric.All(m.DataPoints())
	gmetric.All(m.DataPoints())(func(_ int, dp T) bool {
		id := identity.OfStream(id, dp)
		if _, has := q[id]; !has && p.size() >= p.cfg.Max {
			// continue
			// TODO: record as metric
			return true
		}
		q.store(id, dp)
		return true
	})

	gmetric.Clear(m.DataPoints())
}

func (p *Processor) Export() error {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	md := pmetric.NewMetrics()
	until := pcommon.NewTimestampFromTime(time.Now().Add(-p.cfg.Hold))

	nums := p.nums.take(until)
	hist := p.hist.take(until)
	expo := p.expo.take(until)
	summ := p.summ.take(until)

	mids := make([]identity.Metric, 0, len(nums)+len(hist)+len(expo)+len(summ))
	ids := chain(keys(nums), keys(hist), keys(expo), keys(summ))
	// for id := range ids
	ids(func(id identity.Stream) bool {
		mids = append(mids, id.Metric())
		return true
	})

	mt := p.meta.Select(mids...)

	// for id := range ids
	ids(func(id identity.Stream) bool {
		m := mt.Lookup(id.Metric())
		switch m := gmetric.Typed(m).(type) {
		case gmetric.Gauge:
			nums[id].MoveAndAppendTo(m.DataPoints())
		case gmetric.Sum:
			nums[id].MoveAndAppendTo(m.DataPoints())
		case gmetric.Expo:
			expo[id].MoveAndAppendTo(m.DataPoints())
		case gmetric.Hist:
			hist[id].MoveAndAppendTo(m.DataPoints())
		case gmetric.Summ:
			summ[id].MoveAndAppendTo(m.DataPoints())
		}
		return true
	})

	return p.next.ConsumeMetrics(p.ctx, md)
}

func keys[K comparable, V any](m map[K]V) func(func(K) bool) {
	return func(yield func(K) bool) {
		for k := range m {
			if !yield(k) {
				break
			}
		}
	}
}

func chain[T any](iters ...func(func(T) bool)) func(func(T) bool) {
	return func(yield func(T) bool) {
		ok := true
		for _, iter := range iters {
			iter(func(v T) bool {
				ok = yield(v)
				return ok
			})
			if !ok {
				break
			}
		}
	}

}
