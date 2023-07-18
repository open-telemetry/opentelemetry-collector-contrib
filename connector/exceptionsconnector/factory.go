// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
