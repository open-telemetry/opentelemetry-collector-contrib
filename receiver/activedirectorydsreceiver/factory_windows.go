// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package activedirectorydsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver/internal/metadata"
)

var errConfigNotActiveDirectory = fmt.Errorf("config is not valid for the '%s' receiver", metadata.Type)

func createMetricsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	c, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotActiveDirectory
	}

	adds := newActiveDirectoryDSScraper(c.MetricsBuilderConfig, params)
	scraper, err := scraperhelper.NewScraper(
		metadata.Type,
		adds.scrape,
		scraperhelper.WithStart(adds.start),
		scraperhelper.WithShutdown(adds.shutdown),
	)

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
