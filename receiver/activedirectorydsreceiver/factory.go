// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectorydsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
	}
}
