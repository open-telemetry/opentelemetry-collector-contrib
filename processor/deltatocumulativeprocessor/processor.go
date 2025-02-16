// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/telemetry"
)

var _ processor.Metrics = (*Processor)(nil)

type Processor struct {
	next consumer.Metrics
	cfg  Config

	last state

	ctx    context.Context
	cancel context.CancelFunc

	stale *xsync.MapOf[identity.Stream, time.Time]
	tel   telemetry.Metrics
}

func newProcessor(cfg *Config, tel telemetry.Metrics, next consumer.Metrics) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	size := new(atomic.Int64)
	count := new(atomic.Int64)

	proc := Processor{
		next: next,
		cfg:  *cfg,
		last: state{
			size: size,
			nums: maps.Limit(maps.Count(xsync.NewMapOf[identity.Stream, *mutex[pmetric.NumberDataPoint]](), size), int64(cfg.MaxStreams), count),
			hist: maps.Limit(maps.Count(xsync.NewMapOf[identity.Stream, *mutex[pmetric.HistogramDataPoint]](), size), int64(cfg.MaxStreams), count),
			expo: maps.Limit(maps.Count(xsync.NewMapOf[identity.Stream, *mutex[pmetric.ExponentialHistogramDataPoint]](), size), int64(cfg.MaxStreams), count),
		},
		ctx:    ctx,
		cancel: cancel,

		stale: xsync.NewMapOf[identity.Stream, time.Time](),
		tel:   tel,
	}

	tel.WithTracked(proc.last.Size)
	cfg.Metrics(tel)

	return &proc
}

type vals struct {
	nums *mutex[pmetric.NumberDataPoint]
	hist *mutex[pmetric.HistogramDataPoint]
	expo *mutex[pmetric.ExponentialHistogramDataPoint]
}

func (p *Processor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	now := time.Now()

	const (
		keep = true
		drop = false
	)

	zero := vals{
		nums: guard(pmetric.NewNumberDataPoint()),
		hist: guard(pmetric.NewHistogramDataPoint()),
		expo: guard(pmetric.NewExponentialHistogramDataPoint()),
	}

	metrics.Filter(md, func(m metrics.Metric) bool {
		if m.AggregationTemporality() != pmetric.AggregationTemporalityDelta {
			return keep
		}

		// aggregate the datapoints.
		// using filter here, as the pmetric.*DataPoint are reference types so
		// we can modify them using their "value".
		m.Filter(func(id identity.Stream, dp any) bool {
			// count the processed datatype.
			// uses whatever value of attrs has at return-time
			var attrs telemetry.Attributes
			defer func() { p.tel.Datapoints().Inc(ctx, attrs...) }()

			var err error
			switch dp := dp.(type) {
			case pmetric.NumberDataPoint:
				last, loaded := p.last.nums.LoadOrStore(id, zero.nums)
				if maps.Exceeded(last, loaded) {
					// state is full, reject stream
					attrs.Set(telemetry.Error("limit"))
					return drop
				}

				// stream is ok and active, update stale tracker
				p.stale.Store(id, now)

				if !loaded {
					// cached zero was stored, alloc new one
					zero.nums = guard(pmetric.NewNumberDataPoint())
				}

				last.use(func(last pmetric.NumberDataPoint) {
					err = delta.AccumulateInto(last, dp)
					last.CopyTo(dp)
				})
			case pmetric.HistogramDataPoint:
				last, loaded := p.last.hist.LoadOrStore(id, zero.hist)
				if maps.Exceeded(last, loaded) {
					// state is full, reject stream
					attrs.Set(telemetry.Error("limit"))
					return drop
				}

				// stream is ok and active, update stale tracker
				p.stale.Store(id, now)

				if !loaded {
					// cached zero was stored, alloc new one
					zero.hist = guard(pmetric.NewHistogramDataPoint())
				}

				last.use(func(last pmetric.HistogramDataPoint) {
					err = delta.AccumulateInto(last, dp)
					last.CopyTo(dp)
				})
			case pmetric.ExponentialHistogramDataPoint:
				last, loaded := p.last.expo.LoadOrStore(id, zero.expo)
				if maps.Exceeded(last, loaded) {
					// state is full, reject stream
					attrs.Set(telemetry.Error("limit"))
					return drop
				}

				// stream is ok and active, update stale tracker
				p.stale.Store(id, now)

				if !loaded {
					// cached zero was stored, alloc new one
					zero.expo = guard(pmetric.NewExponentialHistogramDataPoint())
				}

				last.use(func(last pmetric.ExponentialHistogramDataPoint) {
					err = delta.AccumulateInto(last, dp)
					last.CopyTo(dp)
				})
			}

			if err != nil {
				attrs.Set(telemetry.Cause(err))
				return drop
			}

			return keep
		})

		// all remaining datapoints of this metric are now cumulative
		m.Typed().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		// if no datapoints remain, drop empty metric
		return m.Typed().Len() > 0
	})

	// no need to continue pipeline if we dropped all metrics
	if md.MetricCount() == 0 {
		return nil
	}
	return p.next.ConsumeMetrics(ctx, md)
}

func (p *Processor) Start(_ context.Context, _ component.Host) error {
	if p.cfg.MaxStale != 0 {
		// delete stale streams once per minute
		go func() {
			tick := time.NewTicker(time.Minute)
			defer tick.Stop()
			for {
				select {
				case <-p.ctx.Done():
					return
				case <-tick.C:
					now := time.Now()
					p.stale.Range(func(id identity.Stream, last time.Time) bool {
						if now.Sub(last) > p.cfg.MaxStale {
							p.last.nums.LoadAndDelete(id)
							p.last.hist.LoadAndDelete(id)
							p.last.expo.LoadAndDelete(id)
							p.stale.Delete(id)
						}
						return true
					})
				}
			}
		}()
	}

	return nil
}

func (p *Processor) Shutdown(_ context.Context) error {
	p.cancel()
	return nil
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// state keeps a cumulative value, aggregated over time, per stream
type state struct {
	size *atomic.Int64
	nums maps.Concurrent[identity.Stream, *mutex[pmetric.NumberDataPoint]]
	hist maps.Concurrent[identity.Stream, *mutex[pmetric.HistogramDataPoint]]
	expo maps.Concurrent[identity.Stream, *mutex[pmetric.ExponentialHistogramDataPoint]]
}

func (s state) Size() int {
	return int(s.size.Load())
}

type mutex[T any] struct {
	mtx sync.Mutex
	v   T
}

func (mtx *mutex[T]) use(do func(T)) {
	mtx.mtx.Lock()
	do(mtx.v)
	mtx.mtx.Unlock()
}

func guard[T any](v T) *mutex[T] {
	return &mutex[T]{v: v}
}
