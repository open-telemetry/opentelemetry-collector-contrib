package netflowreceiver

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var (
	typeStr = component.MustNewType("netflow")
)

const (
	defaultSockets   = 1
	defaultWorkers   = 2
	defaultQueueSize = 1_000_000
)

// NewFactory creates a factory for netflow receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelAlpha))
}

func createDefaultConfig() component.Config {
	return &Config{
		Listeners: []ListenerConfig{
			{
				Scheme:    "sflow",
				Port:      6343,
				Sockets:   defaultSockets,
				Workers:   defaultWorkers,
				QueueSize: defaultQueueSize,
			},
			{
				Scheme:    "netflow",
				Port:      2055,
				Sockets:   defaultSockets,
				Workers:   defaultWorkers,
				QueueSize: defaultQueueSize,
			},
		},
	}
}

func createLogsReceiver(_ context.Context, params receiver.CreateSettings, cfg component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	logger := params.Logger
	conf := cfg.(*Config)

	nr := &netflowReceiver{
		logger:      logger,
		logConsumer: consumer,
		config:      conf,
	}

	return nr, nil
}
