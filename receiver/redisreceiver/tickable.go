package redisreceiver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector/consumer"
)

type tickable interface {
	setup() error
	run() error
}

var _ tickable = (*redisTickable)(nil)

type redisTickable struct {
	ctx             context.Context
	metricsConsumer consumer.MetricsConsumer
	metricsService  *metricsService
	redisMetrics    []*redisMetric
}

func newRedisTickable(ctx context.Context, client client, metricsConsumer consumer.MetricsConsumer) *redisTickable {
	return &redisTickable{ctx: ctx, metricsService: newMetricsService(client), metricsConsumer: metricsConsumer}
}

func (r *redisTickable) setup() error {
	r.redisMetrics = getDefaultRedisMetrics()
	return nil
}

func (r *redisTickable) run() error {
	md, err := r.metricsService.getMetricsData(r.redisMetrics)
	if err != nil {
		return err
	}
	return r.metricsConsumer.ConsumeMetricsData(r.ctx, *md)
}
