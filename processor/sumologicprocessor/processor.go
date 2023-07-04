// Copyright 2022 Sumo Logic, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sumologicprocessor

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

func newsumologicProcessor(set processor.CreateSettings, config *Config) (*sumologicProcessor, error) {
	cloudNamespaceProcessor, err := newCloudNamespaceProcessor(config.AddCloudNamespace)
	if err != nil {
		return nil, err
	}

	translateAttributesProcessor, err := newTranslateAttributesProcessor(config.TranslateAttributes)
	if err != nil {
		return nil, err
	}

	translateTelegrafMetricsProcessor, err := newTranslateTelegrafMetricsProcessor(config.TranslateTelegrafAttributes)
	if err != nil {
		return nil, err
	}

	nestingProcessor, err := newNestingProcessor(config.NestAttributes)
	if err != nil {
		return nil, err
	}

	aggregateAttributesProcessor, err := newAggregateAttributesProcessor(config.AggregateAttributes)
	if err != nil {
		return nil, err
	}

	logFieldsConversionProcessor, err := newLogFieldConversionProcessor(config.LogFieldsAttributes)
	if err != nil {
		return nil, err
	}

	translateDockerMetricsProcessor, err := newTranslateDockerMetricsProcessor(config.TranslateDockerMetrics)
	if err != nil {
		return nil, err
	}

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

	return processor, nil
}

func (processor *sumologicProcessor) start(_ context.Context, host component.Host) error {
	procs := processor.subprocessors
	processor.logger.Info(
		"Processor sumologic has started.",
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
	processor.logger.Info("Processor sumologic has shut down.")
	return nil
}

func (processor *sumologicProcessor) processLogs(_ context.Context, logs plog.Logs) (plog.Logs, error) {
	for i := 0; i < len(processor.subprocessors); i++ {
		subprocessor := processor.subprocessors[i]
		if err := subprocessor.processLogs(logs); err != nil {
			return logs, fmt.Errorf("failed to process logs for property %s: %v", subprocessor.ConfigPropertyName(), err)
		}
	}

	return logs, nil
}

func (processor *sumologicProcessor) processMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < len(processor.subprocessors); i++ {
		subprocessor := processor.subprocessors[i]
		if err := subprocessor.processMetrics(metrics); err != nil {
			return metrics, fmt.Errorf("failed to process metrics for property %s: %v", subprocessor.ConfigPropertyName(), err)
		}
	}

	return metrics, nil
}

func (processor *sumologicProcessor) processTraces(ctx context.Context, traces ptrace.Traces) (ptrace.Traces, error) {
	for i := 0; i < len(processor.subprocessors); i++ {
		subprocessor := processor.subprocessors[i]
		if err := subprocessor.processTraces(traces); err != nil {
			return traces, fmt.Errorf("failed to process traces for property %s: %v", subprocessor.ConfigPropertyName(), err)
		}
	}

	return traces, nil
}
