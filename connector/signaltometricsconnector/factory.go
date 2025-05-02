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
	)
}

func createDefaultConfig() component.Config {
	return &config.Config{}
}

func createTracesToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Traces, error) {
	c := cfg.(*config.Config)
	parser, err := ottlspan.NewParser(customottl.SpanFuncs(), set.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL statement parser for spans: %w", err)
	}

	metricDefs := make([]model.MetricDef[ottlspan.TransformContext], 0, len(c.Spans))
	for _, info := range c.Spans {
		var md model.MetricDef[ottlspan.TransformContext]
		if err := md.FromMetricInfo(info, parser, set.TelemetrySettings); err != nil {
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
	}, nil
}

func createMetricsToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Metrics, error) {
	c := cfg.(*config.Config)
	parser, err := ottldatapoint.NewParser(customottl.DatapointFuncs(), set.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL statement parser for datapoints: %w", err)
	}

	metricDefs := make([]model.MetricDef[ottldatapoint.TransformContext], 0, len(c.Datapoints))
	for _, info := range c.Datapoints {
		var md model.MetricDef[ottldatapoint.TransformContext]
		if err := md.FromMetricInfo(info, parser, set.TelemetrySettings); err != nil {
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
	}, nil
}

func createLogsToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Logs, error) {
	c := cfg.(*config.Config)
	parser, err := ottllog.NewParser(customottl.LogFuncs(), set.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL statement parser for logs: %w", err)
	}

	metricDefs := make([]model.MetricDef[ottllog.TransformContext], 0, len(c.Logs))
	for _, info := range c.Logs {
		var md model.MetricDef[ottllog.TransformContext]
		if err := md.FromMetricInfo(info, parser, set.TelemetrySettings); err != nil {
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
	}, nil
}

func createProfilesToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (xconnector.Profiles, error) {
	c := cfg.(*config.Config)
	parser, err := ottlprofile.NewParser(customottl.ProfileFuncs(), set.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL statement parser for profiles: %w", err)
	}

	metricDefs := make([]model.MetricDef[ottlprofile.TransformContext], 0, len(c.Profiles))
	for _, info := range c.Profiles {
		var md model.MetricDef[ottlprofile.TransformContext]
		if err := md.FromMetricInfo(info, parser, set.TelemetrySettings); err != nil {
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
	}, nil
}
