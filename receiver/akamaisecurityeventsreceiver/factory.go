// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package akamaisecurityeventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/akamaisecurityeventsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/akamaisecurityeventsreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	clientConfig := confighttp.NewDefaultClientConfig()

	return &Config{
		ControllerConfig: cfg,
		ClientConfig:     clientConfig,
		Limit:            10000,
		ParseRuleData:    true,
	}
}

func createLogsReceiver(
	_ context.Context,
	settings receiver.Settings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)
	as := newAkamaiSecurityEventsScraper(settings, cfg)

	return scraperhelper.NewLogsController(
		&cfg.ControllerConfig, settings, consumer,
		scraperhelper.AddFactoryWithConfig(
			scraper.NewFactory(metadata.Type, nil,
				scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
					return scraper.NewLogs(as.scrape, scraper.WithStart(as.start), scraper.WithShutdown(as.shutdown))
				}, component.StabilityLevelAlpha)), nil),
	)
}
