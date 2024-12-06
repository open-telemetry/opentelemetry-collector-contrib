// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

// NewFactory creates a factory for tcpcheckreceiver receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

//func createDefaultConfig() component.Config {
//	cfg := scraperhelper.NewDefaultControllerConfig()
//	// ??
//	cfg.CollectionInterval = 10 * time.Second
//
//	return &Config{
//		ControllerConfig: cfg,
//		// do we need to add timeout?
//		TCPClientSettings: configtcp.TCPClientSettings{
//			Timeout: 10 * time.Second,
//		},
//		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
//	}
//}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()

	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets:              []*confignet.TCPAddrConfig{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	tlsCheckConfig, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigTCPCheck
	}

	mp := newScraper(tlsCheckConfig, settings, getConnectionState)
	s, err := scraperhelper.NewScraperWithoutType(mp.scrape)
	if err != nil {
		return nil, err
	}
	opt := scraperhelper.AddScraperWithType(metadata.Type, s)

	return scraperhelper.NewScraperControllerReceiver(
		&tlsCheckConfig.ControllerConfig,
		settings,
		consumer,
		opt,
	)
}
