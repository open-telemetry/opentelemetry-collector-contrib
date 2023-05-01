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

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	typeStr   = "routing"
	stability = component.StabilityLevelDevelopment
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		typeStr,
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, stability),
		connector.WithMetricsToMetrics(createMetricsToMetrics, stability),
		connector.WithLogsToLogs(createLogsToLogs, stability),
	)
}

// createDefaultConfig creates the default configuration.
func createDefaultConfig() component.Config {
	return &Config{
		ErrorMode: ottl.PropagateError,
	}
}

func createTracesToTraces(_ context.Context, set connector.CreateSettings, cfg component.Config, traces consumer.Traces) (connector.Traces, error) {
	return newTracesConnector(set, cfg, traces)
}

func createMetricsToMetrics(_ context.Context, set connector.CreateSettings, cfg component.Config, metrics consumer.Metrics) (connector.Metrics, error) {
	return newMetricsConnector(set, cfg, metrics)
}

func createLogsToLogs(_ context.Context, set connector.CreateSettings, cfg component.Config, logs consumer.Logs) (connector.Logs, error) {
	return newLogsConnector(set, cfg, logs)
}
