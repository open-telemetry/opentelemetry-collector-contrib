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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maybe"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
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

	sums Pipeline[data.Number]
	expo Pipeline[data.ExpHistogram]
	hist Pipeline[data.Histogram]

	mtx sync.Mutex
}

func newProcessor(cfg *Config, log *zap.Logger, telb *metadata.TelemetryBuilder, next consumer.Metrics) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	tel := telemetry.New(telb)
	proc := Processor{
		log:    log,
		ctx:    ctx,
		cancel: cancel,
		next:   next,

		sums: pipeline[data.Number](cfg, &tel),
		expo: pipeline[data.ExpHistogram](cfg, &tel),
		hist: pipeline[data.Histogram](cfg, &tel),
	}

	return &proc
}

type Pipeline[D data.Point[D]] struct {
	aggr  streams.Aggregator[D]
	stale maybe.Ptr[staleness.Staleness[D]]
}

func pipeline[D data.Point[D]](cfg *Config, tel *telemetry.Telemetry) Pipeline[D] {
	var pipe Pipeline[D]

	var dps streams.Map[D]
	dps = delta.New[D]()
	dps = telemetry.ObserveItems(dps, &tel.Metrics)

	if cfg.MaxStale > 0 {
		tel.WithStale(cfg.MaxStale)
		stale := maybe.Some(staleness.NewStaleness(cfg.MaxStale, dps))
		pipe.stale = stale
		dps, _ = stale.Try()
	}
	if cfg.MaxStreams > 0 {
		tel.WithLimit(int64(cfg.MaxStreams))
		lim := streams.Limit(dps, cfg.MaxStreams)
		if stale, ok := pipe.stale.Try(); ok {
			lim.Evictor = stale
		}
		dps = lim
	}

	dps = telemetry.ObserveNonFatal(dps, &tel.Metrics)

	pipe.aggr = streams.IntoAggregator(dps)
	return pipe
}

func (p *Processor) Start(_ context.Context, _ component.Host) error {
	sums, sok := p.sums.stale.Try()
	expo, eok := p.expo.stale.Try()
	hist, hok := p.hist.stale.Try()
	if !(sok && eok && hok) {
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
				sums.ExpireOldEntries()
				expo.ExpireOldEntries()
				hist.ExpireOldEntries()
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
	if err := context.Cause(p.ctx); err != nil {
		return err
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()

	var errs error
	metrics.Filter(md, func(m metrics.Metric) bool {
		var n int
		//exhaustive:enforce
		switch m.Type() {
		case pmetric.MetricTypeGauge:
			n = m.Gauge().DataPoints().Len()
		case pmetric.MetricTypeSum:
			sum := m.Sum()
			if sum.AggregationTemporality() == pmetric.AggregationTemporalityDelta {
				err := streams.Apply(metrics.Sum(m), p.sums.aggr.Aggregate)
				errs = errors.Join(errs, err)
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
			n = sum.DataPoints().Len()
		case pmetric.MetricTypeHistogram:
			hist := m.Histogram()
			if hist.AggregationTemporality() == pmetric.AggregationTemporalityDelta {
				err := streams.Apply(metrics.Histogram(m), p.hist.aggr.Aggregate)
				errs = errors.Join(errs, err)
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
			n = hist.DataPoints().Len()
		case pmetric.MetricTypeExponentialHistogram:
			expo := m.ExponentialHistogram()
			if expo.AggregationTemporality() == pmetric.AggregationTemporalityDelta {
				err := streams.Apply(metrics.ExpHistogram(m), p.expo.aggr.Aggregate)
				errs = errors.Join(errs, err)
				expo.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
			n = expo.DataPoints().Len()
		case pmetric.MetricTypeSummary:
			n = m.Summary().DataPoints().Len()
		}
		return n > 0
	})
	if errs != nil {
		return errs
	}

	if md.MetricCount() == 0 {
		return nil
	}
	return p.next.ConsumeMetrics(ctx, md)
}
