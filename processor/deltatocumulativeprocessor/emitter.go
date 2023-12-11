package deltatocumulativeprocessor

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
)

type Emitter struct {
	log *zap.Logger

	aggr metrics.Aggregator
	dest consumer.Metrics

	Interval time.Duration
}

func (e *Emitter) Run(ctx context.Context) {
	t := time.NewTicker(e.Interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(ctx, e.Interval)
			go func() {
				mm := e.aggr.Export()
				if err := e.dest.ConsumeMetrics(ctx, mm.Merge()); err != nil {
					e.log.Error("exporting aggregated values", zap.Error(err))
				}
				cancel()
			}()
		}
	}
}
