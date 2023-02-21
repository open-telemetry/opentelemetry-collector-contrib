// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

const (
	typeStr   = "count"
	stability = component.StabilityLevelDevelopment
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		typeStr,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetrics, component.StabilityLevelDevelopment),
		connector.WithMetricsToMetrics(createMetricsToMetrics, component.StabilityLevelDevelopment),
		connector.WithLogsToMetrics(createLogsToMetrics, component.StabilityLevelDevelopment),
	)
}

// createDefaultConfig creates the default configuration.
func createDefaultConfig() component.Config {
	return &Config{}
}

// createTracesToMetrics creates a traces to metrics connector based on provided config.
func createTracesToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Traces, error) {
	c := cfg.(*Config)

	spanMatchExprs := make(map[string]expr.BoolExpr[ottlspan.TransformContext], len(c.Spans))
	spanParser, err := newSpanParser(set.TelemetrySettings.Logger)
	if err != nil {
		return nil, err
	}
	for name, info := range c.Spans {
		if len(info.Conditions) == 0 {
			continue
		}
		// Error checked in Config.Validate()
		spanMatchExprs[name], _ = parseConditions(spanParser, info.Conditions)
	}

	spanEventMatchExprs := make(map[string]expr.BoolExpr[ottlspanevent.TransformContext], len(c.SpanEvents))
	spanEventParser, err := newSpanEventParser(set.TelemetrySettings.Logger)
	if err != nil {
		return nil, err
	}
	for name, info := range c.SpanEvents {
		if len(info.Conditions) == 0 {
			continue
		}
		// Error checked in Config.Validate()
		spanEventMatchExprs[name], _ = parseConditions(spanEventParser, info.Conditions)
	}

	return &count{
		metricsConsumer: nextConsumer,
		spansCounterFactory: &counterFactory[ottlspan.TransformContext]{
			matchExprs:  spanMatchExprs,
			metricInfos: c.Spans,
		},
		spanEventsCounterFactory: &counterFactory[ottlspanevent.TransformContext]{
			matchExprs:  spanEventMatchExprs,
			metricInfos: c.SpanEvents,
		},
	}, nil
}

// createMetricsToMetrics creates a metricds to metrics connector based on provided config.
func createMetricsToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Metrics, error) {
	c := cfg.(*Config)

	metricMatchExprs := make(map[string]expr.BoolExpr[ottlmetric.TransformContext], len(c.Metrics))
	metricParser, err := newMetricParser(set.TelemetrySettings.Logger)
	if err != nil {
		return nil, err
	}
	for name, info := range c.Metrics {
		if len(info.Conditions) == 0 {
			continue
		}
		// Error checked in Config.Validate()
		metricMatchExprs[name], _ = parseConditions(metricParser, info.Conditions)
	}

	dataPointMatchExprs := make(map[string]expr.BoolExpr[ottldatapoint.TransformContext], len(c.DataPoints))
	dataPointParser, err := newDataPointParser(set.TelemetrySettings.Logger)
	if err != nil {
		return nil, err
	}
	for name, info := range c.DataPoints {
		if len(info.Conditions) == 0 {
			continue
		}
		// Error checked in Config.Validate()
		dataPointMatchExprs[name], _ = parseConditions(dataPointParser, info.Conditions)
	}

	return &count{
		metricsConsumer: nextConsumer,
		metricsCounterFactory: &counterFactory[ottlmetric.TransformContext]{
			matchExprs:  metricMatchExprs,
			metricInfos: c.Metrics,
		},
		dataPointsCounterFactory: &counterFactory[ottldatapoint.TransformContext]{
			matchExprs:  dataPointMatchExprs,
			metricInfos: c.DataPoints,
		},
	}, nil
}

// createLogsToMetrics creates a logs to metrics connector based on provided config.
func createLogsToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Logs, error) {
	c := cfg.(*Config)

	matchExprs := make(map[string]expr.BoolExpr[ottllog.TransformContext], len(c.Logs))
	logParser, err := newLogParser(set.TelemetrySettings.Logger)
	if err != nil {
		return nil, err
	}
	for name, info := range c.Logs {
		if len(info.Conditions) == 0 {
			continue
		}
		// Error checked in Config.Validate()
		matchExprs[name], _ = parseConditions(logParser, info.Conditions)
	}

	return &count{
		metricsConsumer: nextConsumer,
		logsCounterFactory: &counterFactory[ottllog.TransformContext]{
			matchExprs:  matchExprs,
			metricInfos: c.Logs,
		},
	}, nil
}
