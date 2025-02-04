// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zookeeperscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/scraper/zookeeperscraper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/scraper/zookeeperscraper/internal/metadata"
)

const (
	defaultEndpoint = "localhost:2181"
)

func NewFactory() scraper.Factory {
	return scraper.NewFactory(
		metadata.Type,
		createDefaultConfig,
		scraper.WithMetrics(createMetrics, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TCPAddrConfig: confignet.TCPAddrConfig{
			Endpoint: defaultEndpoint,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// createMetrics creates zookeeper (metrics) scraper.
func createMetrics(
	_ context.Context,
	params scraper.Settings,
	config component.Config,
) (scraper.Metrics, error) {
	return newZookeeperMetricsScraper(params, config.(*Config)), nil
}
