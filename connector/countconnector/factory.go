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
	return &Config{
		Traces: MetricInfo{
			Name:        defaultMetricNameSpans,
			Description: defaultMetricDescSpans,
		},
		Metrics: MetricInfo{
			Name:        defaultMetricNameDataPoints,
			Description: defaultMetricDescDataPoints,
		},
		Logs: MetricInfo{
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
	return &count{cfg: *c, metricsConsumer: nextConsumer}, nil
}

// createMetricsToMetrics creates a metrics connector based on provided config.
func createMetricsToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Metrics, error) {
	c := cfg.(*Config)
	return &count{cfg: *c, metricsConsumer: nextConsumer}, nil
}

// createLogsToMetrics creates a logs to metrics connector based on provided config.
func createLogsToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Logs, error) {
	c := cfg.(*Config)
	return &count{cfg: *c, metricsConsumer: nextConsumer}, nil
}
