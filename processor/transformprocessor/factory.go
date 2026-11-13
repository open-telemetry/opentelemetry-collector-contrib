// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper"
	"go.opentelemetry.io/collector/processor/xprocessor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlexemplar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/factoryoptions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/profiles"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

type transformProcessorFactory struct {
	factoryoptions.Options
}

// FactoryOption applies changes to transformProcessorFactory.
type FactoryOption func(options *factoryoptions.Options)

// WithDataPointFunctions will override the default OTTL datapoint context functions with the provided dataPointFunctions in resulting processor.
// Subsequent uses of WithDataPointFunctions will merge the provided dataPointFunctions with the previously registered functions.
func WithDataPointFunctions(dataPointFunctions []ottl.Factory[*ottldatapoint.TransformContext]) FactoryOption {
	return func(options *factoryoptions.Options) {
		if !options.DefaultDataPointFunctionsOverridden {
			options.DataPointFunctions = map[string]ottl.Factory[*ottldatapoint.TransformContext]{}
			options.DefaultDataPointFunctionsOverridden = true
		}
		options.DataPointFunctions = factoryoptions.MergeFunctionsToMap(options.DataPointFunctions, dataPointFunctions)
	}
}

// WithExemplarFunctions will override the default OTTL exemplar context functions with the provided exemplarFunctions in resulting processor.
// Subsequent uses of WithExemplarFunctions will merge the provided exemplarFunctions with the previously registered functions.
func WithExemplarFunctions(exemplarFunctions []ottl.Factory[*ottlexemplar.TransformContext]) FactoryOption {
	return func(options *factoryoptions.Options) {
		if !options.DefaultExemplarFunctionsOverridden {
			options.ExemplarFunctions = map[string]ottl.Factory[*ottlexemplar.TransformContext]{}
			options.DefaultExemplarFunctionsOverridden = true
		}
		options.ExemplarFunctions = factoryoptions.MergeFunctionsToMap(options.ExemplarFunctions, exemplarFunctions)
	}
}

// WithLogFunctions will override the default OTTL log context functions with the provided logFunctions in the resulting processor.
// Subsequent uses of WithLogFunctions will merge the provided logFunctions with the previously registered functions.
func WithLogFunctions(logFunctions []ottl.Factory[*ottllog.TransformContext]) FactoryOption {
	return func(options *factoryoptions.Options) {
		if !options.DefaultLogFunctionsOverridden {
			options.LogFunctions = map[string]ottl.Factory[*ottllog.TransformContext]{}
			options.DefaultLogFunctionsOverridden = true
		}
		options.LogFunctions = factoryoptions.MergeFunctionsToMap(options.LogFunctions, logFunctions)
	}
}

// WithMetricFunctions will override the default OTTL metric context functions with the provided metricFunctions in the resulting processor.
// Subsequent uses of WithMetricFunctions will merge the provided metricFunctions with the previously registered functions.
func WithMetricFunctions(metricFunctions []ottl.Factory[*ottlmetric.TransformContext]) FactoryOption {
	return func(options *factoryoptions.Options) {
		if !options.DefaultMetricFunctionsOverridden {
			options.MetricFunctions = map[string]ottl.Factory[*ottlmetric.TransformContext]{}
			options.DefaultMetricFunctionsOverridden = true
		}
		options.MetricFunctions = factoryoptions.MergeFunctionsToMap(options.MetricFunctions, metricFunctions)
	}
}

// WithSpanEventFunctions will override the default OTTL spanevent context functions with the provided spanEventFunctions in the resulting processor.
// Subsequent uses of WithSpanEventFunctions will merge the provided spanEventFunctions with the previously registered functions.
func WithSpanEventFunctions(spanEventFunctions []ottl.Factory[*ottlspanevent.TransformContext]) FactoryOption {
	return func(options *factoryoptions.Options) {
		if !options.DefaultSpanEventFunctionsOverridden {
			options.SpanEventFunctions = map[string]ottl.Factory[*ottlspanevent.TransformContext]{}
			options.DefaultSpanEventFunctionsOverridden = true
		}
		options.SpanEventFunctions = factoryoptions.MergeFunctionsToMap(options.SpanEventFunctions, spanEventFunctions)
	}
}

// WithSpanFunctions will override the default OTTL span context functions with the provided spanFunctions in the resulting processor.
// Subsequent uses of WithSpanFunctions will merge the provided spanFunctions with the previously registered functions.
func WithSpanFunctions(spanFunctions []ottl.Factory[*ottlspan.TransformContext]) FactoryOption {
	return func(options *factoryoptions.Options) {
		if !options.DefaultSpanFunctionsOverridden {
			options.SpanFunctions = map[string]ottl.Factory[*ottlspan.TransformContext]{}
			options.DefaultSpanFunctionsOverridden = true
		}
		options.SpanFunctions = factoryoptions.MergeFunctionsToMap(options.SpanFunctions, spanFunctions)
	}
}

func NewFactory() processor.Factory {
	return NewFactoryWithOptions()
}

