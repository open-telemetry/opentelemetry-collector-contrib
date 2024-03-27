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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

var _ processor.Metrics = (*Processor)(nil)

type Processor struct {
	next consumer.Metrics

	log    *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc

	aggr  streams.Aggregator[data.Number]
	stale *staleness.Staleness[data.Number]

	mtx sync.Mutex
}

func newProcessor(cfg *Config, log *zap.Logger, next consumer.Metrics) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	proc := Processor{
		log:    log,
		ctx:    ctx,
		cancel: cancel,
		next:   next,
	}

	var dps streams.Map[data.Number]
	dps = delta.New[data.Number]()

	if cfg.MaxStale > 0 {
		stale := staleness.NewStaleness(cfg.MaxStale, dps)
		proc.stale = stale
		dps = stale
	}
	if cfg.MaxStreams > 0 {
		lim := streams.Limit(dps, cfg.MaxStreams)
		if proc.stale != nil {
			lim.Evictor = proc.stale
		}
		dps = lim
	}

	proc.aggr = streams.IntoAggregator(dps)
	return &proc
}

func (p *Processor) Start(_ context.Context, _ component.Host) error {
	if p.stale == nil {
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
				p.stale.ExpireOldEntries()
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
