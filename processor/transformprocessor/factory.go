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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/profiles"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

type transformProcessorFactory struct {
	dataPointFunctions                  map[string]ottl.Factory[ottldatapoint.TransformContext]
	logFunctions                        map[string]ottl.Factory[ottllog.TransformContext]
	metricFunctions                     map[string]ottl.Factory[ottlmetric.TransformContext]
	spanEventFunctions                  map[string]ottl.Factory[ottlspanevent.TransformContext]
	spanFunctions                       map[string]ottl.Factory[ottlspan.TransformContext]
	profileFunctions                    map[string]ottl.Factory[ottlprofile.TransformContext]
	defaultDataPointFunctionsOverridden bool
	defaultLogFunctionsOverridden       bool
	defaultMetricFunctionsOverridden    bool
	defaultSpanEventFunctionsOverridden bool
	defaultSpanFunctionsOverridden      bool
	defaultProfileFunctionsOverridden   bool
}

// FactoryOption applies changes to transformProcessorFactory.
type FactoryOption func(factory *transformProcessorFactory)

// WithDataPointFunctions will override the default OTTL datapoint context functions with the provided dataPointFunctions in resulting processor.
// Subsequent uses of WithDataPointFunctions will merge the provided dataPointFunctions with the previously registered functions.
func WithDataPointFunctions(dataPointFunctions []ottl.Factory[ottldatapoint.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		if !factory.defaultDataPointFunctionsOverridden {
			factory.dataPointFunctions = map[string]ottl.Factory[ottldatapoint.TransformContext]{}
			factory.defaultDataPointFunctionsOverridden = true
		}
		factory.dataPointFunctions = mergeFunctionsToMap(factory.dataPointFunctions, dataPointFunctions)
	}
}

// WithLogFunctions will override the default OTTL log context functions with the provided logFunctions in the resulting processor.
// Subsequent uses of WithLogFunctions will merge the provided logFunctions with the previously registered functions.
func WithLogFunctions(logFunctions []ottl.Factory[ottllog.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		if !factory.defaultLogFunctionsOverridden {
			factory.logFunctions = map[string]ottl.Factory[ottllog.TransformContext]{}
			factory.defaultLogFunctionsOverridden = true
		}
		factory.logFunctions = mergeFunctionsToMap(factory.logFunctions, logFunctions)
	}
}

// WithMetricFunctions will override the default OTTL metric context functions with the provided metricFunctions in the resulting processor.
// Subsequent uses of WithMetricFunctions will merge the provided metricFunctions with the previously registered functions.
func WithMetricFunctions(metricFunctions []ottl.Factory[ottlmetric.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		if !factory.defaultMetricFunctionsOverridden {
			factory.metricFunctions = map[string]ottl.Factory[ottlmetric.TransformContext]{}
			factory.defaultMetricFunctionsOverridden = true
		}
		factory.metricFunctions = mergeFunctionsToMap(factory.metricFunctions, metricFunctions)
	}
}

// WithSpanEventFunctions will override the default OTTL spanevent context functions with the provided spanEventFunctions in the resulting processor.
// Subsequent uses of WithSpanEventFunctions will merge the provided spanEventFunctions with the previously registered functions.
func WithSpanEventFunctions(spanEventFunctions []ottl.Factory[ottlspanevent.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		if !factory.defaultSpanEventFunctionsOverridden {
			factory.spanEventFunctions = map[string]ottl.Factory[ottlspanevent.TransformContext]{}
			factory.defaultSpanEventFunctionsOverridden = true
		}
		factory.spanEventFunctions = mergeFunctionsToMap(factory.spanEventFunctions, spanEventFunctions)
	}
}

// WithSpanFunctions will override the default OTTL span context functions with the provided spanFunctions in the resulting processor.
// Subsequent uses of WithSpanFunctions will merge the provided spanFunctions with the previously registered functions.
func WithSpanFunctions(spanFunctions []ottl.Factory[ottlspan.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		if !factory.defaultSpanFunctionsOverridden {
			factory.spanFunctions = map[string]ottl.Factory[ottlspan.TransformContext]{}
			factory.defaultSpanFunctionsOverridden = true
		}
		factory.spanFunctions = mergeFunctionsToMap(factory.spanFunctions, spanFunctions)
	}
}

