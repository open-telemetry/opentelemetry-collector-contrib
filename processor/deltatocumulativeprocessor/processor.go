// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

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

	aggr streams.Aggregator[data.Number]
	exp  *streams.Expiry[data.Number]

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
		exp := streams.ExpireAfter(dps, cfg.MaxStale)
		proc.exp = &exp
		dps = &exp
	}

	proc.aggr = streams.IntoAggregator(dps)
	return &proc
}

func (p *Processor) Start(_ context.Context, _ component.Host) error {
	if p.exp != nil {
		go func() {
			for {
				p.mtx.Lock()
				next := p.exp.ExpireOldEntries()
				p.mtx.Unlock()

				select {
				case <-next:
				case <-p.ctx.Done():
					return
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
