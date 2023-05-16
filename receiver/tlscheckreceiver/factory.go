// Copyright The OpenTelemetry Authors
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

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"context"
	"errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/configtls"

	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

const (
	typeStr = "tlscheck"
)

var errConfigNotTLSCheck = errors.New("config was not a TLS check receiver config")

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.Stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
		},
		TLSCertsClientSettings: configtls.TLSCertsClientSettings{
			Timeout: 10 * time.Second,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(ctx context.Context, params receiver.CreateSettings, rConfig component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	cfg, ok := rConfig.(*Config)
	if !ok {
		return nil, errConfigNotTLSCheck
	}

	tlsCheckScraper := newScraper(cfg, params)
	scraper, err := scraperhelper.NewScraper(typeStr, tlsCheckScraper.scrape, scraperhelper.WithStart(tlsCheckScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&cfg.ScraperControllerSettings, params, consumer, scraperhelper.AddScraper(scraper))
}
