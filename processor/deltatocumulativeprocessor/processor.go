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

	state state
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
		state: state{
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

	const (
		keep = true
		drop = false
	)

	var errs error
	metrics.Filter(md, func(m metrics.Metric) bool {
		if m.AggregationTemporality() != pmetric.AggregationTemporalityDelta {
			return keep
		}

		var each aggrFunc
		switch m := m.Typed().(type) {
		case metrics.Sum:
			each = use(m, p.state.nums)
		case metrics.Histogram:
			each = use(m, p.state.hist)
		case metrics.ExpHistogram:
			each = use(m, p.state.expo)
		}

		var do = drop
		err := each(func(id identity.Stream, aggr func() error) error {
			// if stream not seen before and stream limit is reached, reject
			if !p.state.Has(id) && p.state.Len() >= p.cfg.MaxStreams {
				// TODO: record metric
				return streams.Drop
			}
			do = keep

			// stream is alive, refresh it
			p.stale.Refresh(now, id)

			// accumulate current dp into state
			return aggr()
		})
		errs = errors.Join(errs, err)
		return do
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

// aggrFunc calls `do` for datapoint of a metric, giving the caller the
// opportunity to decide whether to perform aggregation in a type-agnostic way,
// while keeping underlying strong typing.
//
// if `aggr` is called, the current datapoint is accumulated into the streams
// cumulative state.
//
// if any error is returned, the current datapoint is dropped, the error
// collected and eventually returned. [streams.Drop] silently drops a datapoint
type aggrFunc func(do func(id identity.Stream, aggr func() error) error) error

// use returns an AggrFunc for the given metric, accumulating into the given state,
// given `do` calls its `aggr`
func use[T data.Typed[T], M metric[T]](m M, state streams.Map[T]) aggrFunc {
	return func(do func(id identity.Stream, aggr func() error) error) error {
		return streams.Apply(m, func(id identity.Stream, dp T) (T, error) {
			// load previously aggregated state for this stream
			acc, ok := state.Load(id)

			// to be invoked by caller if accumulation is desired
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

			// update state to possibly changed value
			state.Store(id, acc)

			// store new value in output metrics slice
			return acc, err
		})
	}
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

type metric[T data.Point[T]] interface {
	metrics.Filterable[T]
	SetAggregationTemporality(pmetric.AggregationTemporality)
}

// state keeps a cumulative value, aggregated over time, per stream
type state struct {
	nums streams.Map[data.Number]
	hist streams.Map[data.Histogram]
	expo streams.Map[data.ExpHistogram]
}

func (m state) Len() int {
	return m.nums.Len() + m.hist.Len() + m.expo.Len()
}

func (m state) Has(id identity.Stream) bool {
	_, nok := m.nums.Load(id)
	_, hok := m.hist.Load(id)
	_, eok := m.expo.Load(id)
	return nok || hok || eok
}

func (m state) Delete(id identity.Stream) {
	m.nums.Delete(id)
	m.hist.Delete(id)
	m.expo.Delete(id)
}
