// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver/internal/metadata"
)

const (
	defaultAPIVersion = "3.3.1"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultReceiverConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() *Config {
	cfg := scraperhelper.NewDefaultScraperControllerSettings(metadata.Type)
	cfg.CollectionInterval = 10 * time.Second
	cfg.Timeout = 5 * time.Second

	return &Config{
		ScraperControllerSettings: cfg,
		Endpoint:                  "unix:///run/podman/podman.sock",
		APIVersion:                defaultAPIVersion,
	}
}

func createDefaultReceiverConfig() component.Config {
	return createDefaultConfig()
}

func createMetricsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	podmanConfig := config.(*Config)
	return newMetricsReceiver(ctx, params, podmanConfig, consumer, nil)
}
