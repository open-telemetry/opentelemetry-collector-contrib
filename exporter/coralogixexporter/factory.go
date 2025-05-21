// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter/internal/metadata"
)

// NewFactory by Coralogix
func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithTraces(createTraceExporter, metadata.TracesStability),
		xexporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithProfiles(createProfilesExporter, metadata.ProfilesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		QueueSettings:   exporterhelper.NewDefaultQueueConfig(),
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
		DomainSettings: configgrpc.ClientConfig{
			Compression: configcompression.TypeGzip,
		},
		ClientConfig: configgrpc.ClientConfig{
			Endpoint: "https://",
		},
		// Traces GRPC client
		Traces: configgrpc.ClientConfig{
			Endpoint:    "https://",
			Compression: configcompression.TypeGzip,
		},
		Metrics: configgrpc.ClientConfig{
			Endpoint: "https://",
			// Default to gzip compression
			Compression:     configcompression.TypeGzip,
			WriteBufferSize: 512 * 1024,
		},
		Logs: configgrpc.ClientConfig{
			Endpoint:    "https://",
			Compression: configcompression.TypeGzip,
		},
		PrivateKey: "",
		AppName:    "",
		RateLimiter: RateLimiterConfig{
			Enabled:   false,
			Threshold: 10,
			Duration:  time.Minute,
		},
	}
}

func createTraceExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Traces, error) {
	cfg := config.(*Config)

	exporter, err := newTracesExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTraces(
		ctx,
		set,
		config,
		exporter.pushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Metrics, error) {
	cfg := config.(*Config)

	oce, err := newMetricsExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		oce.pushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithShutdown(oce.shutdown),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Logs, error) {
	cfg := config.(*Config)

	oce, err := newLogsExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		oce.pushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithShutdown(oce.shutdown),
	)
}

func createProfilesExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (xexporter.Profiles, error) {
	cfg := config.(*Config)

	oce, err := newProfilesExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	return xexporterhelper.NewProfilesExporter(
		ctx,
		set,
		cfg,
		oce.pushProfiles,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithShutdown(oce.shutdown),
	)
}
