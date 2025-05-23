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
	dataPointFunctions                 map[string]ottl.Factory[ottldatapoint.TransformContext]
	logFunctions                       map[string]ottl.Factory[ottllog.TransformContext]
	metricFunctions                    map[string]ottl.Factory[ottlmetric.TransformContext]
	spanEventFunctions                 map[string]ottl.Factory[ottlspanevent.TransformContext]
	spanFunctions                      map[string]ottl.Factory[ottlspan.TransformContext]
	defaultDataPointFunctionsOverriden bool
	defaultLogFunctionsOverriden       bool
	defaultMetricFunctionsOverriden    bool
	defaultSpanEventFunctionsOverriden bool
	defaultSpanFunctionsOverriden      bool
}

// FactoryOption applies changes to transformProcessorFactory.
type FactoryOption func(factory *transformProcessorFactory)

// WithDataPointFunctions set ottl datapoint functions in resulting processor.
func WithDataPointFunctions(dataPointFunctions ...ottl.Factory[ottldatapoint.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.dataPointFunctions = createFunctionsMap(dataPointFunctions)
		factory.defaultDataPointFunctionsOverriden = true
	}
}

// WithLogFunctions set ottl log functions in resulting processor.
func WithLogFunctions(logFunctions ...ottl.Factory[ottllog.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.logFunctions = createFunctionsMap(logFunctions)
		factory.defaultLogFunctionsOverriden = true
	}
}

// WithMetricFunctions set ottl metric functions in resulting processor.
func WithMetricFunctions(metricFunctions ...ottl.Factory[ottlmetric.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.metricFunctions = createFunctionsMap(metricFunctions)
		factory.defaultMetricFunctionsOverriden = true
	}
}

// WithSpanEventFunctions set ottl span event functions in resulting processor.
func WithSpanEventFunctions(spanEventFunctions ...ottl.Factory[ottlspanevent.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.spanEventFunctions = createFunctionsMap(spanEventFunctions)
		factory.defaultSpanEventFunctionsOverriden = true
	}
}

// WithSpanFunctions set ottl span functions in resulting processor.
func WithSpanFunctions(spanFunctions ...ottl.Factory[ottlspan.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		factory.spanFunctions = createFunctionsMap(spanFunctions)
		factory.defaultSpanFunctionsOverriden = true
	}
}

// NewFactory can receive FactoryOption like With*Functions to set ottl functions of the resulting processor.
func NewFactory(options ...FactoryOption) processor.Factory {
	f := &transformProcessorFactory{
		dataPointFunctions:                 defaultDataPointFunctionsMap(),
		logFunctions:                       defaultLogFunctionsMap(),
		metricFunctions:                    defaultMetricFunctionsMap(),
		spanEventFunctions:                 defaultSpanEventFunctionsMap(),
		spanFunctions:                      defaultSpanFunctionsMap(),
		defaultDataPointFunctionsOverriden: false,
		defaultLogFunctionsOverriden:       false,
		defaultMetricFunctionsOverriden:    false,
		defaultSpanEventFunctionsOverriden: false,
		defaultSpanFunctionsOverriden:      false,
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
		ErrorMode:          ottl.PropagateError,
		TraceStatements:    []common.ContextStatements{},
		MetricStatements:   []common.ContextStatements{},
		LogStatements:      []common.ContextStatements{},
		dataPointFunctions: f.dataPointFunctions,
		logFunctions:       f.logFunctions,
		metricFunctions:    f.metricFunctions,
		spanEventFunctions: f.spanEventFunctions,
		spanFunctions:      f.spanFunctions,
	}
}

func (f *transformProcessorFactory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)
	if f.defaultLogFunctionsOverriden {
		set.Logger.Sugar().Debug("non-default ottl log functions are set in \"transform\" processor")
	}
	proc, err := logs.NewProcessor(oCfg.LogStatements, oCfg.ErrorMode, oCfg.FlattenData, set.TelemetrySettings, f.logFunctions)
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
	if f.defaultSpanEventFunctionsOverriden || f.defaultSpanFunctionsOverriden {
		set.Logger.Sugar().Debug("non-default ottl trace functions are set in \"transform\" processor")
	}
	proc, err := traces.NewProcessor(oCfg.TraceStatements, oCfg.ErrorMode, set.TelemetrySettings, f.spanFunctions, f.spanEventFunctions)
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
	if f.defaultMetricFunctionsOverriden || f.defaultDataPointFunctionsOverriden {
		set.Logger.Sugar().Debug("non-default ottl metric functions are set in \"transform\" processor")
	}
	proc, err := metrics.NewProcessor(oCfg.MetricStatements, oCfg.ErrorMode, set.TelemetrySettings, f.metricFunctions, f.dataPointFunctions)
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
