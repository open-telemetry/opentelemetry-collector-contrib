// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"context"
	"errors"

	"github.com/coreos/go-systemd/v22/dbus"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

// NewFactory creates a factory for systemd receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Units:                make([]string, 0),
	}
}

func createMetricsReceiver(
	ctx context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("provided configuration is invalid")
	}

	f := func(ctx context.Context) (dbusClient, error) {
		return dbus.NewSystemConnectionContext(ctx)
	}

	s := newSystemdReceiver(settings, config, f)

	sm, err := scraper.NewMetrics(s.scrape, scraper.WithStart(s.Start), scraper.WithShutdown(s.Shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&config.ControllerConfig,
		settings,
		consumer,
		scraperhelper.AddScraper(metadata.Type, sm),
	)
}
