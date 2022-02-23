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

package elasticsearchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

const (
	typeStr                   = "elasticsearch"
	defaultCollectionInterval = 10 * time.Second
	defaultHTTPClientTimeout  = 10 * time.Second
)

// NewFactory creates a factory for elasticsearch receiver.
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver))
}

// createDefaultConfig creates the default elasticsearchreceiver config.
func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
			CollectionInterval: defaultCollectionInterval,
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: defaultEndpoint,
			Timeout:  defaultHTTPClientTimeout,
		},
		Metrics: metadata.DefaultMetricsSettings(),
		Nodes:   []string{"_all"},
	}
}

var errConfigNotES = errors.New("config was not an elasticsearch receiver config")

// createMetricsReceiver creates a metrics receiver for scraping elasticsearch metrics.
func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotES
	}
	es := newElasticSearchScraper(params.TelemetrySettings, c)
	scraper, err := scraperhelper.NewScraper(typeStr, es.scrape, scraperhelper.WithStart(es.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&c.ScraperControllerSettings,
		params,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}
