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

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"context"
	"io"
	"net/http"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
	schemas "go.opentelemetry.io/otel/schema/v1.0"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
)

const (
	// typeStr is the value of "type" key in configuration.
	typeStr = "schema"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

type schemaFetcherFunc func(context context.Context, schemaURL string) (*ast.Schema, error)

type factory struct {
	schemaFetcher schemaFetcherFunc

	schemasMux sync.RWMutex
	schemas    map[string]*ast.Schema
}

// NewFactory returns a new factory for the Attributes processor.
func NewFactory() component.ProcessorFactory {
	return newFactoryWithFetcher(downloadSchema)
}

func newFactory(schemaFetcher schemaFetcherFunc) *factory {
	return &factory{
		schemaFetcher: schemaFetcher,
		schemas:       map[string]*ast.Schema{},
	}
}

func newFactoryWithFetcher(schemaFetcher schemaFetcherFunc) component.ProcessorFactory {
	f := newFactory(schemaFetcher)
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(f.createTracesProcessor),
		component.WithMetricsProcessor(f.createMetricsProcessor),
		component.WithLogsProcessor(f.createLogProcessor))
}

// Note: This isn't a valid configuration because the processor would do no work.
func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
	}
}

func (f *factory) getSchema(context context.Context, schemaURL string) (*ast.Schema, error) {

	// Try the path with read lock first.
	f.schemasMux.RLock()
	if schema, ok := f.schemas[schemaURL]; ok {
		f.schemasMux.RUnlock()
		return schema, nil
	}
	f.schemasMux.RUnlock()

	// Don't have it. Do the full lock and download.
	f.schemasMux.Lock()
	defer f.schemasMux.Unlock()

	if schema, ok := f.schemas[schemaURL]; ok {
		// Already have the schema. Probably some other goroutine downloaded it before
		// we acquired the lock.
		return schema, nil
	}

	schema, err := f.schemaFetcher(context, schemaURL)
	if err != nil {
		return nil, err
	}

	f.schemas[schemaURL] = schema
	return schema, nil
}

func downloadSchema(context context.Context, schemaURL string) (*ast.Schema, error) {
	req, err := http.NewRequestWithContext(context, "GET", schemaURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}()

	schema, err := schemas.Parse(resp.Body)

	return schema, err
}

func (f *factory) createTracesProcessor(
	_ context.Context,
	_ component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	oCfg := cfg.(*Config)

	p := newSchemaProcessor(f, oCfg)

	return processorhelper.NewTracesProcessor(
		cfg,
		nextConsumer,
		p.processTraces,
		processorhelper.WithCapabilities(processorCapabilities))
}

func (f *factory) createMetricsProcessor(
	_ context.Context,
	_ component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	oCfg := cfg.(*Config)

	p := newSchemaProcessor(f, oCfg)

	return processorhelper.NewMetricsProcessor(
		cfg,
		nextConsumer,
		p.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities))
}

func (f *factory) createLogProcessor(
	_ context.Context,
	_ component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Logs,
) (component.LogsProcessor, error) {
	oCfg := cfg.(*Config)

	p := newSchemaProcessor(f, oCfg)

	return processorhelper.NewLogsProcessor(
		cfg,
		nextConsumer,
		p.processLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}