// NewFactoryWithOptions can receive FactoryOption like With*Functions to register non-default OTTL functions in the resulting processor.
func NewFactoryWithOptions(options ...FactoryOption) processor.Factory {
	f := &transformProcessorFactory{
		Options: factoryoptions.Options{
			DataPointFunctions: defaultDataPointFunctionsMap(),
			ExemplarFunctions:  defaultExemplarFunctionsMap(),
			LogFunctions:       defaultLogFunctionsMap(),
			MetricFunctions:    defaultMetricFunctionsMap(),
			SpanEventFunctions: defaultSpanEventFunctionsMap(),
			SpanFunctions:      defaultSpanFunctionsMap(),
			ProfileFunctions:   defaultProfileFunctionsMap(),
		},
	}
	for _, o := range options {
		o(&f.Options)
	}

	return xprocessor.NewFactory(
		metadata.Type,
		f.createDefaultConfig,
		xprocessor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
		xprocessor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
		xprocessor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
		xprocessor.WithProfiles(f.createProfilesProcessor, metadata.ProfilesStability),
	)
}

func (f *transformProcessorFactory) createDefaultConfig() component.Config {
	return &Config{
		ErrorMode:          ottl.IgnoreError,
		TraceStatements:    []common.ContextStatements{},
		MetricStatements:   []common.ContextStatements{},
		LogStatements:      []common.ContextStatements{},
		ProfileStatements:  []common.ContextStatements{},
		dataPointFunctions: f.DataPointFunctions,
		exemplarFunctions:  f.ExemplarFunctions,
		logFunctions:       f.LogFunctions,
		metricFunctions:    f.MetricFunctions,
		spanEventFunctions: f.SpanEventFunctions,
		spanFunctions:      f.SpanFunctions,
		profileFunctions:   f.ProfileFunctions,
	}
}

func (f *transformProcessorFactory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)
	if f.DefaultLogFunctionsOverridden {
		set.Logger.Debug("non-default OTTL log functions have been registered in the \"transform\" processor", zap.Bool("log", f.DefaultLogFunctionsOverridden))
	}
	proc, err := logs.NewProcessor(oCfg.LogStatements, oCfg.ErrorMode, oCfg.Flatten.Log, set.TelemetrySettings, f.LogFunctions)
	if err != nil {
		return nil, fmt.Errorf("invalid config for \"transform\" processor %w", err)
	}
	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessLogs,
		processorhelper.WithCapabilities(processorCapabilities),
	)
}

func (f *transformProcessorFactory) createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)
	if f.DefaultSpanEventFunctionsOverridden || f.DefaultSpanFunctionsOverridden {
		set.Logger.Debug("non-default OTTL trace functions have been registered in the \"transform\" processor",
			zap.Bool("span", f.DefaultSpanFunctionsOverridden),
			zap.Bool("spanevent", f.DefaultSpanEventFunctionsOverridden),
		)
	}
	proc, err := traces.NewProcessor(oCfg.TraceStatements, oCfg.ErrorMode, set.TelemetrySettings, f.SpanFunctions, f.SpanEventFunctions)
	if err != nil {
		return nil, fmt.Errorf("invalid config for \"transform\" processor %w", err)
	}
	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessTraces,
		processorhelper.WithCapabilities(processorCapabilities),
	)
}

func (f *transformProcessorFactory) createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	oCfg := cfg.(*Config)
	oCfg.logger = set.Logger
	if f.DefaultDataPointFunctionsOverridden || f.DefaultExemplarFunctionsOverridden || f.DefaultMetricFunctionsOverridden {
		set.Logger.Debug("non-default OTTL metric functions have been registered in the \"transform\" processor",
			zap.Bool("datapoint", f.DefaultDataPointFunctionsOverridden),
			zap.Bool("exemplar", f.DefaultExemplarFunctionsOverridden),
			zap.Bool("metric", f.DefaultMetricFunctionsOverridden),
		)
	}
	proc, err := metrics.NewProcessor(oCfg.MetricStatements, oCfg.ErrorMode, set.TelemetrySettings, f.MetricFunctions, f.DataPointFunctions, f.ExemplarFunctions)
	if err != nil {
		return nil, fmt.Errorf("invalid config for \"transform\" processor %w", err)
	}
	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
	)
}

func (f *transformProcessorFactory) createProfilesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer xconsumer.Profiles,
) (xprocessor.Profiles, error) {
	if !metadata.ProcessorTransformEnableProfilesFeatureGate.IsEnabled() {
		return nil, errProfilesGateDisabled
	}
	oCfg := cfg.(*Config)
	oCfg.logger = set.Logger

	if f.DefaultProfileFunctionsOverridden {
		set.Logger.Debug("non-default OTTL profile functions have been registered in the \"transform\" processor", zap.Bool("profile", f.DefaultProfileFunctionsOverridden))
	}
	proc, err := profiles.NewProcessor(oCfg.ProfileStatements, oCfg.ErrorMode, set.TelemetrySettings, f.ProfileFunctions)
	if err != nil {
		return nil, fmt.Errorf("invalid config for \"transform\" processor %w", err)
	}
	return xprocessorhelper.NewProfiles(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessProfiles,
		xprocessorhelper.WithCapabilities(processorCapabilities),
	)
}
