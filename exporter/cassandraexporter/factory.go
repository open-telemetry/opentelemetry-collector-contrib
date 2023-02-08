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
	return exporter.NewFactory(typeStr, createDefaultConfig, exporter.WithTraces(createTracesExporter, stability), exporter.WithLogs(createLogsExporter, stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		DSN:        "127.0.0.1",
		Keyspace:   "otel",
		TraceTable: "otel_spans",
		LogsTable:  "otel_logs",
		Replication: Replication{
			Class:             "SimpleStrategy",
			ReplicationFactor: 1,
		},
		Compression: Compression{
			Algorithm: "LZ4Compressor",
		},
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

func createLogsExporter(ctx context.Context, set exporter.CreateSettings, cfg component.Config) (exporter.Logs, error) {
	c := cfg.(*Config)
	exporter, err := newLogsExporter(set.Logger, c)

	if err != nil {
		return nil, fmt.Errorf("cannot configure cassandra traces exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(ctx, set, cfg, exporter.pushLogsData, exporterhelper.WithShutdown(exporter.Shutdown))
}
