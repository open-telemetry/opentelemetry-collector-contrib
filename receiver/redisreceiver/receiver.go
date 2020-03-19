package redisreceiver

import (
	"github.com/go-redis/redis/v7"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/interval"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"go.uber.org/zap"
)

type redisReceiver struct {
	logger         *zap.Logger
	config         *Config
	consumer       consumer.MetricsConsumer
	intervalRunner *interval.Runner
}

var _ receiver.MetricsReceiver = (*redisReceiver)(nil)

func newRedisReceiver(
	logger *zap.Logger,
	config *Config,
	consumer consumer.MetricsConsumer,
) *redisReceiver {
	return &redisReceiver{
		logger:   logger,
		config:   config,
		consumer: consumer,
	}
}

func (r redisReceiver) Start(host component.Host) error {
	client := newRedisClient(&redis.Options{
		Addr:     r.config.Endpoint,
		Password: r.config.Password,
	})
	redisRunnable := newRedisRunnable(host.Context(), client, r.consumer, r.logger)
	r.intervalRunner = interval.NewRunner(r.config.RefreshInterval, redisRunnable)

	go func() {
		if err := r.intervalRunner.Start(); err != nil {
			host.ReportFatalError(err)
		}
	}()

	return nil
}

func (r redisReceiver) Shutdown() error {
	r.intervalRunner.Stop()
	return nil
}
