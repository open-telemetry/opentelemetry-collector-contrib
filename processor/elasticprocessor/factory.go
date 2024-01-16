package elasticconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	typeStr = "elastic"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Filter processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, component.StabilityLevelAlpha),
		processor.WithLogs(createLogsProcessor, component.StabilityLevelAlpha),
		processor.WithTraces(createTracesProcessor, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	noopProcessor := func(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) { return md, nil }
	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		noopProcessor,
		processorhelper.WithCapabilities(processorCapabilities))
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	noopProcessor := func(_ context.Context, ld plog.Logs) (plog.Logs, error) { return ld, nil }
	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		noopProcessor,
		processorhelper.WithCapabilities(processorCapabilities))
}

func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	noopProcessor := func(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) { return td, nil }
	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		noopProcessor,
		processorhelper.WithCapabilities(processorCapabilities))
}
