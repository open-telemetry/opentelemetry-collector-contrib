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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	typeStr   = "count"
	stability = component.StabilityLevelDevelopment
	scopeName = "otelcol/countconnector"

	defaultMetricNameSpans = "trace.span.count"
	defaultMetricDescSpans = "The number of spans observed."

	defaultMetricNameDataPoints = "metric.data_point.count"
	defaultMetricDescDataPoints = "The number of data points observed."

	defaultMetricNameLogRecords = "log.record.count"
	defaultMetricDescLogRecords = "The number of log records observed."
)

// TypeConfig for a data type
type TypeConfig struct {
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
}

// Config for the connector
type Config struct {
	Traces  TypeConfig `mapstructure:"traces"`
	Metrics TypeConfig `mapstructure:"metrics"`
	Logs    TypeConfig `mapstructure:"logs"`
}

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
	return &Config{
		Traces: TypeConfig{
			Name:        defaultMetricNameSpans,
			Description: defaultMetricDescSpans,
		},
		Metrics: TypeConfig{
			Name:        defaultMetricNameDataPoints,
			Description: defaultMetricDescDataPoints,
		},
		Logs: TypeConfig{
			Name:        defaultMetricNameLogRecords,
			Description: defaultMetricDescLogRecords,
		},
	}
}

// createTracesToMetrics creates a traces to metrics connector based on provided config.
func createTracesToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Traces, error) {
	c := cfg.(*Config)
	return &count{Config: *c, Metrics: nextConsumer}, nil
}

// createMetricsToMetrics creates a metrics connector based on provided config.
func createMetricsToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Metrics, error) {
	c := cfg.(*Config)
	return &count{Config: *c, Metrics: nextConsumer}, nil
}

// createLogsToMetrics creates a logs to metrics connector based on provided config.
func createLogsToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Logs, error) {
	c := cfg.(*Config)
	return &count{Config: *c, Metrics: nextConsumer}, nil
}

// count can count spans, data points, or log records and emit
// the count onto a metrics pipeline.
type count struct {
	Config
	consumer.Metrics
	component.StartFunc
	component.ShutdownFunc
}

func (c *count) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *count) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	countMetrics := pmetric.NewMetrics()
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceSpan.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		var count int
		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			count += resourceSpan.ScopeSpans().At(j).Spans().Len()
		}
		setCountMetric(countScope.Metrics().AppendEmpty(), c.Config.Traces, count)
	}
	return c.Metrics.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	countMetrics := pmetric.NewMetrics()
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceMetric.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		count := 0
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			count += resourceMetric.ScopeMetrics().At(j).Metrics().Len()
		}
		setCountMetric(countScope.Metrics().AppendEmpty(), c.Config.Metrics, count)
	}
	return c.Metrics.ConsumeMetrics(ctx, countMetrics)
}

func (c *count) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	countMetrics := pmetric.NewMetrics()
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)
		countResource := countMetrics.ResourceMetrics().AppendEmpty()
		resourceLog.Resource().Attributes().CopyTo(countResource.Resource().Attributes())

		countScope := countResource.ScopeMetrics().AppendEmpty()
		countScope.Scope().SetName(scopeName)

		count := 0
		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			count += resourceLog.ScopeLogs().At(j).LogRecords().Len()
		}
		setCountMetric(countScope.Metrics().AppendEmpty(), c.Config.Logs, count)
	}
	return c.Metrics.ConsumeMetrics(ctx, countMetrics)
}

func setCountMetric(countMetric pmetric.Metric, metricType TypeConfig, count int) {
	countMetric.SetName(metricType.Name)
	countMetric.SetDescription(metricType.Description)
	sum := countMetric.SetEmptySum()
	sum.SetIsMonotonic(false)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntValue(int64(count))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
}
