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

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	typeStr = "transform"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func NewFactory() component.ProcessorFactory {
	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(createTracesProcessor),
	)
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		Traces: TracesConfig{
			Queries: []string{},
		},
	}
}

func createTracesProcessor(
	_ context.Context,
	_ component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	return processorhelper.NewTracesProcessor(
		cfg,
		nextConsumer,
		// TODO(anuraaga): Replace with business logic.
		func(ctx context.Context, p pdata.Traces) (pdata.Traces, error) {
			return p, nil
		},
		processorhelper.WithCapabilities(processorCapabilities))
}
