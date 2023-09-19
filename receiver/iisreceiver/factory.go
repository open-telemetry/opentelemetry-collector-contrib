// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

// NewFactory creates a factory for iis receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
	}
}
