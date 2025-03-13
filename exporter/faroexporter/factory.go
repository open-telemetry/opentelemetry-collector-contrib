// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter/internal/metadata"
)

// NewFactory creates a factory for Faro exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTraces, metadata.TracesStability),
		exporter.WithLogs(createLogs, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = 30 * time.Second
	clientConfig.Compression = configcompression.TypeGzip
	clientConfig.WriteBufferSize = 512 * 1024

	return &Config{
		RetryConfig:  configretry.NewDefaultBackOffConfig(),
		QueueConfig:  exporterhelper.NewDefaultQueueConfig(),
		ClientConfig: clientConfig,
	}
}

func createTraces(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	oce, err := newExporter(cfg, set)
	if err != nil {
		return nil, err
	}
	oCfg := cfg.(*Config)

	return exporterhelper.NewTraces(ctx, set, cfg,
		oce.ConsumeTraces,
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithCapabilities(oce.Capabilities()),
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: oCfg.Timeout}),
		exporterhelper.WithRetry(oCfg.RetryConfig),
		exporterhelper.WithQueue(oCfg.QueueConfig),
	)
}

func createLogs(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	oce, err := newExporter(cfg, set)
	if err != nil {
		return nil, err
	}
	oCfg := cfg.(*Config)

	return exporterhelper.NewLogs(ctx, set, cfg,
		oce.ConsumeLogs,
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithCapabilities(oce.Capabilities()),
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: oCfg.Timeout}),
		exporterhelper.WithRetry(oCfg.RetryConfig),
		exporterhelper.WithQueue(oCfg.QueueConfig),
	)
}
