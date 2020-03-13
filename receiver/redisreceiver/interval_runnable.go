package redisreceiver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"go.uber.org/zap"
)

type intervalRunnable interface {
	setup() error
	run() error
}

var _ intervalRunnable = (*redisRunnable)(nil)

type redisRunnable struct {
	ctx             context.Context
	metricsConsumer consumer.MetricsConsumer
	metricsService  *metricsService
	redisMetrics    []*redisMetric
}

func newRedisRunnable(
	ctx context.Context,
	client client,
	metricsConsumer consumer.MetricsConsumer,
	logger *zap.Logger,
) *redisRunnable {
	return &redisRunnable{
		ctx:             ctx,
		metricsService:  newMetricsService(client, logger),
		metricsConsumer: metricsConsumer,
	}
}

func (r *redisRunnable) setup() error {
	r.redisMetrics = getDefaultRedisMetrics()
	return nil
}

func (r *redisRunnable) run() error {
	md, err := r.metricsService.getMetricsData(r.redisMetrics)
	if err != nil {
		return err
	}
	return r.metricsConsumer.ConsumeMetricsData(r.ctx, *md)
}
