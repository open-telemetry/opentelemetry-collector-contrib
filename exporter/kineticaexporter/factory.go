// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter/internal/metadata"
)

// NewFactory creates a factory for Kinetica exporter.
//
//	@return exporter.Factory
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, component.StabilityLevelDevelopment),
		exporter.WithTraces(createTracesExporter, component.StabilityLevelDevelopment),
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelDevelopment),
	)
}

// createDefaultConfig - function to create a default configuration
//
//	@return component.Config
func createDefaultConfig() component.Config {
	return &Config{
		Host:               "http://localhost:9191/",
		Schema:             "",
		Username:           "",
		Password:           "",
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

	exporter := newLogsExporter(set.Logger, cf)

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
	exporter := newTracesExporter(set.Logger, cf)

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
	exporter := newMetricsExporter(set.Logger, cf)
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exporter.pushMetricsData,
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
	)
}
