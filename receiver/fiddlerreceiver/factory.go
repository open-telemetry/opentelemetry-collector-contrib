package fiddlerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var (
	typeStr = component.MustNewType("fiddler")
)

const (
	defaultInterval = "30m"
)

// NewFactory creates a factory for Fiddler receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelDevelopment))
}

func createDefaultConfig() component.Config {
	return &Config{
		Interval: defaultInterval,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)
	return newFiddlerReceiver(cfg, consumer), nil
}
