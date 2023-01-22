package cassandraexporter

import (
	"context"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr   = "cassandra"
	stability = component.StabilityLevelDevelopment
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(typeStr, createDefaultConfig, exporter.WithTraces(createTracesExporter, stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		DSN:        "127.0.0.1",
		Keyspace:   "otel",
		TraceTable: "otel_spans",
	}
}

func createTracesExporter(ctx context.Context, set exporter.CreateSettings, cfg component.Config) (exporter.Traces, error) {
	c := cfg.(*Config)
	exporter, err := newTracesExporter(set.Logger, c)

	if err != nil {
		return nil, fmt.Errorf("cannot configure cassandra traces exporter: %w", err)
	}

	return exporterhelper.NewTracesExporter(ctx, set, cfg, exporter.pushTraceData, exporterhelper.WithShutdown(exporter.Shutdown))
}
