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
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maybe"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/telemetry"
)

var _ processor.Metrics = (*Processor)(nil)

type Processor struct {
	next consumer.Metrics

	log    *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc

	aggr  streams.Aggregator[data.Number]
	stale maybe.Ptr[staleness.Staleness[data.Number]]

	mtx sync.Mutex
}

func newProcessor(cfg *Config, log *zap.Logger, meter metric.Meter, next consumer.Metrics) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	proc := Processor{
		log:    log,
		ctx:    ctx,
		cancel: cancel,
		next:   next,
	}

	tel := telemetry.New(meter)

	var dps streams.Map[data.Number]
	dps = delta.New[data.Number]()
	dps = telemetry.ObserveItems(dps, &tel.Metrics)

	if cfg.MaxStale > 0 {
		tel.WithStale(meter, cfg.MaxStale)
		stale := maybe.Some(staleness.NewStaleness(cfg.MaxStale, dps))
		proc.stale = stale
		dps, _ = stale.Try()
	}
	if cfg.MaxStreams > 0 {
		tel.WithLimit(meter, int64(cfg.MaxStreams))
		lim := streams.Limit(dps, cfg.MaxStreams)
		if stale, ok := proc.stale.Try(); ok {
			lim.Evictor = stale
		}
		dps = lim
	}

	dps = telemetry.ObserveNonFatal(dps, &tel.Metrics)

	proc.aggr = streams.IntoAggregator(dps)
	return &proc
}

func (p *Processor) Start(_ context.Context, _ component.Host) error {
	stale, ok := p.stale.Try()
	if !ok {
		return nil
	}

	go func() {
		tick := time.NewTicker(time.Minute)
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-tick.C:
				p.mtx.Lock()
				stale.ExpireOldEntries()
				p.mtx.Unlock()
			}
		}
	}()
	return nil
}

func (p *Processor) Shutdown(_ context.Context) error {
	p.cancel()
	return nil
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *Processor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var errs error

	metrics.Each(md, func(m metrics.Metric) {
		switch m.Type() {
		case pmetric.MetricTypeSum:
			sum := m.Sum()
			if sum.AggregationTemporality() == pmetric.AggregationTemporalityDelta {
				err := streams.Aggregate[data.Number](metrics.Sum(m), p.aggr)
				errs = errors.Join(errs, err)
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
		case pmetric.MetricTypeHistogram:
			// TODO
		case pmetric.MetricTypeExponentialHistogram:
			// TODO
		}
	})

	if errs != nil {
		return errs
	}

	return p.next.ConsumeMetrics(ctx, md)
}
