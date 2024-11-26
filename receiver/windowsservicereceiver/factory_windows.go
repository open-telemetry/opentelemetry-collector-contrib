// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package windowsservicereceiver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/scraper"
)

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	_stgs := scraper.Settings{}
	s := newWindowsServiceScraper(params, c)

	scp, err := scraper.NewMetrics(context.Background(), _stgs, s.scrape, scraper.WithStart(s.start), scraper.WithShutdown(s.shutdown))
	if err != nil {
		return nil, err
	}

	scopt := scraperhelper.AddScraper(metadata.Type, scp)

	return scraperhelper.NewScraperControllerReceiver(&c.ControllerConfig, params, consumer, scopt)
}
