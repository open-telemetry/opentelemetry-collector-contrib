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

//go:build !windows
// +build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
)

type receiver struct {
	config        *Config
	set           rcvr.CreateSettings
	clientFactory clientFactory
	scraper       *ContainerScraper
}

func newReceiver(
	_ context.Context,
	set rcvr.CreateSettings,
	config *Config,
	nextConsumer consumer.Metrics,
	clientFactory clientFactory,
) (rcvr.Metrics, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	if clientFactory == nil {
		clientFactory = newLibpodClient
	}

	recv := &receiver{
		config:        config,
		clientFactory: clientFactory,
		set:           set,
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

	r.scraper = newContainerScraper(podmanClient, r.set.Logger, r.config)
	if err = r.scraper.loadContainerList(ctx); err != nil {
		return err
	}
	go r.scraper.containerEventLoop(ctx)
	return nil
}

type result struct {
	md  pmetric.Metrics
	err error
}

func (r *receiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	containers := r.scraper.getContainers()
	results := make(chan result, len(containers))

	wg := &sync.WaitGroup{}
	wg.Add(len(containers))
	for _, c := range containers {
		go func(c container) {
			defer wg.Done()
			stats, err := r.scraper.fetchContainerStats(ctx, c)
			if err != nil {
				results <- result{md: pmetric.Metrics{}, err: err}
				return
			}
			results <- result{md: containerStatsToMetrics(time.Now(), c, &stats), err: nil}
		}(c)
	}

	wg.Wait()
	close(results)

	var errs error
	md := pmetric.NewMetrics()
	for res := range results {
		if res.err != nil {
			// Don't know the number of failed metrics, but one container fetch is a partial error.
			errs = multierr.Append(errs, scrapererror.NewPartialScrapeError(res.err, 0))
			fmt.Println("No stats found!")
			continue
		}
		res.md.ResourceMetrics().CopyTo(md.ResourceMetrics())
	}
	return md, nil
}
