package redisreceiver

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/interval"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"go.uber.org/zap"
)

var _ interval.Runnable = (*redisRunnable)(nil)

// Runs intermittently, fetching info from Redis, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type redisRunnable struct {
	ctx             context.Context
	metricsConsumer consumer.MetricsConsumer
	redisSvc        *redisSvc
	redisMetrics    []*redisMetric
	logger          *zap.Logger
}

func newRedisRunnable(
	ctx context.Context,
	client client,
	metricsConsumer consumer.MetricsConsumer,
	logger *zap.Logger,
) *redisRunnable {
	return &redisRunnable{
		ctx:             ctx,
		redisSvc:        newRedisSvc(client),
		metricsConsumer: metricsConsumer,
		logger:          logger,
	}
}

// Builds a data structure of all of the keys, types, converters and such
// to later extract data from Redis.
func (r *redisRunnable) Setup() error {
	r.redisMetrics = getDefaultRedisMetrics()
	return nil
}

func (r *redisRunnable) Run() error {
	info, err := r.redisSvc.info()
	if err != nil {
		return err
	}
	now := time.Now()
	metrics, warnings := buildFixedProtoMetrics(info, r.redisMetrics, now)
	if warnings != nil {
		r.logger.Warn("errors parsing redis string", zap.Errors("parsing errors", warnings))
	}

	keyspaceMetrics, warnings := buildKeyspaceProtoMetrics(info, now)
	metrics = append(metrics, keyspaceMetrics...)
	if warnings != nil {
		r.logger.Warn("errors parsing keyspace string", zap.Errors("parsing errors", warnings))
	}

	md := newMetricsData(metrics)
	return r.metricsConsumer.ConsumeMetricsData(r.ctx, *md)
}
