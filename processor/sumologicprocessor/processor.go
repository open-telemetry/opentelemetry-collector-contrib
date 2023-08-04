// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

type sumologicSubprocessor interface {
	processLogs(plog.Logs) error
	processMetrics(pmetric.Metrics) error
	processTraces(ptrace.Traces) error
	isEnabled() bool
	ConfigPropertyName() string
}

type sumologicProcessor struct {
	logger        *zap.Logger
	subprocessors []sumologicSubprocessor
}

func newsumologicProcessor(set processor.CreateSettings, config *Config) *sumologicProcessor {
	cloudNamespaceProcessor := newCloudNamespaceProcessor(config.AddCloudNamespace)

	translateAttributesProcessor := newTranslateAttributesProcessor(config.TranslateAttributes)

	translateTelegrafMetricsProcessor := newTranslateTelegrafMetricsProcessor(config.TranslateTelegrafAttributes)

	nestingProcessor := newNestingProcessor(config.NestAttributes)

	aggregateAttributesProcessor := newAggregateAttributesProcessor(config.AggregateAttributes)

	logFieldsConversionProcessor := newLogFieldConversionProcessor(config.LogFieldsAttributes)

	translateDockerMetricsProcessor := newTranslateDockerMetricsProcessor(config.TranslateDockerMetrics)

	processors := []sumologicSubprocessor{
		cloudNamespaceProcessor,
		translateAttributesProcessor,
		translateTelegrafMetricsProcessor,
		nestingProcessor,
		aggregateAttributesProcessor,
		logFieldsConversionProcessor,
		translateDockerMetricsProcessor,
	}

	processor := &sumologicProcessor{
		logger:        set.Logger,
		subprocessors: processors,
	}

	return processor
}

func (processor *sumologicProcessor) start(_ context.Context, _ component.Host) error {
	procs := processor.subprocessors
	processor.logger.Info(
		"Sumo Logic Processor has started.",
		zap.Bool(procs[0].ConfigPropertyName(), procs[0].isEnabled()),
		zap.Bool(procs[1].ConfigPropertyName(), procs[1].isEnabled()),
		zap.Bool(procs[2].ConfigPropertyName(), procs[2].isEnabled()),
		zap.Bool(procs[3].ConfigPropertyName(), procs[3].isEnabled()),
		zap.Bool(procs[4].ConfigPropertyName(), procs[4].isEnabled()),
		zap.Bool(procs[5].ConfigPropertyName(), procs[5].isEnabled()),
	)
	return nil
}

func (processor *sumologicProcessor) shutdown(_ context.Context) error {
	processor.logger.Info("Sumo Logic Processor has shut down.")
	return nil
}

func (processor *sumologicProcessor) processLogs(_ context.Context, logs plog.Logs) (plog.Logs, error) {
	for i := 0; i < len(processor.subprocessors); i++ {
		subprocessor := processor.subprocessors[i]
		if err := subprocessor.processLogs(logs); err != nil {
			return logs, fmt.Errorf("failed to process logs for property %s: %w", subprocessor.ConfigPropertyName(), err)
		}
	}

	return logs, nil
}

func (processor *sumologicProcessor) processMetrics(_ context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < len(processor.subprocessors); i++ {
		subprocessor := processor.subprocessors[i]
		if err := subprocessor.processMetrics(metrics); err != nil {
			return metrics, fmt.Errorf("failed to process metrics for property %s: %w", subprocessor.ConfigPropertyName(), err)
		}
	}

	return metrics, nil
}

func (processor *sumologicProcessor) processTraces(_ context.Context, traces ptrace.Traces) (ptrace.Traces, error) {
	for i := 0; i < len(processor.subprocessors); i++ {
		subprocessor := processor.subprocessors[i]
		if err := subprocessor.processTraces(traces); err != nil {
			return traces, fmt.Errorf("failed to process traces for property %s: %w", subprocessor.ConfigPropertyName(), err)
		}
	}

	return traces, nil
}
