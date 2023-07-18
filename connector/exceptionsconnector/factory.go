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

//go:generate mdatagen metadata.yaml

package exceptionsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

// NewFactory creates a factory for the exceptions connector.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, metadata.TracesToMetricsStability),
		connector.WithTracesToLogs(createTracesToLogsConnector, metadata.TracesToLogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Dimensions: []Dimension{
			{Name: exceptionTypeKey},
			{Name: exceptionMessageKey},
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
	lc, err := newLogsConnector(params.Logger, cfg)
	if err != nil {
		return nil, err
	}
	lc.logsConsumer = nextConsumer
	return lc, nil
}
