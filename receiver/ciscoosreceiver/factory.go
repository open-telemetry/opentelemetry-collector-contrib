// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"
)

var scraperFactories = map[component.Type]scraper.Factory{
	component.MustNewType("interfaces"): interfacesscraper.NewFactory(),
	component.MustNewType("system"):     systemscraper.NewFactory(),
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
	return &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Devices:          []DeviceConfig{},
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

	if len(conf.Devices) == 0 || len(conf.Scrapers) == 0 {
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
			sysCfg.Devices = convertToSystemScraperDeviceConfigs(conf.Devices)
		}
		if intCfg, ok := scraperCfg.(*interfacesscraper.Config); ok {
			intCfg.Devices = convertToInterfaceScraperDeviceConfigs(conf.Devices)
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

func convertToSystemScraperDeviceConfigs(devices []DeviceConfig) []systemscraper.DeviceConfig {
	scraperDevices := make([]systemscraper.DeviceConfig, len(devices))
	for i, dev := range devices {
		scraperDevices[i] = systemscraper.DeviceConfig{
			Host: systemscraper.HostInfo{
				Name: dev.Device.Host.Name,
				IP:   dev.Device.Host.IP,
				Port: dev.Device.Host.Port,
			},
			Auth: systemscraper.AuthConfig{
				Username: dev.Auth.Username,
				Password: string(dev.Auth.Password),
				KeyFile:  dev.Auth.KeyFile,
			},
		}
	}
	return scraperDevices
}

func convertToInterfaceScraperDeviceConfigs(devices []DeviceConfig) []interfacesscraper.DeviceConfig {
	scraperDevices := make([]interfacesscraper.DeviceConfig, len(devices))
	for i, dev := range devices {
		scraperDevices[i] = interfacesscraper.DeviceConfig{
			Host: interfacesscraper.HostInfo{
				Name: dev.Device.Host.Name,
				IP:   dev.Device.Host.IP,
				Port: dev.Device.Host.Port,
			},
			Auth: interfacesscraper.AuthConfig{
				Username: dev.Auth.Username,
				Password: string(dev.Auth.Password),
				KeyFile:  dev.Auth.KeyFile,
			},
		}
	}
	return scraperDevices
}

type nopMetricsReceiver struct{}

func (*nopMetricsReceiver) Start(_ context.Context, _ component.Host) error { return nil }
func (*nopMetricsReceiver) Shutdown(_ context.Context) error                { return nil }
