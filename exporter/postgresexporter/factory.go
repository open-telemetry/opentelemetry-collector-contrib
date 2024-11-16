package postgresexporter

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/postgresexporter/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
		QueueSettings:    exporterhelper.NewDefaultQueueConfig(),
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
		Username:         "postgres",
		Password:         "postgres",
		Database:         "postgres",
		Port:             5432,
		Host:             "localhost",
		LogsTableName:    "otel_logs",
		TracesTableName:  "otel_traces",
		MetricsTableName: "otel_metrics",
	}
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config) (exporter.Logs, error) {

	set.Logger.Info("createLogsExporter called")

	cfg := config.(*Config)
	s, err := newLogsExporter(set.Logger, cfg)
	if err != nil {
		set.Logger.Error("Failed to create new logs exporter", zap.Error(err))
		return nil, err
	}

	set.Logger.Info("newLogsExporter created successfully")

	exporter, err := exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		s.pushLogs,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithStart(s.start),
		exporterhelper.WithShutdown(s.shutdown),
		exporterhelper.WithTimeout(s.cfg.TimeoutSettings),
	)

	if err != nil {
		set.Logger.Error("Failed to create logs exporter", zap.Error(err))
		return nil, err
	}

	set.Logger.Info("Logs exporter created successfully")
	return exporter, nil
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config) (exporter.Traces, error) {

	set.Logger.Info("createTracesExporter called")

	cfg := config.(*Config)
	s, err := newTracesExporter(set.Logger, cfg)
	if err != nil {
		set.Logger.Error("Failed to create new traces exporter", zap.Error(err))
		return nil, err
	}

	set.Logger.Info("newTracesExporter created successfully")

	exporter, err := exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		s.pushTraceData,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithStart(s.start),
		exporterhelper.WithShutdown(s.shutdown),
		exporterhelper.WithTimeout(s.cfg.TimeoutSettings),
	)

	if err != nil {
		set.Logger.Error("Failed to create traces exporter", zap.Error(err))
		return nil, err
	}

	set.Logger.Info("Traces exporter created successfully")
	return exporter, nil
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config) (exporter.Metrics, error) {

	set.Logger.Info("createMetricExporters called")

	cfg := config.(*Config)
	s, err := newMetricsExporter(set.Logger, cfg)
	if err != nil {
		set.Logger.Error("Failed to create new metrics exporter", zap.Error(err))
		return nil, err
	}

	set.Logger.Info("newMetricExporter created successfully")

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		s.pushMetricData,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithStart(s.start),
		exporterhelper.WithShutdown(s.shutdown),
		exporterhelper.WithTimeout(s.cfg.TimeoutSettings),
	)

	if err != nil {
		set.Logger.Error("Failed to create metrics exporter", zap.Error(err))
		return nil, err
	}

	set.Logger.Info("Metrics exporter created successfully")
	return exporter, nil
}
