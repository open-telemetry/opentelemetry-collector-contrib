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

package redisreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	typeStr = "redis"
)

// NewFactory creates a factory for Redis receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	scs := scraperhelper.DefaultScraperControllerSettings(typeStr)
	scs.CollectionInterval = 10 * time.Second
	return &Config{
		Network:                   "tcp",
		ScraperControllerSettings: scs,
	}
}

func createMetricsReceiver(
	ctx context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	oCfg := cfg.(*Config)

	scrp, err := newRedisScraper(*oCfg, set, nil)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&oCfg.ScraperControllerSettings, set, consumer, scraperhelper.AddScraper(scrp))
}
