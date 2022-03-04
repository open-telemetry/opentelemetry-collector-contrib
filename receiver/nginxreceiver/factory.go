// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nginxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
)

const (
	typeStr = "nginx"
)

// NewFactory creates a factory for nginx receiver.
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
			CollectionInterval: 10 * time.Second,
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:80/status",
			Timeout:  10 * time.Second,
		},
		Metrics: metadata.DefaultMetricsSettings(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	cfg := rConf.(*Config)

	ns := newNginxScraper(params.TelemetrySettings, cfg)
	scraper, err := scraperhelper.NewScraper(typeStr, ns.scrape, scraperhelper.WithStart(ns.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings, params, consumer,
		scraperhelper.AddScraper(scraper),
	)
}
