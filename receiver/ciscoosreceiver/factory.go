// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"
)

var scraperFactories = map[component.Type]scraper.Factory{
	component.MustNewType("system"): systemscraper.NewFactory(),
}

// NewFactory creates a factory for Cisco OS receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.Timeout = 10 * time.Second
	cfg.CollectionInterval = 60 * time.Second

	return &Config{
		ControllerConfig: cfg,
		Scrapers:         map[component.Type]component.Config{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	conf := cfg.(*Config)

	if conf.Device.Device.Host.IP == "" || len(conf.Scrapers) == 0 {
		return &nopMetricsReceiver{}, nil
	}

	var scraperOptions []scraperhelper.ControllerOption
	for scraperType, scraperCfg := range conf.Scrapers {
		factory, exists := scraperFactories[scraperType]
		if !exists {
			set.Logger.Warn("Unsupported scraper type", zap.String("type", scraperType.String()))
			continue
		}

		// Inject device configuration into scraper config
		if sysCfg, ok := scraperCfg.(*systemscraper.Config); ok {
			sysCfg.Device = convertToSystemScraperDeviceConfig(conf.Device)
		}

		scraperOptions = append(scraperOptions, scraperhelper.AddFactoryWithConfig(factory, scraperCfg))
	}

	if len(scraperOptions) == 0 {
		return &nopMetricsReceiver{}, nil
	}

	return scraperhelper.NewMetricsController(
		&conf.ControllerConfig,
		set,
		consumer,
		scraperOptions...,
	)
}

func convertToSystemScraperDeviceConfig(device DeviceConfig) systemscraper.DeviceConfig {
	return systemscraper.DeviceConfig{
		Host: systemscraper.HostInfo{
			Name: device.Device.Host.Name,
			IP:   device.Device.Host.IP,
			Port: device.Device.Host.Port,
		},
		Auth: systemscraper.AuthConfig{
			Username: device.Auth.Username,
			Password: string(device.Auth.Password),
			KeyFile:  device.Auth.KeyFile,
		},
	}
}

type nopMetricsReceiver struct{}

func (*nopMetricsReceiver) Start(_ context.Context, _ component.Host) error { return nil }
func (*nopMetricsReceiver) Shutdown(_ context.Context) error                { return nil }
