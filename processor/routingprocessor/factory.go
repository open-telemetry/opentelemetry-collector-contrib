// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr = "routing"
)

// NewFactory creates a factory for the routing processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTracesProcessor),
		component.WithMetricsProcessor(createMetricsProcessor),
		component.WithLogsProcessor(createLogsProcessor),
	)
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		AttributeSource:   defaultAttributeSource,
	}
}

func createTracesProcessor(_ context.Context, params component.ProcessorCreateSettings, cfg config.Processor, nextConsumer consumer.Traces) (component.TracesProcessor, error) {
	warnIfNotLastInPipeline(nextConsumer, params.Logger)
	return newProcessor(params.Logger, cfg), nil
}

func createMetricsProcessor(_ context.Context, params component.ProcessorCreateSettings, cfg config.Processor, nextConsumer consumer.Metrics) (component.MetricsProcessor, error) {
	warnIfNotLastInPipeline(nextConsumer, params.Logger)
	return newProcessor(params.Logger, cfg), nil
}

func createLogsProcessor(_ context.Context, params component.ProcessorCreateSettings, cfg config.Processor, nextConsumer consumer.Logs) (component.LogsProcessor, error) {
	warnIfNotLastInPipeline(nextConsumer, params.Logger)
	return newProcessor(params.Logger, cfg), nil
}

func warnIfNotLastInPipeline(nextConsumer interface{}, logger *zap.Logger) {
	_, ok := nextConsumer.(component.Processor)
	if ok {
		logger.Warn("another processor has been defined after the routing processor: it will NOT receive any data!")
	}
}
