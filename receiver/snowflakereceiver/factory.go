// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver/internal/metadata"
)

const (
	typeStr         = "snowflake"
	defaultInterval = 30 * time.Minute
	defaultRole     = "ACCOUNTADMIN"
	defaultDB       = "SNOWFLAKE"
	defaultSchema   = "ACCOUNT_USAGE"
)

func createDefaultConfig() component.Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: defaultInterval,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Schema:               defaultSchema,
		Database:             defaultDB,
		Role:                 defaultRole,
	}
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelDevelopment),
	)
}

func createMetricsReceiver(_ context.Context,
	params receiver.CreateSettings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)
	snowflakeScraper := newSnowflakeMetricsScraper(params, cfg)

	scraper, err := scraperhelper.NewScraper(typeStr, snowflakeScraper.scrape, scraperhelper.WithStart(snowflakeScraper.start), scraperhelper.WithShutdown(snowflakeScraper.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings,
		params,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}
