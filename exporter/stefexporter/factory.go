// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter/internal/metadata"
)

// NewFactory creates a factory for STEF exporter
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutConfig: exporterhelper.TimeoutConfig{Timeout: 15 * time.Second},
		QueueConfig:   exporterhelper.NewDefaultQueueConfig(),
		RetryConfig:   configretry.NewDefaultBackOffConfig(),
	}
}

func createMetricsExporter(ctx context.Context, set exporter.Settings, config component.Config) (
	exporter.Metrics, error,
) {
	cfg := config.(*Config)
	stefexporter := newStefExporter(set.TelemetrySettings, cfg)
	return exporterhelper.NewMetrics(
		ctx, set, config,
		stefexporter.exportMetrics,
		exporterhelper.WithStart(stefexporter.Start),
		exporterhelper.WithShutdown(stefexporter.Shutdown),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(cfg.TimeoutConfig),
		exporterhelper.WithQueue(cfg.QueueConfig),
		exporterhelper.WithRetry(cfg.RetryConfig),
	)
}
