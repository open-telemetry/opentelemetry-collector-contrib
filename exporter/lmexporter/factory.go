package lmexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const typeStr = "lmexporter"

// NewFactory creates a LogicMonitor exporter factory
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithLogsExporter(createLogsExporter),
		component.WithTracesExporter(createTracesExporter),
	)
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
	}
}

func createTracesExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	c config.Exporter,
) (component.TracesExporter, error) {
	cfg := c.(*Config)
	exp, err := newTracesExporter(cfg, set.Logger, set.BuildInfo)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		cfg,
		set,
		exp.pushTraces,
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings))
}

func createLogsExporter(_ context.Context, set component.ExporterCreateSettings, cfg config.Exporter) (component.LogsExporter, error) {
	exp, err := newLogsExporter(cfg, set.Logger)
	if err != nil {
		return nil, err
	}
	c := cfg.(*Config)

	return exporterhelper.NewLogsExporter(
		cfg,
		set,
		exp.PushLogData,
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.RetrySettings),
	)
}
