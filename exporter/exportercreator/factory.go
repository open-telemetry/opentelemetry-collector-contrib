// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

// This file implements factory for exporter_creator. An exporter_creator can create other exporters at runtime.

var exporters = sharedcomponent.NewSharedComponents()

// NewFactory creates a factory for exporter creator.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		exporterTemplates: map[string]exporterTemplate{},
		Routing:           RoutingConfig{Rules: []RoutingRule{}},
		DefaultExporters:  []component.ID{},
	}
}

func createLogsExporter(
	_ context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	e := exporters.GetOrAdd(cfg, func() component.Component {
		return newExporterCreator(params, cfg.(*Config))
	})
	c := e.Unwrap()
	return c.(*exporterCreator), nil
}

func createMetricsExporter(
	_ context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	e := exporters.GetOrAdd(cfg, func() component.Component {
		return newExporterCreator(params, cfg.(*Config))
	})
	c := e.Unwrap()
	return c.(*exporterCreator), nil
}

func createTracesExporter(
	_ context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	e := exporters.GetOrAdd(cfg, func() component.Component {
		return newExporterCreator(params, cfg.(*Config))
	})
	c := e.Unwrap()
	return c.(*exporterCreator), nil
}
