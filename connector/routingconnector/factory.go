// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlotelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

func defaultErrorMode() ottl.ErrorMode {
	if metadata.ConnectorRoutingDefaultErrorModeIgnoreFeatureGate.IsEnabled() {
		return ottl.IgnoreError
	}
	return ottl.PropagateError
}

type routingConnectorFactory struct {
	otelColFunctions   map[string]ottl.Factory[*ottlotelcol.TransformContext]
	resourceFunctions  map[string]ottl.Factory[*ottlresource.TransformContext]
	spanFunctions      map[string]ottl.Factory[*ottlspan.TransformContext]
	metricFunctions    map[string]ottl.Factory[*ottlmetric.TransformContext]
	dataPointFunctions map[string]ottl.Factory[*ottldatapoint.TransformContext]
	logFunctions       map[string]ottl.Factory[*ottllog.TransformContext]

	defaultOtelColFunctionsOverridden   bool
	defaultResourceFunctionsOverridden  bool
	defaultSpanFunctionsOverridden      bool
	defaultMetricFunctionsOverridden    bool
	defaultDataPointFunctionsOverridden bool
	defaultLogFunctionsOverridden       bool
}

// FactoryOption applies changes to routingConnectorFactory.
type FactoryOption func(factory *routingConnectorFactory)

// withFunctions overrides target with fns on first use and merges on subsequent uses. If the
// resulting map has no function named "route", the default no-op implementation is added so
// routes using the "condition" field keep working; a caller supplying their own "route" function
// overrides it instead.
func withFunctions[K any](target *map[string]ottl.Factory[K], overridden *bool, fns []ottl.Factory[K]) {
	if !*overridden {
		*target = map[string]ottl.Factory[K]{}
		*overridden = true
	}
	*target = mergeFunctionsToMap(*target, fns)
	if _, ok := (*target)[routeFunctionName]; !ok {
		route := routeFunctionFactory[K]()
		(*target)[route.Name()] = route
	}
}

// WithOtelColFunctions will override the default OTTL otelcol context functions with the provided otelcolFunctions in the resulting connector.
// Subsequent uses of WithOtelColFunctions will merge the provided otelcolFunctions with the previously registered functions.
// The "route" function is always available unless the provided functions include one named "route", in which case it replaces the default no-op implementation.
func WithOtelColFunctions(otelcolFunctions []ottl.Factory[*ottlotelcol.TransformContext]) FactoryOption {
	return func(factory *routingConnectorFactory) {
		withFunctions(&factory.otelColFunctions, &factory.defaultOtelColFunctionsOverridden, otelcolFunctions)
	}
}

// WithResourceFunctions will override the default OTTL resource context functions with the provided resourceFunctions in the resulting connector.
// Subsequent uses of WithResourceFunctions will merge the provided resourceFunctions with the previously registered functions.
// The "route" function is always available unless the provided functions include one named "route", in which case it replaces the default no-op implementation.
func WithResourceFunctions(resourceFunctions []ottl.Factory[*ottlresource.TransformContext]) FactoryOption {
	return func(factory *routingConnectorFactory) {
		withFunctions(&factory.resourceFunctions, &factory.defaultResourceFunctionsOverridden, resourceFunctions)
	}
}

// WithSpanFunctions will override the default OTTL span context functions with the provided spanFunctions in the resulting connector.
// Subsequent uses of WithSpanFunctions will merge the provided spanFunctions with the previously registered functions.
// The "route" function is always available unless the provided functions include one named "route", in which case it replaces the default no-op implementation.
func WithSpanFunctions(spanFunctions []ottl.Factory[*ottlspan.TransformContext]) FactoryOption {
	return func(factory *routingConnectorFactory) {
		withFunctions(&factory.spanFunctions, &factory.defaultSpanFunctionsOverridden, spanFunctions)
	}
}

// WithMetricFunctions will override the default OTTL metric context functions with the provided metricFunctions in the resulting connector.
// Subsequent uses of WithMetricFunctions will merge the provided metricFunctions with the previously registered functions.
// The "route" function is always available unless the provided functions include one named "route", in which case it replaces the default no-op implementation.
func WithMetricFunctions(metricFunctions []ottl.Factory[*ottlmetric.TransformContext]) FactoryOption {
	return func(factory *routingConnectorFactory) {
		withFunctions(&factory.metricFunctions, &factory.defaultMetricFunctionsOverridden, metricFunctions)
	}
}

// WithDataPointFunctions will override the default OTTL datapoint context functions with the provided dataPointFunctions in the resulting connector.
// Subsequent uses of WithDataPointFunctions will merge the provided dataPointFunctions with the previously registered functions.
// The "route" function is always available unless the provided functions include one named "route", in which case it replaces the default no-op implementation.
func WithDataPointFunctions(dataPointFunctions []ottl.Factory[*ottldatapoint.TransformContext]) FactoryOption {
	return func(factory *routingConnectorFactory) {
		withFunctions(&factory.dataPointFunctions, &factory.defaultDataPointFunctionsOverridden, dataPointFunctions)
	}
}

