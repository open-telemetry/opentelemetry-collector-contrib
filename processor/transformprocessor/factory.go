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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

type transformProcessorFactory struct {
	AdditionalLogFunctions       []ottl.Factory[ottllog.TransformContext]
	AdditionalSpanFunctions      []ottl.Factory[ottlspan.TransformContext]
	AdditionalSpanEventFunctions []ottl.Factory[ottlspanevent.TransformContext]
	AdditionalMetricFunctions    []ottl.Factory[ottlmetric.TransformContext]
	AdditionalDataPointFunctions []ottl.Factory[ottldatapoint.TransformContext]
}

// FactoryOption applies changes to transformProcessorFactory.
type FactoryOption func(factory *transformProcessorFactory)

// WithAdditionalMetricFunctions adds ottl metric functions to resulting processor.
func WithAdditionalMetricFunctions(metricFunctions []ottl.Factory[ottlmetric.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.AdditionalMetricFunctions = metricFunctions
	}
}

// WithAdditionalLogFunctions adds ottl log functions to resulting processor.
func WithAdditionalLogFunctions(logFunctions []ottl.Factory[ottllog.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.AdditionalLogFunctions = logFunctions
	}
}

// WithAdditionalSpanFunctions adds ottl span functions to resulting processor.
func WithAdditionalSpanFunctions(spanFunctions []ottl.Factory[ottlspan.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.AdditionalSpanFunctions = spanFunctions
	}
}

// WithAdditionalSpanEventFunctions adds ottl span event functions to resulting processor.
func WithAdditionalSpanEventFunctions(spanEventFunctions []ottl.Factory[ottlspanevent.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.AdditionalSpanEventFunctions = spanEventFunctions
	}
}

// WithAdditionalDataPointFunctions adds ottl data point functions to resulting processor.
func WithAdditionalDataPointFunctions(dataPointFunctions []ottl.Factory[ottldatapoint.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.AdditionalDataPointFunctions = dataPointFunctions
	}
}

// NewFactory can receive FactoryOptions of the WithAdditional*Functions to add ottl functions to the resulting processor.
func NewFactory(options ...FactoryOption) processor.Factory {
	f := &transformProcessorFactory{
		AdditionalSpanFunctions:      []ottl.Factory[ottlspan.TransformContext]{},
		AdditionalSpanEventFunctions: []ottl.Factory[ottlspanevent.TransformContext]{},
		AdditionalLogFunctions:       []ottl.Factory[ottllog.TransformContext]{},
		AdditionalMetricFunctions:    []ottl.Factory[ottlmetric.TransformContext]{},
		AdditionalDataPointFunctions: []ottl.Factory[ottldatapoint.TransformContext]{},
	}
	for _, o := range options {
		o(f)
	}

	return processor.NewFactory(
		metadata.Type,
		f.createDefaultConfig,
		processor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
		processor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
		processor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
	)
}

func (f *transformProcessorFactory) createDefaultConfig() component.Config {
	return &Config{
		ErrorMode:                    ottl.PropagateError,
		TraceStatements:              []common.ContextStatements{},
		MetricStatements:             []common.ContextStatements{},
		LogStatements:                []common.ContextStatements{},
		additionalLogFunctions:       f.AdditionalLogFunctions,
		additionalSpanFunctions:      f.AdditionalSpanFunctions,
		additionalMetricFunctions:    f.AdditionalMetricFunctions,
		additionalSpanEventFunctions: f.AdditionalSpanEventFunctions,
		additionalDataPointFunctions: f.AdditionalDataPointFunctions,
	}
}

func (f *transformProcessorFactory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)
	logFunctions := mergeAdditionalFunctions(DefaultLogFunctions(), f.AdditionalLogFunctions, set.Logger)
	proc, err := logs.NewProcessor(oCfg.LogStatements, oCfg.ErrorMode, oCfg.FlattenData, set.TelemetrySettings, logFunctions)
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
	spanFunctions := mergeAdditionalFunctions(DefaultSpanFunctions(), f.AdditionalSpanFunctions, set.Logger)
	spanEventFunctions := mergeAdditionalFunctions(DefaultSpanEventFunctions(), f.AdditionalSpanEventFunctions, set.Logger)
	proc, err := traces.NewProcessor(oCfg.TraceStatements, oCfg.ErrorMode, set.TelemetrySettings, spanFunctions, spanEventFunctions)
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
	metricFunctions := mergeAdditionalFunctions(DefaultMetricFunctions(), f.AdditionalMetricFunctions, set.Logger)
	dataPointFunctions := mergeAdditionalFunctions(DefaultDataPointFunctions(), f.AdditionalDataPointFunctions, set.Logger)
	proc, err := metrics.NewProcessor(oCfg.MetricStatements, oCfg.ErrorMode, set.TelemetrySettings, metricFunctions, dataPointFunctions)
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
