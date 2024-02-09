package scrubbingprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "scrubbing"
	// The stability level of the processor.
	stability = component.StabilityLevelDevelopment
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func NewFactory() processor.Factory {
	return processor.NewFactory(typeStr, createDefaultConfig,
		processor.WithLogs(createLogsProcessor,
			stability))
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	sp, err := newScrubbingProcessorProcessor(set.Logger, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		sp.ProcessLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}
