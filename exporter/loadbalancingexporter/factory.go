// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/xexporter"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

const (
	zapEndpointKey = "endpoint"
)

// NewFactory creates a factory for the exporter.
func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithTraces(createTracesExporter, metadata.TracesStability),
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		xexporter.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createDefaultConfig() component.Config {
	otlpFactory := otlpexporter.NewFactory()
	otlpDefaultCfg := otlpFactory.CreateDefaultConfig().(*otlpexporter.Config)
	otlpDefaultCfg.ClientConfig.Endpoint = "placeholder:4317"

	return &Config{
		// By default we disable resilience options on loadbalancing exporter level
		// to maintain compatibility with workflow in previous versions
		Protocol: Protocol{
			OTLP: *otlpDefaultCfg,
		},
		QueueSettings: configoptional.Default(exporterhelper.NewDefaultQueueConfig()),
	}
}

func buildExporterConfig(cfg *Config, endpoint string) otlpexporter.Config {
	oCfg := cfg.Protocol.OTLP
	oCfg.ClientConfig.Endpoint = endpoint

	return oCfg
}

func buildExporterSettings(typ component.Type, params exporter.Settings, endpoint string) exporter.Settings {
	if name := params.ID.Name(); name != "" {
		params.ID = component.NewIDWithName(typ, name)
	} else {
		params.ID = component.NewID(typ)
	}
	telemetry := params.TelemetrySettings
	params.Logger = params.Logger.With(zap.String(zapEndpointKey, endpoint))
	telemetry.Logger = params.Logger
	params.TelemetrySettings = telemetry
	return params
}

func buildTopLevelExporterSettings(params exporter.Settings) exporter.Settings {
	noopMeterProvider := metricnoop.NewMeterProvider()
	noopTracerProvider := tracenoop.NewTracerProvider()

	telemetry := params.TelemetrySettings
	telemetry.MeterProvider = noopMeterProvider
	telemetry.TracerProvider = noopTracerProvider
	params.TelemetrySettings = telemetry
	params.MeterProvider = noopMeterProvider
	params.TracerProvider = noopTracerProvider
	return params
}

func buildExporterQueueSettings(cfg *Config) configoptional.Optional[exporterhelper.QueueBatchConfig] {
	if !cfg.QueueSettings.HasValue() {
		return configoptional.None[exporterhelper.QueueBatchConfig]()
	}

	queueSettings := *cfg.QueueSettings.Get()
	if cfg.Enabled && queueSettings.StorageID == nil {
		queueSettings.WaitForResult = true
	}

	return configoptional.Some(queueSettings)
}

func buildExporterResilienceOptions(options []exporterhelper.Option, cfg *Config) []exporterhelper.Option {
	if cfg.TimeoutSettings.Timeout > 0 {
		options = append(options, exporterhelper.WithTimeout(cfg.TimeoutSettings))
	}
	if queueSettings := buildExporterQueueSettings(cfg); queueSettings.HasValue() {
		options = append(options, exporterhelper.WithQueue(queueSettings))
	}
	if cfg.Enabled {
		options = append(options, exporterhelper.WithRetry(cfg.BackOffConfig))
	}

	return options
}

func createTracesExporter(ctx context.Context, params exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	c := cfg.(*Config)
	exp, err := newTracesExporter(params, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot configure loadbalancing traces exporter: %w", err)
	}

	options := []exporterhelper.Option{
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown),
		exporterhelper.WithCapabilities(exp.Capabilities()),
	}
	helperParams := buildTopLevelExporterSettings(params)

	return exporterhelper.NewTraces(
		ctx,
		helperParams,
		cfg,
		exp.ConsumeTraces,
		buildExporterResilienceOptions(options, c)...,
	)
}

func createLogsExporter(ctx context.Context, params exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	c := cfg.(*Config)
	exporter, err := newLogsExporter(params, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot configure loadbalancing logs exporter: %w", err)
	}

	options := []exporterhelper.Option{
		exporterhelper.WithStart(exporter.Start),
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithCapabilities(exporter.Capabilities()),
	}
	helperParams := buildTopLevelExporterSettings(params)

	return exporterhelper.NewLogs(
		ctx,
		helperParams,
		cfg,
		exporter.ConsumeLogs,
		buildExporterResilienceOptions(options, c)...,
	)
}

func createMetricsExporter(ctx context.Context, params exporter.Settings, cfg component.Config) (exporter.Metrics, error) {
	c := cfg.(*Config)
	exporter, err := newMetricsExporter(params, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot configure loadbalancing metrics exporter: %w", err)
	}

	options := []exporterhelper.Option{
		exporterhelper.WithStart(exporter.Start),
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithCapabilities(exporter.Capabilities()),
	}
	helperParams := buildTopLevelExporterSettings(params)

	return exporterhelper.NewMetrics(
		ctx,
		helperParams,
		cfg,
		exporter.ConsumeMetrics,
		buildExporterResilienceOptions(options, c)...,
	)
}
