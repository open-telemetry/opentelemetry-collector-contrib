// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter/internal/metadata"
)

// NewFactory creates a factory for Carbon exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: defaultEndpoint,
		},
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		QueueConfig:     exporterhelper.NewDefaultQueueSettings(),
		RetryConfig:     exporterhelper.NewDefaultRetrySettings(),
	}
}

func createMetricsExporter(
	_ context.Context,
	params exporter.CreateSettings,
	config component.Config,
) (exporter.Metrics, error) {
	exp, err := newCarbonExporter(config.(*Config), params)

	if err != nil {
		return nil, err
	}

	return exp, nil
}
