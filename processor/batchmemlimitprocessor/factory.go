package batchmemlimitprocessor

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

const (
	// The value of "type" key in configuration.
	typeStr = "batchmemorylimit"
	// The stability level of the processor.
	stability = component.StabilityLevelDevelopment
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(typeStr, createDefaultConfig,
		component.WithLogsProcessor(createMemoryLimiterProcessor,
			stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(component.NewID(typeStr)),

		Timeout:        1 * time.Second,
		SendMemorySize: 1000000,
		SendBatchSize:  1000,
	}
}

func createMemoryLimiterProcessor(
	_ context.Context,
	set component.ProcessorCreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (component.LogsProcessor, error) {
	return newBatchMemoryLimiterProcessor(nextConsumer, set.Logger, cfg.(*Config)), nil
}
