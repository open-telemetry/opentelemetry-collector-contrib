package hydrolixexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr = "hydrolix"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, component.StabilityLevelBeta),
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelBeta),
		exporter.WithLogs(createLogsExporter, component.StabilityLevelBeta),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig: confighttp.NewDefaultClientConfig(),
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	config := cfg.(*Config)
	te := newTracesExporter(config, set)

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		te.pushTraces,
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	config := cfg.(*Config)
	me := newMetricsExporter(config, set)

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		me.pushMetrics,
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	config := cfg.(*Config)
	le := newLogsExporter(config, set)

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		le.pushLogs,
	)
}
