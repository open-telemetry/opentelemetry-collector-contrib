// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ocsfconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/ocsfconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/ocsfconnector/internal/metadata"
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithLogsToLogs(createLogsConnector, metadata.LogsToLogsStability),
	)
}

// createDefaultConfig creates the default configuration.
func createDefaultConfig() component.Config {
	return &Config{}
}

// createLogsConnector returns a connector which consume logs and export logs
func createLogsConnector(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (connector.Logs, error) {
	return newLogsConnector(set, cfg, nextConsumer)
}
