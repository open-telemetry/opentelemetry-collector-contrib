// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signaltometricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/customottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return xconnector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xconnector.WithTracesToMetrics(createTracesToMetrics, metadata.TracesToMetricsStability),
		xconnector.WithMetricsToMetrics(createMetricsToMetrics, metadata.MetricsToMetricsStability),
		xconnector.WithLogsToMetrics(createLogsToMetrics, metadata.LogsToMetricsStability),
		xconnector.WithProfilesToMetrics(createProfilesToMetrics, metadata.ProfilesToMetricsStability),
		xconnector.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createDefaultConfig() component.Config {
	return &config.Config{
		ErrorMode: ottl.PropagateError,
	}
}

func createTracesToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Traces, error) {
	c := cfg.(*config.Config)
	ottlParser, err := ottlspan.NewParser(customottl.SpanFuncs(), set.TelemetrySettings, ottlspan.EnablePathContextNames())
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL parser for spans: %w", err)
	}
	pc, err := ottl.NewParserCollection(
		set.TelemetrySettings,
		ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ValueExpression[*ottlspan.TransformContext]](true),
		ottl.WithParserCollectionContext(
			ottlspan.ContextName,
			&ottlParser,
			ottl.WithValueExpressionConverter(func(_ *ottl.ParserCollection[*ottl.ValueExpression[*ottlspan.TransformContext]], _ ottl.ValueExpressionsGetter, parsed []*ottl.ValueExpression[*ottlspan.TransformContext]) (*ottl.ValueExpression[*ottlspan.TransformContext], error) {
				return parsed[0], nil
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL value expression collection for spans: %w", err)
	}

	metricDefs := make([]model.MetricDef[*ottlspan.TransformContext], 0, len(c.Spans))
	for i := range c.Spans {
		info := c.Spans[i]
		var conditions *ottl.ConditionSequence[*ottlspan.TransformContext]
		if len(info.Conditions) > 0 {
			conditions, err = filterottl.NewBoolExprForSpanWithPathContextNames(info.Conditions, customottl.SpanFuncs(), c.ErrorMode, set.TelemetrySettings)
			if err != nil {
				return nil, fmt.Errorf("failed to parse OTTL conditions: %w", err)
			}
		}
		var md model.MetricDef[*ottlspan.TransformContext]
		if err := md.FromMetricInfo(info, pc, ottlspan.ContextName, conditions); err != nil {
			return nil, fmt.Errorf("failed to parse provided metric information; %w", err)
		}
		metricDefs = append(metricDefs, md)
	}

	return &signalToMetrics{
		logger: set.Logger,
		collectorInstanceInfo: model.NewCollectorInstanceInfo(
			set.TelemetrySettings,
		),
		next:           nextConsumer,
		spanMetricDefs: metricDefs,
		errorMode:      c.ErrorMode,
	}, nil
}

func createMetricsToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Metrics, error) {
	c := cfg.(*config.Config)
	ottlParser, err := ottldatapoint.NewParser(customottl.DatapointFuncs(), set.TelemetrySettings, ottldatapoint.EnablePathContextNames())
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL parser for datapoints: %w", err)
	}
	pc, err := ottl.NewParserCollection(
		set.TelemetrySettings,
		ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ValueExpression[*ottldatapoint.TransformContext]](true),
		ottl.WithParserCollectionContext(
			ottldatapoint.ContextName,
			&ottlParser,
			ottl.WithValueExpressionConverter(func(_ *ottl.ParserCollection[*ottl.ValueExpression[*ottldatapoint.TransformContext]], _ ottl.ValueExpressionsGetter, parsed []*ottl.ValueExpression[*ottldatapoint.TransformContext]) (*ottl.ValueExpression[*ottldatapoint.TransformContext], error) {
				return parsed[0], nil
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL value expression collection for datapoints: %w", err)
	}

	metricDefs := make([]model.MetricDef[*ottldatapoint.TransformContext], 0, len(c.Datapoints))
	for i := range c.Datapoints {
		info := c.Datapoints[i]
		var conditions *ottl.ConditionSequence[*ottldatapoint.TransformContext]
		if len(info.Conditions) > 0 {
			conditions, err = filterottl.NewBoolExprForDataPointWithPathContextNames(info.Conditions, customottl.DatapointFuncs(), c.ErrorMode, set.TelemetrySettings)
			if err != nil {
				return nil, fmt.Errorf("failed to parse OTTL conditions: %w", err)
			}
		}
		var md model.MetricDef[*ottldatapoint.TransformContext]
		if err := md.FromMetricInfo(info, pc, ottldatapoint.ContextName, conditions); err != nil {
			return nil, fmt.Errorf("failed to parse provided metric information; %w", err)
		}
		metricDefs = append(metricDefs, md)
	}

	return &signalToMetrics{
		logger: set.Logger,
		collectorInstanceInfo: model.NewCollectorInstanceInfo(
			set.TelemetrySettings,
		),
		next:         nextConsumer,
		dpMetricDefs: metricDefs,
		errorMode:    c.ErrorMode,
	}, nil
}

func createLogsToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Logs, error) {
	c := cfg.(*config.Config)
	ottlParser, err := ottllog.NewParser(customottl.LogFuncs(), set.TelemetrySettings, ottllog.EnablePathContextNames())
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL parser for logs: %w", err)
	}
	pc, err := ottl.NewParserCollection(
		set.TelemetrySettings,
		ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ValueExpression[*ottllog.TransformContext]](true),
		ottl.WithParserCollectionContext(
			ottllog.ContextName,
			&ottlParser,
			ottl.WithValueExpressionConverter(func(_ *ottl.ParserCollection[*ottl.ValueExpression[*ottllog.TransformContext]], _ ottl.ValueExpressionsGetter, parsed []*ottl.ValueExpression[*ottllog.TransformContext]) (*ottl.ValueExpression[*ottllog.TransformContext], error) {
				return parsed[0], nil
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL value expression collection for logs: %w", err)
	}

	metricDefs := make([]model.MetricDef[*ottllog.TransformContext], 0, len(c.Logs))
	for i := range c.Logs {
		info := c.Logs[i]
		var conditions *ottl.ConditionSequence[*ottllog.TransformContext]
		if len(info.Conditions) > 0 {
			conditions, err = filterottl.NewBoolExprForLogWithPathContextNames(info.Conditions, customottl.LogFuncs(), c.ErrorMode, set.TelemetrySettings)
			if err != nil {
				return nil, fmt.Errorf("failed to parse OTTL conditions: %w", err)
			}
		}
		var md model.MetricDef[*ottllog.TransformContext]
		if err := md.FromMetricInfo(info, pc, ottllog.ContextName, conditions); err != nil {
			return nil, fmt.Errorf("failed to parse provided metric information; %w", err)
		}
		metricDefs = append(metricDefs, md)
	}

	return &signalToMetrics{
		logger: set.Logger,
		collectorInstanceInfo: model.NewCollectorInstanceInfo(
			set.TelemetrySettings,
		),
		next:          nextConsumer,
		logMetricDefs: metricDefs,
		errorMode:     c.ErrorMode,
	}, nil
}

func createProfilesToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (xconnector.Profiles, error) {
	c := cfg.(*config.Config)
	ottlParser, err := ottlprofile.NewParser(customottl.ProfileFuncs(), set.TelemetrySettings, ottlprofile.EnablePathContextNames())
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL parser for profiles: %w", err)
	}
	pc, err := ottl.NewParserCollection(
		set.TelemetrySettings,
		ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ValueExpression[*ottlprofile.TransformContext]](true),
		ottl.WithParserCollectionContext(
			ottlprofile.ContextName,
			&ottlParser,
			ottl.WithValueExpressionConverter(func(_ *ottl.ParserCollection[*ottl.ValueExpression[*ottlprofile.TransformContext]], _ ottl.ValueExpressionsGetter, parsed []*ottl.ValueExpression[*ottlprofile.TransformContext]) (*ottl.ValueExpression[*ottlprofile.TransformContext], error) {
				return parsed[0], nil
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL value expression collection for profiles: %w", err)
	}

	metricDefs := make([]model.MetricDef[*ottlprofile.TransformContext], 0, len(c.Profiles))
	for i := range c.Profiles {
		info := c.Profiles[i]
		var conditions *ottl.ConditionSequence[*ottlprofile.TransformContext]
		if len(info.Conditions) > 0 {
			conditions, err = filterottl.NewBoolExprForProfileWithPathContextNames(info.Conditions, customottl.ProfileFuncs(), c.ErrorMode, set.TelemetrySettings)
			if err != nil {
				return nil, fmt.Errorf("failed to parse OTTL conditions: %w", err)
			}
		}
		var md model.MetricDef[*ottlprofile.TransformContext]
		if err := md.FromMetricInfo(info, pc, ottlprofile.ContextName, conditions); err != nil {
			return nil, fmt.Errorf("failed to parse provided metric information; %w", err)
		}
		metricDefs = append(metricDefs, md)
	}

	return &signalToMetrics{
		logger: set.Logger,
		collectorInstanceInfo: model.NewCollectorInstanceInfo(
			set.TelemetrySettings,
		),
		next:              nextConsumer,
		profileMetricDefs: metricDefs,
		errorMode:         c.ErrorMode,
	}, nil
}
