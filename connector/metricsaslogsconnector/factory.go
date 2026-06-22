// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaslogsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/metricsaslogsconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/metricsaslogsconnector/internal/metadata"
)

func NewFactory() connector.Factory {
	return xconnector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xconnector.WithMetricsToLogs(createMetricsToLogs, component.StabilityLevelAlpha),
		xconnector.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createMetricsToLogs(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (connector.Metrics, error) {
	c := cfg.(*Config)
	return &metricsAsLogs{
		logsConsumer: nextConsumer,
		config:       c,
		logger:       set.Logger,
	}, nil
}
