// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudspannerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	defaultCollectionInterval                = 60 * time.Second
	defaultTopMetricsQueryMaxRows            = 100
	defaultBackfillEnabled                   = false
	defaultHideTopnLockstatsRowrangestartkey = false
	defaultTruncateText                      = false
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ScraperControllerSettings:         scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
		TopMetricsQueryMaxRows:            defaultTopMetricsQueryMaxRows,
		BackfillEnabled:                   defaultBackfillEnabled,
		HideTopnLockstatsRowrangestartkey: defaultHideTopnLockstatsRowrangestartkey,
		TruncateText:                      defaultTruncateText,
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {

	rCfg := baseCfg.(*Config)
	r := newGoogleCloudSpannerReceiver(settings.Logger, rCfg)

	scraper, err := scraperhelper.NewScraper(metadata.Type, r.Scrape, scraperhelper.WithStart(r.Start),
		scraperhelper.WithShutdown(r.Shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&rCfg.ScraperControllerSettings, settings, consumer,
		scraperhelper.AddScraper(scraper))
}
