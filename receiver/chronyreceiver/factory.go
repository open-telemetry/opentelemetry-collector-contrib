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
)

const (
	typeStr = "chrony"

	// The stability level of the receiver.
	stability = component.StabilityLevelAlpha
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		newDefaultCongfig,
		receiver.WithMetrics(newMetricsReceiver, stability),
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
		typeStr,
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
