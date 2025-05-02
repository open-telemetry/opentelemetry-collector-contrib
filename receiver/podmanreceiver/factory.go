// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

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
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second
	cfg.Timeout = 5 * time.Second

	return &Config{
		ControllerConfig:     cfg,
		Endpoint:             "unix:///run/podman/podman.sock",
		APIVersion:           defaultAPIVersion,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createDefaultReceiverConfig() component.Config {
	return createDefaultConfig()
}
