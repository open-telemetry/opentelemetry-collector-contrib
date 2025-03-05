// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	collectorscraper "go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

// NewFactory creates a factory for tcpcheckreceiver receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		newDefaultConfig,
		receiver.WithMetrics(newReceiver, metadata.MetricsStability))
}

func newDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()

	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets:              []*confignet.TCPAddrConfig{},
	}
}

func newReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	tcpCheckConfig, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigTCPCheck
	}

	mp := newScraper(tcpCheckConfig, settings)
	s, err := collectorscraper.NewMetrics(mp.scrape)
	if err != nil {
		return nil, err
	}
	opt := scraperhelper.AddScraper(metadata.Type, s)

	return scraperhelper.NewMetricsController(
		&tcpCheckConfig.ControllerConfig,
		settings,
		consumer,
		opt,
	)
}
