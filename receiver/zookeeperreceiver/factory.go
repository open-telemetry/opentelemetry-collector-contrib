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

package zookeeperreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper"
)

const (
	typeStr = "zookeeper"

	defaultCollectionInterval = 10 * time.Second
	defaultTimeout            = 10 * time.Second
)

func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver),
	)
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
			CollectionInterval: defaultCollectionInterval,
		},
		TCPAddr: confignet.TCPAddr{
			Endpoint: ":2181",
		},
		Timeout: defaultTimeout,
	}
}

// CreateMetricsReceiver creates zookeeper (metrics) receiver.
func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	config config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	rConfig := config.(*Config)
	zms, err := newZookeeperMetricsScraper(params.Logger, rConfig)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&rConfig.ScraperControllerSettings,
		params,
		consumer,
		scraperhelper.AddScraper(
			scraperhelper.NewResourceMetricsScraper(
				rConfig.ID(),
				zms.scrape,
				scraperhelper.WithShutdown(zms.shutdown),
			),
		),
	)
}
