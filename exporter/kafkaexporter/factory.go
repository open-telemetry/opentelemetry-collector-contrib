// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
)

const (
	defaultTracesTopic  = "otlp_spans"
	defaultMetricsTopic = "otlp_metrics"
	defaultLogsTopic    = "otlp_logs"
	defaultEncoding     = "otlp_proto"
	// partitioning metrics by resource attributes is disabled by default
	defaultPartitionMetricsByResourceAttributesEnabled = false
	// partitioning logs by resource attributes is disabled by default
	defaultPartitionLogsByResourceAttributesEnabled = false
)

// NewFactory creates Kafka exporter factory.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   exporterhelper.NewDefaultQueueConfig(),
		ClientConfig:    configkafka.NewDefaultClientConfig(),
		Producer:        configkafka.NewDefaultProducerConfig(),
		// using an empty topic to track when it has not been set by user, default is based on traces or metrics.
		Topic:                                "",
		Encoding:                             defaultEncoding,
		PartitionMetricsByResourceAttributes: defaultPartitionMetricsByResourceAttributesEnabled,
		PartitionLogsByResourceAttributes:    defaultPartitionLogsByResourceAttributesEnabled,
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	if oCfg.Topic == "" {
		oCfg.Topic = defaultTracesTopic
	}
	exp := newTracesExporter(oCfg, set)
	return exporterhelper.NewTraces(
		ctx,
		set,
		&oCfg,
		exp.exportData,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable exporterhelper Timeout, because we cannot pass a Context to the Producer,
		// and will rely on the sarama Producer Timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(oCfg.BackOffConfig),
		exporterhelper.WithQueue(oCfg.QueueSettings),
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Close),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	if oCfg.Topic == "" {
		oCfg.Topic = defaultMetricsTopic
	}
	exp := newMetricsExporter(oCfg, set)
	return exporterhelper.NewMetrics(
		ctx,
		set,
		&oCfg,
		exp.exportData,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable exporterhelper Timeout, because we cannot pass a Context to the Producer,
		// and will rely on the sarama Producer Timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(oCfg.BackOffConfig),
		exporterhelper.WithQueue(oCfg.QueueSettings),
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Close),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	if oCfg.Topic == "" {
		oCfg.Topic = defaultLogsTopic
	}
	exp := newLogsExporter(oCfg, set)
	return exporterhelper.NewLogs(
		ctx,
		set,
		&oCfg,
		exp.exportData,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable exporterhelper Timeout, because we cannot pass a Context to the Producer,
		// and will rely on the sarama Producer Timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(oCfg.BackOffConfig),
		exporterhelper.WithQueue(oCfg.QueueSettings),
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Close),
	)
}
