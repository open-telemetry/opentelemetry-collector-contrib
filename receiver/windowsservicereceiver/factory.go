// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func createDefaultConfig() component.Config {
	scfg := scraperhelper.NewDefaultControllerConfig()

	return &Config{
		ControllerConfig: scfg,
	}
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(metadata.Type, createDefaultConfig, receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}
