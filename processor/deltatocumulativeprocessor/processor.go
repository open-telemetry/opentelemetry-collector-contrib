package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"context"
	"errors"

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
}

func newProcessor(cfg *Config, log *zap.Logger) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	proc := Processor{
		log:    log,
		ctx:    ctx,
		cancel: cancel,
		aggr:   streams.NewTracker(delta.Aggregator()),
	}

	return &proc
}

func (p *Processor) Start(ctx context.Context, host component.Host) error {
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
	var errs error

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

	if err := p.next.ConsumeMetrics(ctx, md); err != nil {
		errs = errors.Join(err)
	}
	return errs
}
