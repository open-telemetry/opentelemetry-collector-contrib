// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudspannerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	typeStr = "googlecloudspanner"

	defaultCollectionInterval          = 60 * time.Second
	defaultTopMetricsQueryMaxRows      = 100
	defaultBackfillEnabled             = false
	defaultHideTopnQuerystatsQuerytext = false
)

func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
			CollectionInterval: defaultCollectionInterval,
		},
		TopMetricsQueryMaxRows:      defaultTopMetricsQueryMaxRows,
		BackfillEnabled:             defaultBackfillEnabled,
		HideTopnQuerystatsQuerytext: defaultHideTopnQuerystatsQuerytext,
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings component.ReceiverCreateSettings,
	baseCfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {

	rCfg := baseCfg.(*Config)
	r := newGoogleCloudSpannerReceiver(settings.Logger, rCfg)

	scraper, err := scraperhelper.NewScraper(typeStr, r.Scrape, scraperhelper.WithStart(r.Start),
		scraperhelper.WithShutdown(r.Shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&rCfg.ScraperControllerSettings, settings, consumer,
		scraperhelper.AddScraper(scraper))
}
