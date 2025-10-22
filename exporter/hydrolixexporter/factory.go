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

    return exporterhelper.NewTracesExporter(
        ctx,
        set,
        cfg,
        te.pushTraces,
        exporterhelper.WithTimeout(config.Timeout),
        exporterhelper.WithRetry(config.BackOffConfig),
        exporterhelper.WithQueue(config.QueueSettings),
    )
}

func createMetricsExporter(
    ctx context.Context,
    set exporter.Settings,
    cfg component.Config,
) (exporter.Metrics, error) {
    config := cfg.(*Config)
    me := newMetricsExporter(config, set)

    return exporterhelper.NewMetricsExporter(
        ctx,
        set,
        cfg,
        me.pushMetrics,
        exporterhelper.WithTimeout(config.Timeout),
        exporterhelper.WithRetry(config.BackOffConfig),
        exporterhelper.WithQueue(config.QueueSettings),
    )
}