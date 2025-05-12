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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

type transformProcessorFactory struct {
	ottlprocessor.OttlProcessorFactory
}

// FactoryOption applies changes to transformProcessorFactory.
type FactoryOption func(factory *transformProcessorFactory)

func NewFactory(options ...FactoryOption) processor.Factory {
	f := &transformProcessorFactory{
		OttlProcessorFactory: ottlprocessor.OttlProcessorFactory{
			AdditionalSpanFunctions:      []ottl.Factory[ottlspan.TransformContext]{},
			AdditionalSpanEventFunctions: []ottl.Factory[ottlspanevent.TransformContext]{},
			AdditionalLogFunctions:       []ottl.Factory[ottllog.TransformContext]{},
			AdditionalMetricFunctions:    []ottl.Factory[ottlmetric.TransformContext]{},
			AdditionalDataPointFunctions: []ottl.Factory[ottldatapoint.TransformContext]{},
		},
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
		additionalLogFunctions:       f.OttlProcessorFactory.AdditionalLogFunctions,
		additionalSpanFunctions:      f.OttlProcessorFactory.AdditionalSpanFunctions,
		additionalMetricFunctions:    f.OttlProcessorFactory.AdditionalMetricFunctions,
		additionalSpanEventFunctions: f.OttlProcessorFactory.AdditionalSpanEventFunctions,
		additionalDataPointFunctions: f.OttlProcessorFactory.AdditionalDataPointFunctions,
	}
}

func (f *transformProcessorFactory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)

	proc, err := logs.NewProcessor(oCfg.LogStatements, oCfg.ErrorMode, oCfg.FlattenData, set.TelemetrySettings, f.AdditionalLogFunctions)
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

	proc, err := traces.NewProcessor(oCfg.TraceStatements, oCfg.ErrorMode, set.TelemetrySettings, f.AdditionalSpanFunctions, f.AdditionalSpanEventFunctions)
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

	proc, err := metrics.NewProcessor(oCfg.MetricStatements, oCfg.ErrorMode, set.TelemetrySettings, f.AdditionalMetricFunctions, f.AdditionalDataPointFunctions)
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
