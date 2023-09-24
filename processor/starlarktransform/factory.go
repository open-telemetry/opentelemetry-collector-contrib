package starlarktransform

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/starlarktransformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/starlarktransformprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/starlarktransformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/starlarktransformprocessor/internal/traces"
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
		Code: "def transform(e): return json.decode(e)",
	}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config type")
	}

	return logs.NewProcessor(ctx, set.Logger, config.Code, nextConsumer), nil
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config type")
	}

	return metrics.NewProcessor(ctx, set.Logger, config.Code, nextConsumer), nil
}

func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config type")
	}

	return traces.NewProcessor(ctx, set.Logger, config.Code, nextConsumer), nil
}
