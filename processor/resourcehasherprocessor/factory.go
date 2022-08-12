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

package resourcehasherprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcehasherprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"

	lru "github.com/hashicorp/golang-lru"
)

const (
	// The value of "type" key in configuration.
	typeStr = "resourcehasher"
)

var consumerCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory creates a new factory for ResourceDetection processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTracesProcessor),
		component.WithMetricsProcessor(createMetricsProcessor),
		component.WithLogsProcessor(createLogsProcessor))
}

func createDefaultConfig() config.Processor {
	return &Config{
		MaximumCacheSize:     10000,
		MaximumCacheEntryAge: time.Duration(30 * float64(time.Second)),
	}
}

func createTracesProcessor(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	rdp, err := createResourceHasherProcessor(params, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTracesProcessor(
		cfg,
		nextConsumer,
		rdp.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func createMetricsProcessor(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	rdp, err := createResourceHasherProcessor(params, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetricsProcessor(
		cfg,
		nextConsumer,
		rdp.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func createLogsProcessor(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Logs,
) (component.LogsProcessor, error) {
	rdp, err := createResourceHasherProcessor(params, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogsProcessor(
		cfg,
		nextConsumer,
		rdp.processLogs,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func createResourceHasherProcessor(
	params component.ProcessorCreateSettings,
	cfg config.Processor,
) (*resourceHasherProcessor, error) {
	c := cfg.(*Config)

	cache, err := lru.New(c.MaximumCacheSize)

	if err != nil {
		return nil, err
	}

	return &resourceHasherProcessor{
		logger:    params.Logger,
		hashCache: *cache,
		config:    *c,
	}, nil
}
