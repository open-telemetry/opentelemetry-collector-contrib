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

package nginxreceiver

//go:generate mdatagen metadata.yaml

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	typeStr = "nginx"
)

// NewFactory creates a factory for nginx receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func createDefaultConfig() configmodels.Receiver {
	return &config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: typeStr,
				NameVal: typeStr,
			},
			CollectionInterval: 10 * time.Second,
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://localhost:80/status",
			Timeout:  10 * time.Second,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateParams,
	rConf configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	cfg := rConf.(*config)

	ns := newNginxScraper(params.Logger, cfg)
	scraper := scraperhelper.NewResourceMetricsScraper(typeStr, ns.scrape)

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings, params.Logger, consumer,
		scraperhelper.AddResourceMetricsScraper(scraper),
	)
}
