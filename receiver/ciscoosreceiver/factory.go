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
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"
)

var scraperFactories = map[component.Type]scraper.Factory{
	component.MustNewType("system"):     systemscraper.NewFactory(),
	component.MustNewType("interfaces"): interfacesscraper.NewFactory(),
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

	if len(conf.Devices) == 0 {
		return &nopMetricsReceiver{}, nil
	}

	// Create one metrics controller per device
	var receivers []receiver.Metrics
	for i, device := range conf.Devices {
		if device.Host == "" || len(device.Scrapers) == 0 {
			set.Logger.Warn("Skipping device with missing host or scrapers",
				zap.Int("device_index", i),
				zap.String("device_name", device.Name))
			continue
		}

		connDevice := connection.DeviceConfig{
			Device: connection.DeviceInfo{
				Host: connection.HostInfo{
					Name: device.Name,
					IP:   device.Host,
					Port: device.Port,
				},
			},
			Auth: device.Auth,
		}

		var scraperOptions []scraperhelper.ControllerOption
		for scraperType, scraperCfg := range device.Scrapers {
			factory, exists := scraperFactories[scraperType]
			if !exists {
				set.Logger.Warn("Unsupported scraper type",
					zap.String("type", scraperType.String()),
					zap.String("device", device.Name))
				continue
			}

			freshCfg := factory.CreateDefaultConfig()

			if sysCfg, ok := scraperCfg.(*systemscraper.Config); ok {
				freshSysCfg := freshCfg.(*systemscraper.Config)
				freshSysCfg.MetricsBuilderConfig = sysCfg.MetricsBuilderConfig
				freshSysCfg.Device = connDevice
				freshCfg = freshSysCfg
			}
			if intfCfg, ok := scraperCfg.(*interfacesscraper.Config); ok {
				freshIntfCfg := freshCfg.(*interfacesscraper.Config)
				freshIntfCfg.MetricsBuilderConfig = intfCfg.MetricsBuilderConfig
				freshIntfCfg.Device = connDevice
				freshCfg = freshIntfCfg
			}

			scraperOptions = append(scraperOptions, scraperhelper.AddFactoryWithConfig(factory, freshCfg))
		}

		if len(scraperOptions) == 0 {
			continue
		}

		rcvr, err := scraperhelper.NewMetricsController(
			&conf.ControllerConfig,
			set,
			consumer,
			scraperOptions...,
		)
		if err != nil {
			return nil, err
		}
		receivers = append(receivers, rcvr)
	}

	if len(receivers) == 0 {
		return &nopMetricsReceiver{}, nil
	}

	if len(receivers) == 1 {
		return receivers[0], nil
	}

	return &multiMetricsReceiver{receivers: receivers}, nil
}

type nopMetricsReceiver struct{}

func (*nopMetricsReceiver) Start(_ context.Context, _ component.Host) error { return nil }
func (*nopMetricsReceiver) Shutdown(_ context.Context) error                { return nil }

// multiMetricsReceiver wraps multiple receivers for multi-device support.
type multiMetricsReceiver struct {
	receivers []receiver.Metrics
}

func (m *multiMetricsReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	for _, r := range m.receivers {
		err = multierr.Append(err, r.Start(ctx, host))
	}
	return err
}

func (m *multiMetricsReceiver) Shutdown(ctx context.Context) error {
	var err error
	for _, r := range m.receivers {
		err = multierr.Append(err, r.Shutdown(ctx))
	}
	return err
}
