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
		metadata.Type,
		newDefaultConfiguration,
		processor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
		processor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
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
