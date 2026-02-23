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

	var receivers []receiver.Metrics
	for _, device := range conf.Devices {
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
		for scraperType, scraperCfg := range conf.Scrapers {
			factory, exists := scraperFactories[scraperType]
			if !exists {
				set.Logger.Warn("Unsupported scraper type",
					zap.String("type", scraperType.String()),
					zap.String("device", device.Name))
				continue
			}

			freshCfg := factory.CreateDefaultConfig()

			switch typedCfg := scraperCfg.(type) {
			case *systemscraper.Config:
				freshSysCfg := freshCfg.(*systemscraper.Config)
				freshSysCfg.MetricsBuilderConfig = typedCfg.MetricsBuilderConfig
				freshSysCfg.Device = connDevice
				freshCfg = freshSysCfg
			case *interfacesscraper.Config:
				freshIntfCfg := freshCfg.(*interfacesscraper.Config)
				freshIntfCfg.MetricsBuilderConfig = typedCfg.MetricsBuilderConfig
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
