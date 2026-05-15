// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grafanacloudconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/grafanacloudconnector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/grafanacloudconnector/internal/metadata"
)

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
		HostIdentifiers:      []string{"host.id", "k8s.node.uid", "k8s.node.name"},
		MetricsFlushInterval: 60 * time.Second,
	}
}

func createTracesToMetricsConnector(_ context.Context, params connector.Settings, cfg component.Config, next consumer.Metrics) (connector.Traces, error) {
	c, err := newConnector(params.Logger, params.TelemetrySettings, cfg)
	if err != nil {
		return nil, err
	}
	c.metricsConsumer = next
	return c, nil
}
