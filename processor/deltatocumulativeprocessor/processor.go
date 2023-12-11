package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"context"

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"

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

	aggr metrics.Aggregator
	emit Emitter
}

func newProcessor(cfg *Config, log *zap.Logger, next consumer.Metrics) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	aggr := streams.NewTracker(delta.Aggregator())

	emit := Emitter{
		Interval: cfg.Interval,

		dest: next,
		aggr: aggr,
		log:  log,
	}

	proc := Processor{
		log:    log,
		ctx:    ctx,
		cancel: cancel,
		next:   next,

		aggr: aggr,
		emit: emit,
	}

	return &proc
}

func (p *Processor) Start(ctx context.Context, host component.Host) error {
	go p.emit.Run(p.ctx)
	return nil
}

func (p *Processor) Shutdown(ctx context.Context) error {
	p.cancel()
	return nil
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *Processor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	metrics.Filter(md, func(m metrics.Metric) bool {
		switch m.Type() {
		case pmetric.MetricTypeSum:
			if m.Sum().AggregationTemporality() == pmetric.AggregationTemporalityDelta {
				p.aggr.Consume(m)
				return true
			}
		case pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram:
			// TODO: aggregate
			return true
		}

		return false
	})

	return p.next.ConsumeMetrics(ctx, md)
}
