// Copyright 2020 OpenTelemetry Authors
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

//go:build !windows
// +build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

type receiver struct {
	config        *Config
	set           component.ReceiverCreateSettings
	clientFactory clientFactory
	scraper       *ContainerScraper
}

func newReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	config *Config,
	nextConsumer consumer.Metrics,
	clientFactory clientFactory,
) (component.MetricsReceiver, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	if clientFactory == nil {
		clientFactory = newLibpodClient
	}

	recv := &receiver{
		config:        config,
		set:           set,
		clientFactory: clientFactory,
	}

	scrp, err := scraperhelper.NewScraper(typeStr, recv.scrape, scraperhelper.WithStart(recv.start))
	if err != nil {
		return nil, err
	}
	return scraperhelper.NewScraperControllerReceiver(&recv.config.ScraperControllerSettings, set, nextConsumer, scraperhelper.AddScraper(scrp))
}

func (r *receiver) start(ctx context.Context, _ component.Host) error {
	var err error
	podmanClient, err := r.clientFactory(r.set.Logger, r.config)
	if err != nil {
		return err
	}

	r.scraper = NewContainerScraper(podmanClient, r.set.Logger, r.config)
	if err = r.scraper.LoadContainerList(ctx); err != nil {
		return err
	}
	go r.scraper.ContainerEventLoop(ctx)
	return nil
}

func (r *receiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	stats, err := r.scraper.FetchContainerStats(ctx)
	if err != nil {
		r.set.Logger.Error("error fetching stats", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	md := pmetric.NewMetrics()
	for i := range stats {
		translateStatsToMetrics(&stats[i], time.Now(), md.ResourceMetrics().AppendEmpty())
	}
	return md, nil
}
