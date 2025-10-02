// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
)

const (
	defaultCollectionInterval = 60 * time.Second
	defaultTimeout            = 10 * time.Second
)

// NewFactory creates a factory for Cisco OS receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = defaultCollectionInterval
	cfg.Timeout = defaultTimeout

	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Devices:              []DeviceConfig{},
		Scrapers: ScrapersConfig{
			BGP:         true,
			Environment: true,
			Facts:       true,
			Interfaces:  true,
			Optics:      true,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	conf := cfg.(*Config)

	// TODO: Replace with actual scraper implementation in second PR
	// For skeleton, we'll return a placeholder
	_ = conf
	_ = set
	_ = consumer
	return &nopMetricsReceiver{}, nil
}

// nopMetricsReceiver is a minimal receiver to satisfy component lifecycle tests.
type nopMetricsReceiver struct{}

func (r *nopMetricsReceiver) Start(_ context.Context, _ component.Host) error {
	_ = r
	return nil
}

func (r *nopMetricsReceiver) Shutdown(_ context.Context) error {
	_ = r
	return nil
}
