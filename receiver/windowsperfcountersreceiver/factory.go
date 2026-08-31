// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsperfcountersreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver/internal/metadata"
)

// This file implements Factory for WindowsPerfCounters receiver.

// NewFactory creates a new factory for windows perf counters receiver.
func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

// createDefaultConfig creates the default configuration for receiver.
func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
	}
}
