// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

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
	telemetry "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/lineartelemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

var _ processor.Metrics = (*Linear)(nil)

type Linear struct {
	next processor.Metrics
	cfg  Config

	state state
	mtx   sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	stale staleness.Tracker
	tel   telemetry.Metrics
}

func newLinear(cfg *Config, tel telemetry.Metrics, next processor.Metrics) *Linear {
	ctx, cancel := context.WithCancel(context.Background())

	proc := Linear{
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
		tel:   tel,
	}

	tel.WithTracked(proc.state.Len)
	cfg.Metrics(tel)

	return &proc
}

func (p *Linear) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	now := time.Now()

	const (
		keep = true
		drop = false
	)

	// possible errors encountered while aggregating.
	// errors.Join-ed []streams.Error
	var errs error

	metrics.Filter(md, func(m metrics.Metric) bool {
		if m.AggregationTemporality() != pmetric.AggregationTemporalityDelta {
			return keep
		}

		// NOTE: to make review and migration easier, below only does sums for now.
		// all other datatypes are handled by older code, which is called after this.
		//
		// TODO: implement other datatypes here
		if m.Type() != pmetric.MetricTypeSum {
			return keep
		}

		sum := metrics.Sum(m)
		state := p.state.nums

		// apply fn to each dp in stream. if fn's err != nil, dp is removed from stream
		err := streams.Apply(sum, func(id identity.Stream, dp data.Number) (data.Number, error) {
			acc, ok := state.Load(id)
			// if at stream limit and stream not seen before, reject
			if !ok && p.state.Len() >= p.cfg.MaxStreams {
				p.tel.Datapoints().Inc(ctx, telemetry.Error("limit"))
				return dp, streams.Drop
			}

			// stream is alive, update stale tracker
			p.stale.Refresh(now, id)

			acc, err := func() (data.Number, error) {
				if !ok {
					// new stream: there is no existing aggregation, so start new with current dp
					return dp.Clone(), nil
				}
				// tracked stream: add incoming delta dp to existing cumulative aggregation
				return acc, delta.AccumulateInto(acc, dp)
			}()
			// aggregation failed, record as metric and drop datapoint
			if err != nil {
				p.tel.Datapoints().Inc(ctx, telemetry.Cause(err))
				return acc, streams.Drop
			}

			// store aggregated result in state and return
			p.tel.Datapoints().Inc(ctx)
			_ = state.Store(id, acc)
			return acc, nil
		})
		errs = errors.Join(errs, err)

		// all remaining datapoints are cumulative
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		// if no datapoints remain, drop now-empty metric
		return sum.Len() > 0
	})
	if errs != nil {
		return errs
	}

	// no need to continue pipeline if we dropped all metrics
	if md.MetricCount() == 0 {
		return nil
	}
	return p.next.ConsumeMetrics(ctx, md)
}

func (p *Linear) Start(_ context.Context, _ component.Host) error {
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

func (p *Linear) Shutdown(_ context.Context) error {
	p.cancel()
	return nil
}

func (p *Linear) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

type Metric[T data.Point[T]] interface {
	metrics.Filterable[T]
	SetAggregationTemporality(pmetric.AggregationTemporality)
}

// state keeps a cumulative value, aggregated over time, per stream
type state struct {
	nums streams.Map[data.Number]

	// future use
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
