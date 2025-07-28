// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

const defaultMongoDBPort = 27017

var defaultEndpoint = "localhost:" + strconv.Itoa(defaultMongoDBPort)

// NewFactory creates a factory for mongodb receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelBeta))
}

func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Timeout:          time.Minute,
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		ClientConfig:         configtls.ClientConfig{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	ms := newMongodbScraper(params, cfg)

	s, err := scraper.NewMetrics(
		ms.scrape,
		scraper.WithStart(ms.start),
		scraper.WithShutdown(ms.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}

func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)
	ms := newMongodbScraper(params, cfg)

	opts := make([]scraperhelper.ControllerOption, 0)

	if cfg.Events.DbServerQuerySample.Enabled {
		s, err := scraper.NewLogs(
			ms.scrapeLogs,
			scraper.WithStart(ms.start),
			scraper.WithShutdown(ms.shutdown),
		)
		if err != nil {
			ms.logger.Error("failed to create log scraper", zap.Error(err))
		} else {
			opts = append(
				opts,
				scraperhelper.AddFactoryWithConfig(
					scraper.NewFactory(
						metadata.Type, nil, scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
							return s, nil
						}, component.StabilityLevelAlpha)), nil))
		}
	}
	return scraperhelper.NewLogsController(
		&cfg.ControllerConfig, params, consumer, opts...)
}
