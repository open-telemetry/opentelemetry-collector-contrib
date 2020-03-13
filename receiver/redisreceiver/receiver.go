package redisreceiver

import (
	"sync"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/oterr"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"go.uber.org/zap"
)

type redisReceiver struct {
	sync.Mutex
	logger         *zap.Logger
	config         *Config
	consumer       consumer.MetricsConsumer
	intervalRunner *intervalRunner

	startOnce sync.Once
	stopOnce  sync.Once
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
	r.Lock()
	defer r.Unlock()

	err := oterr.ErrAlreadyStarted

	r.startOnce.Do(func() {
		err = nil

		client := newRedisClient(&redis.Options{
			Addr:     r.config.Endpoint,
			Password: r.config.Password,
		})
		redisRunnable := newRedisRunnable(host.Context(), client, r.consumer, r.logger)
		r.intervalRunner = newIntervalRunner(10 * time.Second, redisRunnable)

		go func() {
			if err := r.intervalRunner.start(); err != nil {
				host.ReportFatalError(err)
			}
		}()
	})

	return err
}

func (r redisReceiver) Shutdown() error {
	r.Lock()
	defer r.Unlock()
	err := oterr.ErrAlreadyStopped
	r.stopOnce.Do(func() {
		err = nil
		r.intervalRunner.stop()
	})
	return err
}
