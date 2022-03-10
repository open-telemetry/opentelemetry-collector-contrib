// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const typeStr = "schema"

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// Factory will store any of the precompiled schemas in future
type factory struct{}

// NewDefaultConfiguration returns the configuration for schema transformer processor
// with the default values being used throughout it
func NewDefaultConfiguration() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
	}
}

func NewFactory() component.ProcessorFactory {
	f := &factory{}
	return component.NewProcessorFactory(
		typeStr,
		NewDefaultConfiguration,
		component.WithLogsProcessor(f.createLogsProcessor),
		component.WithMetricsProcessor(f.createMetricsProcessor),
		component.WithTracesProcessor(f.createTracesProcessor),
	)
}

func (f factory) createLogsProcessor(
	ctx context.Context,
	set component.ProcessorCreateSettings,
	cfg config.Processor,
	next consumer.Logs,
) (component.LogsProcessor, error) {
	transformer, err := newTransformer(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogsProcessor(
		cfg,
		next,
		transformer.processLogs,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(transformer.start),
	)
}

func (f factory) createMetricsProcessor(
	ctx context.Context,
	set component.ProcessorCreateSettings,
	cfg config.Processor,
	next consumer.Metrics,
) (component.MetricsProcessor, error) {
	transformer, err := newTransformer(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewMetricsProcessor(
		cfg,
		next,
		transformer.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(transformer.start),
	)
}

func (f factory) createTracesProcessor(
	ctx context.Context,
	set component.ProcessorCreateSettings,
	cfg config.Processor,
	next consumer.Traces,
) (component.TracesProcessor, error) {
	transformer, err := newTransformer(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTracesProcessor(
		cfg,
		next,
		transformer.processTraces,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(transformer.start),
	)
}
