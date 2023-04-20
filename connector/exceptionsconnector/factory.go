// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exceptionsconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const (
	// The value of "type" key in configuration.
	typeStr = "exceptions"
	// The stability level of the processor.
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for the exceptions connector.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		typeStr,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, stability),
		connector.WithTracesToLogs(createTracesToLogsConnector, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Dimensions: []Dimension{
			{Name: "exception.type"},
			{Name: "exception.message"},
		},
	}
}

func createTracesToMetricsConnector(ctx context.Context, params connector.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	mc, err := newMetricsConnector(params.Logger, cfg)
	if err != nil {
		return nil, err
	}
	mc.metricsConsumer = nextConsumer
	return mc, nil
}

func createTracesToLogsConnector(ctx context.Context, params connector.CreateSettings, cfg component.Config, nextConsumer consumer.Logs) (connector.Traces, error) {
	lc, err := newLogsConnector(params.Logger)
	if err != nil {
		return nil, err
	}
	lc.logsConsumer = nextConsumer
	return lc, nil
}
