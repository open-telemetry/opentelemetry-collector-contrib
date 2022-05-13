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

package bigipreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/metadata"
)

const typeStr = "bigip"

var errConfigNotBigip = errors.New("config was not a Big-IP receiver config")

// NewFactory creates a new receiver factory for Big-IP
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver))
}

// createDefaultConfig creates a config for Big-IP with as many default values as possible
func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
			CollectionInterval: 10 * time.Second,
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: defaultEndpoint,
			Timeout:  10 * time.Second,
		},
		Metrics: metadata.DefaultMetricsSettings(),
	}
}

// creates the metric receiver for Big-IP
func createMetricsReceiver(_ context.Context, params component.ReceiverCreateSettings, rConf config.Receiver, consumer consumer.Metrics) (component.MetricsReceiver, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotBigip
	}

	bigipScraper := newScraper(params.Logger, cfg, params.TelemetrySettings)
	scraper, err := scraperhelper.NewScraper(typeStr, bigipScraper.scrape, scraperhelper.WithStart(bigipScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&cfg.ScraperControllerSettings, params, consumer, scraperhelper.AddScraper(scraper))
}
