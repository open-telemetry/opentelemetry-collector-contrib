package redisreceiver

import (
	"github.com/go-redis/redis/v7"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"go.uber.org/zap"
)

type redisReceiver struct {
	logger    *zap.Logger
	config    *Config
	consumer  consumer.MetricsConsumer
	scheduler *scheduler
}

var _ receiver.MetricsReceiver = (*redisReceiver)(nil)

func newRedisReceiver(logger *zap.Logger, config *Config, consumer consumer.MetricsConsumer) *redisReceiver {
	return &redisReceiver{
		logger:   logger,
		config:   config,
		consumer: consumer,
	}
}

func (r redisReceiver) Start(host component.Host) error {
	client := newRedisClient(&redis.Options{
		Addr:     r.config.Endpoint, // e.g. "localhost:6379"
		Password: r.config.Password,
	})
	redisTickable := newRedisTickable(host.Context(), client, r.consumer)
	r.scheduler = newScheduler(10, []tickable{redisTickable})
	return r.scheduler.start()
}

func (r redisReceiver) Shutdown() error {
	return r.scheduler.stop()
}
