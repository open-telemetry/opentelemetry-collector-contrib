package batchmemlimitprocessor

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// The value of "type" key in configuration.
	typeStr = "batchmemorylimit"
	// The stability level of the processor.
	stability = component.StabilityLevelDevelopment
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func NewFactory() processor.Factory {
	return processor.NewFactory(typeStr, createDefaultConfig,
		processor.WithLogs(createMemoryLimiterProcessor,
			stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		Timeout:        1 * time.Second,
		SendMemorySize: 1000000,
		SendBatchSize:  1000,
	}
}

func createMemoryLimiterProcessor(
	_ context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	return newBatchMemoryLimiterProcessor(nextConsumer, set.Logger, cfg.(*Config)), nil
}
