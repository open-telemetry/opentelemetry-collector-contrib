// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter/internal/metadata"
)

// create BMC Helix Exporter factory
func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		xexporter.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

// creates the default configuration for the BMC Helix exporter
func createDefaultConfig() component.Config {
	httpClientConfig := confighttp.NewDefaultClientConfig()
	httpClientConfig.Timeout = 10 * time.Second

	return &Config{
		ClientConfig:               httpClientConfig,
		RetryConfig:                configretry.NewDefaultBackOffConfig(),
		EnrichMetricWithAttributes: true,
	}
}

// creates an exporter.Metrics that records observability metrics for BMC Helix
func createMetricsExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Metrics, error) {
	config := cfg.(*Config)
	exporter, err := newMetricsExporter(config, set)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetrics(
		ctx,
		set,
		config,
		exporter.pushMetrics,
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(config.RetryConfig),
		exporterhelper.WithStart(exporter.start),
	)
}
