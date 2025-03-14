// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// factory will store any of the precompiled schemas in future
type factory struct{}

// newDefaultConfiguration returns the configuration for schema processor
// with the default values being used throughout it
func newDefaultConfiguration() component.Config {
	return &Config{
		ClientConfig: confighttp.NewDefaultClientConfig(),
	}
}

func NewFactory() processor.Factory {
	f := &factory{}
	return processor.NewFactory(
		metadata.Type,
		newDefaultConfiguration,
		processor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
		processor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
	)
}

func (f factory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Logs,
) (processor.Logs, error) {
	schemaProcessor, err := newSchemaProcessor(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		next,
		schemaProcessor.processLogs,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(schemaProcessor.start),
	)
}

func (f factory) createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (processor.Metrics, error) {
	schemaProcessor, err := newSchemaProcessor(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		next,
		schemaProcessor.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(schemaProcessor.start),
	)
}

func (f factory) createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	schemaProcessor, err := newSchemaProcessor(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		next,
		schemaProcessor.processTraces,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(schemaProcessor.start),
	)
}
