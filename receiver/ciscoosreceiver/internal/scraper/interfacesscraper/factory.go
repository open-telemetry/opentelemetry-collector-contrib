// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper/internal/metadata"
)

// NewFactory creates a factory for interfaces scraper.
func NewFactory() scraper.Factory {
	return scraper.NewFactory(
		component.MustNewType("interfaces"),
		createDefaultConfig,
		scraper.WithMetrics(createMetricsScraper, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsScraper(_ context.Context, settings scraper.Settings, cfg component.Config) (scraper.Metrics, error) {
	interfacesCfg := cfg.(*Config)
	return newInterfacesScraper(settings.Logger, interfacesCfg), nil
}

func newInterfacesScraper(logger *zap.Logger, config *Config) *interfacesScraper {
	return &interfacesScraper{
		logger: logger,
		config: config,
	}
}
