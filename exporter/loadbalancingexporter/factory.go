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
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
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
		// Note: OTLP is not set by default to allow OTLP/HTTP configuration
		// The default gRPC config is stored implicitly and used when neither protocol is configured
		Protocol: Protocol{
			OTLP: configoptional.None[otlpexporter.Config](),
		},
		QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}
}

func buildExporterConfig(cfg *Config, endpoint string) *otlpexporter.Config {
	var oCfg *otlpexporter.Config
	if cfg.Protocol.OTLP.HasValue() {
		// Merge configured OTLP with default to ensure all fields have proper defaults
		otlpFactory := otlpexporter.NewFactory()
		otlpDefaultCfg := otlpFactory.CreateDefaultConfig().(*otlpexporter.Config)
		configuredOTLP := cfg.Protocol.OTLP.Get()
		
		// Use the configured values, falling back to defaults for unset fields
		if configuredOTLP.ClientConfig.Endpoint != "" {
			otlpDefaultCfg.ClientConfig.Endpoint = configuredOTLP.ClientConfig.Endpoint
		}
		if configuredOTLP.ClientConfig.Headers != nil {
			otlpDefaultCfg.ClientConfig.Headers = configuredOTLP.ClientConfig.Headers
		}
		if configuredOTLP.TimeoutConfig.Timeout > 0 {
			otlpDefaultCfg.TimeoutConfig.Timeout = configuredOTLP.TimeoutConfig.Timeout
		}
		if configuredOTLP.RetryConfig.Enabled {
			otlpDefaultCfg.RetryConfig = configuredOTLP.RetryConfig
		}
		if configuredOTLP.QueueConfig.HasValue() {
			otlpDefaultCfg.QueueConfig = configuredOTLP.QueueConfig
		}
		
		oCfg = otlpDefaultCfg
	} else {
		// Use default gRPC config when neither protocol is explicitly configured
		otlpFactory := otlpexporter.NewFactory()
		otlpDefaultCfg := otlpFactory.CreateDefaultConfig().(*otlpexporter.Config)
		oCfg = otlpDefaultCfg
	}
	oCfg.ClientConfig.Endpoint = endpoint

	return oCfg
}

func buildHTTPExporterConfig(cfg *Config, resolvedBackend string) (*otlphttpexporter.Config, error) {
	httpCfg := cfg.Protocol.OTLPHTTP.Get()

	// Set default encoding to proto if not explicitly configured
	if httpCfg.Encoding == "" {
		httpCfg.Encoding = otlphttpexporter.EncodingProto
	}

	// For HTTP, we need to rewrite the endpoint to preserve the configured path
	// while replacing the host and port with the resolved backend
	var rewrittenEndpoint string
	var err error

	// Prefer signal-specific endpoints if configured
	switch {
	case httpCfg.MetricsEndpoint != "":
		rewrittenEndpoint, err = rewriteHTTPEndpoint(httpCfg.MetricsEndpoint, resolvedBackend)
		if err != nil {
			return nil, fmt.Errorf("failed to rewrite metrics endpoint: %w", err)
		}
		httpCfg.MetricsEndpoint = rewrittenEndpoint
	case httpCfg.ClientConfig.Endpoint != "":
		// Fall back to the generic endpoint with /v1/metrics appended
		rewrittenEndpoint, err = rewriteHTTPEndpoint(httpCfg.ClientConfig.Endpoint, resolvedBackend)
		if err != nil {
			return nil, fmt.Errorf("failed to rewrite endpoint: %w", err)
		}
		httpCfg.ClientConfig.Endpoint = rewrittenEndpoint
	default:
		// Use the default with resolved backend
		httpCfg.ClientConfig.Endpoint = fmt.Sprintf("http://%s/v1/metrics", resolvedBackend)
	}

	return httpCfg, nil
}

func buildExporterSettings(typ component.Type, params exporter.Settings, endpoint string) exporter.Settings {
	exporterParams := params
	if name := params.ID.Name(); name != "" {
		exporterParams.ID = component.NewIDWithName(typ, name)
	} else {
		exporterParams.ID = component.NewID(typ)
	}
	telemetry := params.TelemetrySettings
	telemetry.MeterProvider = metricnoop.NewMeterProvider()
	telemetry.TracerProvider = tracenoop.NewTracerProvider()
	exporterParams.Logger = params.Logger.With(zap.String(zapEndpointKey, endpoint))
	telemetry.Logger = exporterParams.Logger
	exporterParams.TelemetrySettings = telemetry
	return exporterParams
}

func buildExporterResilienceOptions(options []exporterhelper.Option, cfg *Config) []exporterhelper.Option {
	if cfg.TimeoutSettings.Timeout > 0 {
		options = append(options, exporterhelper.WithTimeout(cfg.TimeoutSettings))
	}
	if cfg.QueueSettings.HasValue() {
		options = append(options, exporterhelper.WithQueue(cfg.QueueSettings))
	}
	if cfg.BackOffConfig.Enabled {
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
