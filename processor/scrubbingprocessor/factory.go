package scrubbingprocessor

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/scrubbingprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type, createDefaultConfig,
		processor.WithLogs(
			createLogsProcessor,
			metadata.LogsStability,
		),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	sp, err := newScrubbingProcessorProcessor(set.Logger, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		sp.ProcessLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}
