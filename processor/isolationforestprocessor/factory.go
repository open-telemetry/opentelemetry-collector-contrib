// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// factory.go - OpenTelemetry Collector factory implementation
package isolationforestprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

const (
	typeStr   = "isolationforest"
	stability = component.StabilityLevelAlpha
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
		processor.WithMetrics(createMetricsProcessor, stability),
		processor.WithLogs(createLogsProcessor, stability),
	)
}

func createTracesProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	processorCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("configuration is not of type *Config")
	}

	if err := processorCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	set.Logger.Info("Creating isolation forest traces processor",
		zap.String("processor_id", set.ID.String()),
		zap.Int("forest_size", processorCfg.ForestSize),
		zap.String("mode", processorCfg.Mode),
		zap.Float64("threshold", processorCfg.Threshold),
	)

	proc, err := newIsolationForestProcessor(processorCfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create processor: %w", err)
	}

	return &tracesProcessor{
		isolationForestProcessor: proc,
		nextConsumer:             nextConsumer,
		logger:                   set.Logger,
	}, nil
}

func createMetricsProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processorCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("configuration is not of type *Config")
	}

	if err := processorCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	set.Logger.Info("Creating isolation forest metrics processor",
		zap.String("processor_id", set.ID.String()),
		zap.Int("forest_size", processorCfg.ForestSize),
		zap.String("mode", processorCfg.Mode),
	)

	proc, err := newIsolationForestProcessor(processorCfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create processor: %w", err)
	}

	return &metricsProcessor{
		isolationForestProcessor: proc,
		nextConsumer:             nextConsumer,
		logger:                   set.Logger,
	}, nil
}

func createLogsProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	processorCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("configuration is not of type *Config")
	}

	if err := processorCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	set.Logger.Info("Creating isolation forest logs processor",
		zap.String("processor_id", set.ID.String()),
		zap.Int("forest_size", processorCfg.ForestSize),
		zap.String("mode", processorCfg.Mode),
	)

	proc, err := newIsolationForestProcessor(processorCfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create processor: %w", err)
	}

	return &logsProcessor{
		isolationForestProcessor: proc,
		nextConsumer:             nextConsumer,
		logger:                   set.Logger,
	}, nil
}

type tracesProcessor struct {
	*isolationForestProcessor
	nextConsumer consumer.Traces
	logger       *zap.Logger
}

func (tp *tracesProcessor) Start(ctx context.Context, host component.Host) error {
	return tp.isolationForestProcessor.Start(ctx, host)
}

func (tp *tracesProcessor) Shutdown(ctx context.Context) error {
	return tp.isolationForestProcessor.Shutdown(ctx)
}

func (tp *tracesProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	processedTraces, err := tp.processTraces(ctx, td)
	if err != nil {
		return err
	}
	return tp.nextConsumer.ConsumeTraces(ctx, processedTraces)
}

func (*tracesProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

type metricsProcessor struct {
	*isolationForestProcessor
	nextConsumer consumer.Metrics
	logger       *zap.Logger
}

func (mp *metricsProcessor) Start(ctx context.Context, host component.Host) error {
	return mp.isolationForestProcessor.Start(ctx, host)
}

func (mp *metricsProcessor) Shutdown(ctx context.Context) error {
	return mp.isolationForestProcessor.Shutdown(ctx)
}

func (mp *metricsProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	processedMetrics, err := mp.processMetrics(ctx, md)
	if err != nil {
		return err
	}
	return mp.nextConsumer.ConsumeMetrics(ctx, processedMetrics)
}

func (*metricsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

type logsProcessor struct {
	*isolationForestProcessor
	nextConsumer consumer.Logs
	logger       *zap.Logger
}

func (lp *logsProcessor) Start(ctx context.Context, host component.Host) error {
	return lp.isolationForestProcessor.Start(ctx, host)
}

func (lp *logsProcessor) Shutdown(ctx context.Context) error {
	return lp.isolationForestProcessor.Shutdown(ctx)
}

func (lp *logsProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	processedLogs, err := lp.processLogs(ctx, ld)
	if err != nil {
		return err
	}
	return lp.nextConsumer.ConsumeLogs(ctx, processedLogs)
}

func (*logsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}
