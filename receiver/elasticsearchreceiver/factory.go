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
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

const (
	typeStr                   = "elasticsearch"
	stability                 = component.StabilityLevelBeta
	defaultCollectionInterval = 10 * time.Second
	defaultHTTPClientTimeout  = 10 * time.Second
)

// NewFactory creates a factory for elasticsearch receiver.
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver, stability))
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

func logDeprecatedFeatureGateForDirection(log *zap.Logger, gate featuregate.Gate) {
	log.Warn("WARNING: The " + gate.ID + " feature gate is deprecated and will be removed in the next release. The change to remove " +
		"the direction attribute has been reverted in the specification. See https://github.com/open-telemetry/opentelemetry-specification/issues/2726 " +
		"for additional details.")
}

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
	es := newElasticSearchScraper(params, c)

	if !es.emitMetricsWithDirectionAttribute {
		logDeprecatedFeatureGateForDirection(es.settings.Logger, emitMetricsWithDirectionAttributeFeatureGate)
	}

	if es.emitMetricsWithoutDirectionAttribute {
		logDeprecatedFeatureGateForDirection(es.settings.Logger, emitMetricsWithoutDirectionAttributeFeatureGate)
	}
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