// WithLogFunctions will override the default OTTL log context functions with the provided logFunctions in the resulting connector.
// Subsequent uses of WithLogFunctions will merge the provided logFunctions with the previously registered functions.
// The "route" function is always available unless the provided functions include one named "route", in which case it replaces the default no-op implementation.
func WithLogFunctions(logFunctions []ottl.Factory[*ottllog.TransformContext]) FactoryOption {
	return func(factory *routingConnectorFactory) {
		withFunctions(&factory.logFunctions, &factory.defaultLogFunctionsOverridden, logFunctions)
	}
}

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return NewFactoryWithOptions()
}

// NewFactoryWithOptions can receive FactoryOption like With*Functions to register non-default OTTL functions in the resulting connector.
func NewFactoryWithOptions(options ...FactoryOption) connector.Factory {
	f := &routingConnectorFactory{
		otelColFunctions:   defaultOtelColFunctionsMap(),
		resourceFunctions:  defaultResourceFunctionsMap(),
		spanFunctions:      defaultSpanFunctionsMap(),
		metricFunctions:    defaultMetricFunctionsMap(),
		dataPointFunctions: defaultDataPointFunctionsMap(),
		logFunctions:       defaultLogFunctionsMap(),
	}
	for _, o := range options {
		o(f)
	}

	return connector.NewFactory(
		metadata.Type,
		f.createDefaultConfig,
		connector.WithTracesToTraces(f.createTracesToTraces, metadata.TracesToTracesStability),
		connector.WithMetricsToMetrics(f.createMetricsToMetrics, metadata.MetricsToMetricsStability),
		connector.WithLogsToLogs(f.createLogsToLogs, metadata.LogsToLogsStability),
	)
}

// createDefaultConfig creates the default configuration.
func (f *routingConnectorFactory) createDefaultConfig() component.Config {
	return &Config{
		ErrorMode:          defaultErrorMode(),
		otelColFunctions:   f.otelColFunctions,
		resourceFunctions:  f.resourceFunctions,
		spanFunctions:      f.spanFunctions,
		metricFunctions:    f.metricFunctions,
		dataPointFunctions: f.dataPointFunctions,
		logFunctions:       f.logFunctions,
	}
}

// createTracesToTraces creates a traces to traces connector based on provided config.
func (f *routingConnectorFactory) createTracesToTraces(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	traces consumer.Traces,
) (connector.Traces, error) {
	if f.defaultOtelColFunctionsOverridden || f.defaultResourceFunctionsOverridden || f.defaultSpanFunctionsOverridden {
		set.Logger.Debug(`non-default OTTL functions have been registered in the "routing" connector`,
			zap.Bool("otelcol", f.defaultOtelColFunctionsOverridden),
			zap.Bool("resource", f.defaultResourceFunctionsOverridden),
			zap.Bool("span", f.defaultSpanFunctionsOverridden),
		)
	}
	return newTracesConnector(set, cfg, traces)
}

// createMetricsToMetrics creates a metrics to metrics connector based on provided config.
func (f *routingConnectorFactory) createMetricsToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	metrics consumer.Metrics,
) (connector.Metrics, error) {
	if f.defaultOtelColFunctionsOverridden || f.defaultResourceFunctionsOverridden || f.defaultMetricFunctionsOverridden || f.defaultDataPointFunctionsOverridden {
		set.Logger.Debug(`non-default OTTL functions have been registered in the "routing" connector`,
			zap.Bool("otelcol", f.defaultOtelColFunctionsOverridden),
			zap.Bool("resource", f.defaultResourceFunctionsOverridden),
			zap.Bool("metric", f.defaultMetricFunctionsOverridden),
			zap.Bool("datapoint", f.defaultDataPointFunctionsOverridden),
		)
	}
	return newMetricsConnector(set, cfg, metrics)
}

// createLogsToLogs creates a logs to logs connector based on provided config.
func (f *routingConnectorFactory) createLogsToLogs(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	logs consumer.Logs,
) (connector.Logs, error) {
	if f.defaultOtelColFunctionsOverridden || f.defaultResourceFunctionsOverridden || f.defaultLogFunctionsOverridden {
		set.Logger.Debug(`non-default OTTL functions have been registered in the "routing" connector`,
			zap.Bool("otelcol", f.defaultOtelColFunctionsOverridden),
			zap.Bool("resource", f.defaultResourceFunctionsOverridden),
			zap.Bool("log", f.defaultLogFunctionsOverridden),
		)
	}
	return newLogsConnector(set, cfg, logs)
}
