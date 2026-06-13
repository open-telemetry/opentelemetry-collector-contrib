// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/metadata"
)

const (
	// The stability level of the processor.
	connectorStability = component.StabilityLevelDevelopment
)

var (
	virtualNodeFeatureGate = metadata.ConnectorServicegraphVirtualNodeFeatureGate
	// TODO: Remove this feature gate when the legacy metric names are removed.
	legacyMetricNamesFeatureGate   = metadata.ConnectorServicegraphLegacyLatencyMetricNamesFeatureGate
	legacyLatencyUnitMsFeatureGate = metadata.ConnectorServicegraphLegacyLatencyUnitMsFeatureGate
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return xconnector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xconnector.WithTracesToMetrics(createTracesToMetricsConnector, metadata.TracesToMetricsStability),
		xconnector.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Store: StoreConfig{
			TTL:      2 * time.Second,
			MaxItems: 1000,
		},
		CacheLoop:              time.Minute,
		StoreExpirationLoop:    2 * time.Second,
		MetricsTimestampOffset: 0,
	}
}

func createTracesToMetricsConnector(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	return newConnector(params.TelemetrySettings, cfg, nextConsumer)
}
