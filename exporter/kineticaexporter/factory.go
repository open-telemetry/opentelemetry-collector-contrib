package kineticaotelexporter

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// NewFactory creates a factory for Kinetica exporter.
//
//	@return exporter.Factory
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		CreateDefaultConfig,
		exporter.WithLogs(createLogsExporter, component.StabilityLevelDevelopment),
		exporter.WithTraces(createTracesExporter, component.StabilityLevelDevelopment),
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelAlpha),
	)
}

// CreateDefaultConfig - function to create a default configuration
//
//	@return component.Config
func CreateDefaultConfig() component.Config {
	return &Config{
		Host:               "http://localhost:9191/",
		Schema:             "",
		Username:           "admin",
		Password:           "password",
		BypassSslCertCheck: true,
	}
}

// createLogsExporter creates a new exporter for logs.
//
//	@param ctx
//	@param set
//	@param cfg
//	@return exporter.Logs
//	@return error
func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	cf := cfg.(*Config)

	exporter, err := newLogsExporter(set.Logger, cf)
	if err != nil {
		return nil, fmt.Errorf("Cannot configure Kinetica logs exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exporter.pushLogsData,
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
	)
}

// createTracesExporter - creates a new exporter for traces
//
//	@param ctx
//	@param set
//	@param cfg
//	@return exporter.Traces
//	@return error
func createTracesExporter(ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config) (exporter.Traces, error) {

	cf := cfg.(*Config)
	exporter, err := newTracesExporter(set.Logger, cf)
	if err != nil {
		return nil, fmt.Errorf("Cannot configure Kinetica traces exporter: %w", err)
	}
	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		exporter.pushTraceData,
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
	)
}

// createMetricsExporter - creates a new exporter for metrics
//
//	@param ctx
//	@param set
//	@param cfg
//	@return exporter.Metrics
//	@return error
func createMetricsExporter(ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config) (exporter.Metrics, error) {

	cf := cfg.(*Config)
	exporter, err := newMetricsExporter(set.Logger, cf)
	if err != nil {
		return nil, fmt.Errorf("Cannot configure Kinetica metrics exporter: %w", err)
	}
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exporter.pushMetricsData,
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
	)
}
