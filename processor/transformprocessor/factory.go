// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

type transformProcessorFactory struct {
	additionalLogFunctions    []ottl.Factory[ottllog.TransformContext]
	additionalSpanFunctions   []ottl.Factory[ottlspan.TransformContext]
	additionalMetricFunctions []ottl.Factory[ottlmetric.TransformContext]
}

// FactoryOption applies changes to transformProcessorFactory.
type FactoryOption func(factory *transformProcessorFactory)

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
	return func(factory *transformProcessorFactory) {
		factory.additionalMetricFunctions = metricFunctions
	}
}

// WithAdditionalLogFunctions adds ottl log functions to resulting processor
func WithAdditionalLogFunctions(logFunctions []ottl.Factory[ottllog.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.additionalLogFunctions = logFunctions
	}
}

// WithAdditionalSpanFunctions adds ottl span functions to resulting processor
func WithAdditionalSpanFunctions(spanFunctions []ottl.Factory[ottlspan.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.additionalSpanFunctions = spanFunctions
	}
}

func NewFactory(options ...FactoryOption) processor.Factory {
	f := &transformProcessorFactory{
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
		processor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
		processor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
		processor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ErrorMode:        ottl.PropagateError,
		TraceStatements:  []common.ContextStatements{},
		MetricStatements: []common.ContextStatements{},
		LogStatements:    []common.ContextStatements{},
	}
}

func (f *transformProcessorFactory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)

	proc, err := logs.NewProcessor(oCfg.LogStatements, oCfg.ErrorMode, oCfg.FlattenData, set.TelemetrySettings, f.additionalLogFunctions)
	if err != nil {
		return nil, fmt.Errorf("invalid config for \"transform\" processor %w", err)
	}
	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}

func (f *transformProcessorFactory) createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)

	proc, err := traces.NewProcessor(oCfg.TraceStatements, oCfg.ErrorMode, set.TelemetrySettings, f.additionalSpanFunctions)
	if err != nil {
		return nil, fmt.Errorf("invalid config for \"transform\" processor %w", err)
	}
	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessTraces,
		processorhelper.WithCapabilities(processorCapabilities))
}

func (f *transformProcessorFactory) createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	oCfg := cfg.(*Config)
	oCfg.logger = set.Logger

	proc, err := metrics.NewProcessor(oCfg.MetricStatements, oCfg.ErrorMode, set.TelemetrySettings, f.additionalMetricFunctions)
	if err != nil {
		return nil, fmt.Errorf("invalid config for \"transform\" processor %w", err)
	}
	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessMetrics,
		processorhelper.WithCapabilities(processorCapabilities))
}
