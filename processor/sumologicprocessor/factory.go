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

package sumologicschemaprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr        = "sumologic_schema"
	stabilityLevel = component.StabilityLevelBeta
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stabilityLevel),
		processor.WithMetrics(createMetricsProcessor, stabilityLevel),
		processor.WithLogs(createLogsProcessor, stabilityLevel),
	)
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	processor, err := newSumologicSchemaProcessor(set, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		processor.processLogs,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(processor.start),
		processorhelper.WithShutdown(processor.shutdown))
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processor, err := newSumologicSchemaProcessor(set, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		processor.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(processor.start),
		processorhelper.WithShutdown(processor.shutdown))
}

func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	processor, err := newSumologicSchemaProcessor(set, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		processor.processTraces,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(processor.start),
		processorhelper.WithShutdown(processor.shutdown))
}
