// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

const (
	zapEndpointKey = "endpoint"
)

// NewFactory creates a factory for the exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
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
	}
}

func buildExporterConfig(cfg *Config, endpoint string) otlpexporter.Config {
	oCfg := cfg.Protocol.OTLP
	oCfg.ClientConfig.Endpoint = endpoint

	return oCfg
}

func buildExporterSettings(typ component.Type, params exporter.Settings, endpoint string) exporter.Settings {
	// Override child exporter ID to segregate metrics from loadbalancing top level
	childName := fmt.Sprintf("%s_%s", params.ID, endpoint)
	params.ID = component.NewIDWithName(typ, childName)
	// Add "endpoint" attribute to child exporter logger to segregate logs from loadbalancing top level
	params.Logger = params.Logger.With(zap.String(zapEndpointKey, endpoint))

	return params
}

func buildExporterResilienceOptions(options []exporterhelper.Option, cfg *Config) []exporterhelper.Option {
	if cfg.TimeoutSettings.Timeout > 0 {
		options = append(options, exporterhelper.WithTimeout(cfg.TimeoutSettings))
	}
	if cfg.QueueSettings.Enabled {
		options = append(options, exporterhelper.WithQueue(cfg.QueueSettings))
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

	return exporterhelper.NewTraces(
		ctx,
		params,
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

	return exporterhelper.NewLogs(
		ctx,
		params,
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

	return exporterhelper.NewMetrics(
		ctx,
		params,
		cfg,
		exporter.ConsumeMetrics,
		buildExporterResilienceOptions(options, c)...,
	)
}
