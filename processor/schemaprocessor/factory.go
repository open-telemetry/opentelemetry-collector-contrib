// Copyright The OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	typeStr = "schema"
	// The stability level of the processor.
	stability = component.StabilityLevelDevelopment
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// factory will store any of the precompiled schemas in future
type factory struct{}

// newDefaultConfiguration returns the configuration for schema transformer processor
// with the default values being used throughout it
func newDefaultConfiguration() component.Config {
	return &Config{
		HTTPClientSettings: confighttp.NewDefaultHTTPClientSettings(),
	}
}

func NewFactory() processor.Factory {
	f := &factory{}
	return processor.NewFactory(
		typeStr,
		newDefaultConfiguration,
		processor.WithLogs(f.createLogsProcessor, stability),
		processor.WithMetrics(f.createMetricsProcessor, stability),
		processor.WithTraces(f.createTracesProcessor, stability),
	)
}

func (f factory) createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	next consumer.Logs,
) (processor.Logs, error) {
	transformer, err := newTransformer(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		next,
		transformer.processLogs,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(transformer.start),
	)
}

func (f factory) createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	next consumer.Metrics,
) (processor.Metrics, error) {
	transformer, err := newTransformer(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		next,
		transformer.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(transformer.start),
	)
}

func (f factory) createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	transformer, err := newTransformer(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		next,
		transformer.processTraces,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(transformer.start),
	)
}
