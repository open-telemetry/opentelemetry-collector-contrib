package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/identity"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

var _ processor.Metrics = (*Processor)(nil)

type Processor struct {
	next consumer.Metrics

	log    *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc

	aggr delta.Aggregator
}

func newProcessor(cfg *Config, log *zap.Logger) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	proc := Processor{
		log:    log,
		ctx:    ctx,
		cancel: cancel,
		aggr:   delta.NewGuard(delta.NewSum()),
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
	filterMetrics(md.ResourceMetrics(), func(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) bool {
		switch m.Type() {
		case pmetric.MetricTypeSum:
			sum := m.Sum()
			if sum.AggregationTemporality() != pmetric.AggregationTemporalityDelta {
				return false
			}
			id := identity.OfMetric(rm.Resource(), sm.Scope(), m)
			if err := p.aggr.Aggregate(id, sum.DataPoints()); err != nil {
				errs = errors.Join(errs, err)
			}
			return true
		case pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram:
			panic("todo")
		}

		return false
	})

	if err := p.next.ConsumeMetrics(ctx, md); err != nil {
		errs = errors.Join(err)
	}
	return errs
}

func filterMetrics(resourceMetrics pmetric.ResourceMetricsSlice, fn func(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) bool) {
	resourceMetrics.RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				return fn(rm, sm, m)
			})
			return false
		})
		return false
	})
}
