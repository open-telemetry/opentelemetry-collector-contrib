package redisreceiver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector/config/configerror"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"go.uber.org/zap"
)

const (
	typeStr = "redis"
)

var _ receiver.Factory = &Factory{}

type Factory struct {
}

func (f Factory) Type() string {
	return typeStr
}

func (f Factory) CustomUnmarshaler() receiver.CustomUnmarshaler {
	return nil
}

func (f Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{}
}

func (f Factory) CreateTraceReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	nextConsumer consumer.TraceConsumer,
) (receiver.TraceReceiver, error) {
	// No trace receiver for now.
	return nil, configerror.ErrDataTypeIsNotSupported
}

func (f Factory) CreateMetricsReceiver(
	logger *zap.Logger,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (receiver.MetricsReceiver, error) {
	return newRedisReceiver(logger, cfg.(*Config), consumer), nil
}
