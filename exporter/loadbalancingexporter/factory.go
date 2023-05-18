// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
)

const (
	// The value of "type" key in configuration.
	typeStr = "loadbalancing"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for the exporter.
func NewFactory() exporter.Factory {
	_ = view.Register(MetricViews()...)

	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
		exporter.WithLogs(createLogsExporter, stability),
	)
}

func createDefaultConfig() component.Config {
	otlpFactory := otlpexporter.NewFactory()
	otlpDefaultCfg := otlpFactory.CreateDefaultConfig().(*otlpexporter.Config)

	return &Config{
		Protocol: Protocol{
			OTLP: *otlpDefaultCfg,
		},
	}
}

func createTracesExporter(_ context.Context, params exporter.CreateSettings, cfg component.Config) (exporter.Traces, error) {
	return newTracesExporter(params, cfg)
}

func createLogsExporter(_ context.Context, params exporter.CreateSettings, cfg component.Config) (exporter.Logs, error) {
	return newLogsExporter(params, cfg)
}
