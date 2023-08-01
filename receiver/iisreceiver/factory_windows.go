// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	rConf component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	rcvr := newIisReceiver(params, cfg, nextConsumer)

	scraper, err := scraperhelper.NewScraper(metadata.Type, rcvr.scrape,
		scraperhelper.WithStart(rcvr.start),
		scraperhelper.WithShutdown(rcvr.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings, params, nextConsumer,
		scraperhelper.AddScraper(scraper),
	)
}
