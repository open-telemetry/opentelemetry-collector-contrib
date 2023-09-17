package pytransform

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/pytransformprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

// NewFactory creates a factory for the routing processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		// pass event back to otel without changes
		Code: "send(event)",
	}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {

	ep, err := getEmbeddedPython()
	if err != nil {
		return nil, err
	}

	return newLogsProcessor(ctx, set.Logger, cfg.(*Config), ep, nextConsumer), nil
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {

	ep, err := getEmbeddedPython()
	if err != nil {
		return nil, err
	}

	return newMetricsProcessor(ctx, set.Logger, cfg.(*Config), ep, nextConsumer), nil
}

func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {

	ep, err := getEmbeddedPython()
	if err != nil {
		return nil, err
	}

	return newTraceProcessor(ctx, set.Logger, cfg.(*Config), ep, nextConsumer), nil
}
