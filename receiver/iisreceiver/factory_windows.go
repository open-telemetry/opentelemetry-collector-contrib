// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	rcvr := newIisReceiver(params, cfg, nextConsumer)

	s, err := scraper.NewMetrics(rcvr.scrape,
		scraper.WithStart(rcvr.start),
		scraper.WithShutdown(rcvr.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ControllerConfig, params, nextConsumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}
