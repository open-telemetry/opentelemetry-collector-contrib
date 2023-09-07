// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chronyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		newDefaultCongfig,
		receiver.WithMetrics(newMetricsReceiver, metadata.MetricsStability),
	)
}

func newMetricsReceiver(
	ctx context.Context,
	set receiver.CreateSettings,
	rCfg component.Config,
	consumer consumer.Metrics) (receiver.Metrics, error) {
	cfg, ok := rCfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("wrong config provided: %w", errInvalidValue)
	}

	chronyc, err := chrony.New(cfg.Endpoint, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	scraper, err := scraperhelper.NewScraper(
		metadata.Type,
		newScraper(ctx, chronyc, cfg, set).scrape,
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings,
		set,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}
