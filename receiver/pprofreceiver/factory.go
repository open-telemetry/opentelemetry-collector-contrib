// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.opentelemetry.io/collector/scraper/scraperhelper/xscraperhelper"
	"go.opentelemetry.io/collector/scraper/xscraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithProfiles(createProfilesReceiver, metadata.ProfilesStability))
}

func createDefaultConfig() component.Config {
	scraperCfg := scraperhelper.NewDefaultControllerConfig()
	scraperCfg.CollectionInterval = 10 * time.Second

	return &Config{
		ControllerConfig: scraperCfg,
		Include:          "",
	}
}

func createProfilesReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	rCfg := cfg.(*Config)

	scraperFactory := xscraper.NewFactory(
		metadata.Type,
		func() component.Config { return &Config{} },
		xscraper.WithProfiles(func(_ context.Context, set scraper.Settings, _ component.Config) (xscraper.Profiles, error) {
			ps := newScraper(rCfg, receiver.Settings{
				ID:                set.ID,
				TelemetrySettings: set.TelemetrySettings,
				BuildInfo:         settings.BuildInfo,
			})
			return xscraper.NewProfiles(ps.scrape, xscraper.WithStart(ps.start))
		}, metadata.ProfilesStability),
	)

	return xscraperhelper.NewProfilesController(
		&rCfg.ControllerConfig,
		settings,
		consumer,
		xscraperhelper.AddFactoryWithConfig(scraperFactory, rCfg),
	)
}
