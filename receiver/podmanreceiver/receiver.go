// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver/internal/metadata"
)

type metricsReceiver struct {
	config        *Config
	set           receiver.CreateSettings
	clientFactory clientFactory
	scraper       *ContainerScraper
}

func newMetricsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	config *Config,
	nextConsumer consumer.Metrics,
	clientFactory clientFactory,
) (receiver.Metrics, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	if clientFactory == nil {
		clientFactory = newLibpodClient
	}

	recv := &metricsReceiver{
		config:        config,
		clientFactory: clientFactory,
		set:           set,
	}

	scrp, err := scraperhelper.NewScraper(metadata.Type, recv.scrape, scraperhelper.WithStart(recv.start))
	if err != nil {
		return nil, err
	}
	return scraperhelper.NewScraperControllerReceiver(&recv.config.ScraperControllerSettings, set, nextConsumer, scraperhelper.AddScraper(scrp))
}

func (r *metricsReceiver) start(ctx context.Context, _ component.Host) error {
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

func (r *metricsReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
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
