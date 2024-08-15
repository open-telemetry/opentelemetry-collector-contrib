// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"
	exp "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

var _ processor.Metrics = (*Processor)(nil)

type Processor struct {
	next consumer.Metrics
	cfg  Config

	state Map
	mtx   sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	stale staleness.Tracker
	tel   metadata.TelemetryBuilder
}

func newProcessor(cfg *Config, tel *metadata.TelemetryBuilder, next consumer.Metrics) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	proc := Processor{
		next: next,
		cfg:  *cfg,
		state: Map{
			nums: make(exp.HashMap[data.Number]),
			hist: make(exp.HashMap[data.Histogram]),
			expo: make(exp.HashMap[data.ExpHistogram]),
		},
		ctx:    ctx,
		cancel: cancel,

		stale: staleness.NewTracker(),
		tel:   *tel,
	}
	return &proc
}

func (p *Processor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	now := time.Now()

	var errs error
	metrics.Filter(md, func(m metrics.Metric) bool {
		if m.AggregationTemporality() != pmetric.AggregationTemporalityDelta {
			return true
		}

		var each AggrFunc
		switch m := m.Typed().(type) {
		case metrics.Sum:
			each = use(m, p.state.nums)
		case metrics.Histogram:
			each = use(m, p.state.hist)
		case metrics.ExpHistogram:
			each = use(m, p.state.expo)
		}

		var ok bool
		err := each(func(id identity.Stream, aggr func() error) error {
			// if stream not seen before and stream limit is reached, reject
			if !p.state.Has(id) && p.state.Len() >= p.cfg.MaxStreams {
				// TODO: record metric
				return streams.Drop
			}
			ok = true

			// stream is alive, refresh it
			p.stale.Refresh(now, id)

			// accumulate current dp into state
			return aggr()
		})
		errs = errors.Join(errs, err)

		return ok
	})

	if errs != nil {
		return errs
	}

	// no need to continue pipeline if we dropped all data
	if md.MetricCount() == 0 {
		return nil
	}
	return p.next.ConsumeMetrics(ctx, md)
}

// AggrFunc calls `do` for datapoint of a metric, giving the caller the
// opportunity to decide whether to perform aggregation in a type-agnostic way,
// while keeping underlying strong typing.
//
// if `aggr` is called, the current datapoint is accumulated into the streams
// cumulative state.
//
// if any error is returned, the current datapoint is dropped, the error
// collected and eventually returned. [streams.Drop] silently drops a datapoint
type AggrFunc func(do func(id identity.Stream, aggr func() error) error) error

// use returns an AggrFunc for the given metric, accumulating into the given state,
// given `do` calls its `aggr`
func use[T data.Typed[T], M Metric[T]](m M, state streams.Map[T]) AggrFunc {
	return func(do func(id identity.Stream, aggr func() error) error) error {
		return streams.Apply(m, func(id identity.Stream, dp T) (T, error) {
			acc, ok := state.Load(id)
			aggr := func() error {
				m.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				// no previous state: take dp as-is
				if !ok {
					acc = dp
					return nil
				}

				// accumulate into previous state
				return delta.AccumulateInto(acc, dp)
			}
			err := do(id, aggr)
			state.Store(id, acc)
			return acc, err
		})
	}
}

func (p *Processor) Start(_ context.Context, _ component.Host) error {
	if p.cfg.MaxStale != 0 {
		go func() {
			tick := time.NewTicker(time.Minute)
			defer tick.Stop()
			for {
				select {
				case <-p.ctx.Done():
					return
				case <-tick.C:
					p.mtx.Lock()
					stale := p.stale.Collect(p.cfg.MaxStale)
					for _, id := range stale {
						p.state.Delete(id)
					}
					p.mtx.Unlock()
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

type Metric[T data.Point[T]] interface {
	metrics.Filterable[T]
	SetAggregationTemporality(pmetric.AggregationTemporality)
}

type Map struct {
	nums streams.Map[data.Number]
	hist streams.Map[data.Histogram]
	expo streams.Map[data.ExpHistogram]
}

func (m Map) Len() int {
	return m.nums.Len() + m.hist.Len() + m.expo.Len()
}

func (m Map) Has(id identity.Stream) bool {
	_, nok := m.nums.Load(id)
	_, hok := m.hist.Load(id)
	_, eok := m.expo.Load(id)
	return nok || hok || eok
}

func (m Map) Delete(id identity.Stream) {
	m.nums.Delete(id)
	m.hist.Delete(id)
	m.expo.Delete(id)
}
