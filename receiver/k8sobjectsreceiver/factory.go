package k8sobjectreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

const (
	typeStr   = "k8sobjects"
	stability = component.StabilityLevelAlpha
)

func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithLogsReceiver(
			createLogsReceiver,
			stability,
		),
	)
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
	}
}

func createLogsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Logs,
) (component.LogsReceiver, error) {
	rcfg := cfg.(*Config)
	return newReceiver(params, rcfg, consumer)
}
