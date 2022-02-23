// Copyright  OpenTelemetry Authors
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

package redactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "redaction"
)

// NewFactory creates a factory for the redaction processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTracesProcessor),
	)
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
	}
}

// createTracesProcessor creates an instance of redaction for processing traces
func createTracesProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	next consumer.Traces,
) (component.TracesProcessor, error) {
	oCfg := cfg.(*Config)

	redaction, err := newRedaction(ctx, oCfg, params.Logger, next)
	if err != nil {
		// TODO: Placeholder for an error metric in the next PR
		return nil, fmt.Errorf("error creating a redaction processor: %w", err)
	}

	return processorhelper.NewTracesProcessor(
		cfg,
		next,
		redaction.processTraces,
		processorhelper.WithCapabilities(redaction.Capabilities()),
		processorhelper.WithStart(redaction.Start),
		processorhelper.WithShutdown(redaction.Shutdown))
}
