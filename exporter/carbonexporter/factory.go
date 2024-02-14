// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter/internal/metadata"
)

// Defaults for not specified configuration settings.
const (
	defaultEndpoint = "localhost:2003"
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
		MaxIdleConns:    100,
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		QueueConfig:     exporterhelper.NewDefaultQueueSettings(),
		RetryConfig:     configretry.NewDefaultBackOffConfig(),
	}
}

func createMetricsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	config component.Config,
) (exporter.Metrics, error) {
	exp, err := newCarbonExporter(ctx, config.(*Config), params)

	if err != nil {
		return nil, err
	}

	return exp, nil
}
