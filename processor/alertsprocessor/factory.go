package alertsprocessor

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const typeStr = "alertsprocessor"

func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithMetrics(createMetrics, component.StabilityLevelAlpha),
		processor.WithLogs(createLogs, component.StabilityLevelAlpha),
		processor.WithTraces(createTraces, component.StabilityLevelAlpha),
	)
}

func createMetrics(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Metrics) (processor.Metrics, error) {
	c := cfg.(*Config)
	if err := c.Validate(); err != nil { return nil, err }
	p, err := newProcessor(ctx, set, c, next, nil, nil)
	if err != nil { return nil, err }
	return processorhelper.NewMetrics(
		ctx, set, cfg, next, p.processMetrics,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown),
		processorhelper.WithTimeouts(processorhelper.Timeouts{Process: c.Evaluation.Timeout, Shutdown: 5 * time.Second}),
	)
}

func createLogs(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Logs) (processor.Logs, error) {
	c := cfg.(*Config)
	if err := c.Validate(); err != nil { return nil, err }
	p, err := newProcessor(ctx, set, c, nil, next, nil)
	if err != nil { return nil, err }
	return processorhelper.NewLogs(
		ctx, set, cfg, next, p.processLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown),
		processorhelper.WithTimeouts(processorhelper.Timeouts{Process: c.Evaluation.Timeout, Shutdown: 5 * time.Second}),
	)
}

func createTraces(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	c := cfg.(*Config)
	if err := c.Validate(); err != nil { return nil, err }
	p, err := newProcessor(ctx, set, c, nil, nil, next)
	if err != nil { return nil, err }
	return processorhelper.NewTraces(
		ctx, set, cfg, next, p.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown),
		processorhelper.WithTimeouts(processorhelper.Timeouts{Process: c.Evaluation.Timeout, Shutdown: 5 * time.Second}),
	)
}
