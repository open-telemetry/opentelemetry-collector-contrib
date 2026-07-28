// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()

	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		DNSServers:           []string{},
		Hostnames:            []HostnameConfig{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	config := cfg.(*Config)
	_ = config
	_ = settings
	_ = consumer
	return &noopMetricsReceiver{}, nil
}

// noopMetricsReceiver is a minimal receiver to satisfy component lifecycle tests.
type noopMetricsReceiver struct{}

func (r *noopMetricsReceiver) Start(_ context.Context, _ component.Host) error {
	_ = r
	return nil
}

func (r *noopMetricsReceiver) Shutdown(_ context.Context) error {
	_ = r
	return nil
}
