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

package memcachedreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/service/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver/internal/metadata"
)

const (
	typeStr                   = "memcached"
	stability                 = component.StabilityLevelBeta
	defaultEndpoint           = "localhost:11211"
	defaultTimeout            = 10 * time.Second
	defaultCollectionInterval = 10 * time.Second
)

// NewFactory creates a factory for memcached receiver.
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver, stability))
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
			CollectionInterval: defaultCollectionInterval,
		},
		Timeout: defaultTimeout,
		NetAddr: confignet.NetAddr{
			Endpoint: defaultEndpoint,
		},
		Metrics: metadata.DefaultMetricsSettings(),
	}
}

func logDeprecatedFeatureGateForDirection(log *zap.Logger, gate featuregate.Gate) {
	log.Warn("WARNING: The " + gate.ID + " feature gate is deprecated and will be removed in the next release. The change to remove " +
		"the direction attribute has been reverted in the specification. See https://github.com/open-telemetry/opentelemetry-specification/issues/2726 " +
		"for additional details.")
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	cfg := rConf.(*Config)

	ms := newMemcachedScraper(params, cfg)

	if !ms.emitMetricsWithDirectionAttribute {
		logDeprecatedFeatureGateForDirection(ms.logger, emitMetricsWithDirectionAttributeFeatureGate)
	}

	if ms.emitMetricsWithoutDirectionAttribute {
		logDeprecatedFeatureGateForDirection(ms.logger, emitMetricsWithoutDirectionAttributeFeatureGate)
	}

	scraper, err := scraperhelper.NewScraper(typeStr, ms.scrape)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings, params, consumer,
		scraperhelper.AddScraper(scraper),
	)
}
