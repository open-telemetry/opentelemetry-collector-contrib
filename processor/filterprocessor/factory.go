// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

type filterProcessorFactory struct {
	additionalLogFunctions    []ottl.Factory[ottllog.TransformContext]
	additionalSpanFunctions   []ottl.Factory[ottlspan.TransformContext]
	additionalMetricFunctions []ottl.Factory[ottlmetric.TransformContext]
}

// FactoryOption applies changes to filterProcessorFactory.
type FactoryOption func(factory *filterProcessorFactory)

// defaultAdditionalMetricFunctions returns an empty slice of ottl function factories.
func defaultAdditionalMetricFunctions() []ottl.Factory[ottlmetric.TransformContext] {
	return []ottl.Factory[ottlmetric.TransformContext]{}
}

// defaultAdditionalLogFunctions returns an empty slice of ottl function factories.
func defaultAdditionalLogFunctions() []ottl.Factory[ottllog.TransformContext] {
	return []ottl.Factory[ottllog.TransformContext]{}
}

// defaultAdditionalSpanFunctions returns an empty slice of ottl function factories.
func defaultAdditionalSpanFunctions() []ottl.Factory[ottlspan.TransformContext] {
	return []ottl.Factory[ottlspan.TransformContext]{}
}

// WithAdditionalMetricFunctions adds ottl metric functions to resulting processor
func WithAdditionalMetricFunctions(metricFunctions []ottl.Factory[ottlmetric.TransformContext]) FactoryOption {
	return func(factory *filterProcessorFactory) {
		factory.additionalMetricFunctions = metricFunctions
	}
}

// WithAdditionalLogFunctions adds ottl log functions to resulting processor
func WithAdditionalLogFunctions(logFunctions []ottl.Factory[ottllog.TransformContext]) FactoryOption {
	return func(factory *filterProcessorFactory) {
		factory.additionalLogFunctions = logFunctions
	}
}

// WithAdditionalSpanFunctions adds ottl span functions to resulting processor
func WithAdditionalSpanFunctions(spanFunctions []ottl.Factory[ottlspan.TransformContext]) FactoryOption {
	return func(factory *filterProcessorFactory) {
		factory.additionalSpanFunctions = spanFunctions
	}
}

// NewFactory returns a new factory for the Filter processor.
func NewFactory(options ...FactoryOption) processor.Factory {
	f := &filterProcessorFactory{
		additionalMetricFunctions: defaultAdditionalMetricFunctions(),
		additionalLogFunctions:    defaultAdditionalLogFunctions(),
		additionalSpanFunctions:   defaultAdditionalSpanFunctions(),
	}
	for _, o := range options {
		o(f)
	}

	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
		processor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
		processor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ErrorMode: ottl.PropagateError,
	}
}

func (f *filterProcessorFactory) createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	fp, err := newFilterMetricProcessor(set, cfg.(*Config), f.additionalMetricFunctions...)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		fp.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities))
}

func (f *filterProcessorFactory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	fp, err := newFilterLogsProcessor(set, cfg.(*Config), f.additionalLogFunctions...)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		fp.processLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}

func (f *filterProcessorFactory) createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	fp, err := newFilterSpansProcessor(set, cfg.(*Config), f.additionalSpanFunctions...)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		fp.processTraces,
		processorhelper.WithCapabilities(processorCapabilities))
}