// WithProfileFunctions will override the default OTTL profile context functions with the provided profileFunctions in the resulting processor.
// Subsequent uses of WithProfileFunctions will merge the provided profileFunctions with the previously registered functions.
func WithProfileFunctions(profileFunctions []ottl.Factory[ottlprofile.TransformContext]) FactoryOption {
	return func(factory *transformProcessorFactory) {
		if !factory.defaultProfileFunctionsOverridden {
			factory.profileFunctions = map[string]ottl.Factory[ottlprofile.TransformContext]{}
			factory.defaultProfileFunctionsOverridden = true
		}
		factory.profileFunctions = mergeFunctionsToMap(factory.profileFunctions, profileFunctions)
	}
}

func NewFactory() processor.Factory {
	return NewFactoryWithOptions()
}

// NewFactoryWithOptions can receive FactoryOption like With*Functions to register non-default OTTL functions in the resulting processor.
func NewFactoryWithOptions(options ...FactoryOption) processor.Factory {
	f := &transformProcessorFactory{
		dataPointFunctions: defaultDataPointFunctionsMap(),
		logFunctions:       defaultLogFunctionsMap(),
		metricFunctions:    defaultMetricFunctionsMap(),
		spanEventFunctions: defaultSpanEventFunctionsMap(),
		spanFunctions:      defaultSpanFunctionsMap(),
		profileFunctions:   defaultProfileFunctionsMap(),
	}
	for _, o := range options {
		o(f)
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
		ErrorMode:          ottl.PropagateError,
		TraceStatements:    []common.ContextStatements{},
		MetricStatements:   []common.ContextStatements{},
		LogStatements:      []common.ContextStatements{},
		ProfileStatements:  []common.ContextStatements{},
		dataPointFunctions: f.dataPointFunctions,
		logFunctions:       f.logFunctions,
		metricFunctions:    f.metricFunctions,
		spanEventFunctions: f.spanEventFunctions,
		spanFunctions:      f.spanFunctions,
		profileFunctions:   f.profileFunctions,
	}
}

func (f *transformProcessorFactory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)
	if f.defaultLogFunctionsOverridden {
		set.Logger.Debug("non-default OTTL log functions have been registered in the \"transform\" processor", zap.Bool("log", f.defaultLogFunctionsOverridden))
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
	if f.defaultSpanEventFunctionsOverridden || f.defaultSpanFunctionsOverridden {
		set.Logger.Debug("non-default OTTL trace functions have been registered in the \"transform\" processor",
			zap.Bool("span", f.defaultSpanFunctionsOverridden),
			zap.Bool("spanevent", f.defaultSpanEventFunctionsOverridden),
		)
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
	if f.defaultDataPointFunctionsOverridden || f.defaultMetricFunctionsOverridden {
		set.Logger.Debug("non-default OTTL metric functions have been registered in the \"transform\" processor",
			zap.Bool("datapoint", f.defaultDataPointFunctionsOverridden),
			zap.Bool("metric", f.defaultMetricFunctionsOverridden),
		)
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

func (f *transformProcessorFactory) createProfilesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer xconsumer.Profiles,
) (xprocessor.Profiles, error) {
	oCfg := cfg.(*Config)
	oCfg.logger = set.Logger

	if f.defaultProfileFunctionsOverridden {
		set.Logger.Debug("non-default OTTL profile functions have been registered in the \"transform\" processor", zap.Bool("profile", f.defaultProfileFunctionsOverridden))
	}
	proc, err := profiles.NewProcessor(oCfg.ProfileStatements, oCfg.ErrorMode, set.TelemetrySettings, f.profileFunctions)
	if err != nil {
		return nil, fmt.Errorf("invalid config for \"transform\" processor %w", err)
	}
	return xprocessorhelper.NewProfiles(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessProfiles,
		xprocessorhelper.WithCapabilities(processorCapabilities))
}
