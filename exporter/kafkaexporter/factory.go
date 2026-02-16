// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

const (
	defaultLogsTopic        = "otlp_logs"
	defaultLogsEncoding     = "otlp_proto"
	defaultMetricsTopic     = "otlp_metrics"
	defaultMetricsEncoding  = "otlp_proto"
	defaultTracesTopic      = "otlp_spans"
	defaultTracesEncoding   = "otlp_proto"
	defaultProfilesTopic    = "otlp_profiles"
	defaultProfilesEncoding = "otlp_proto"

	// partitioning metrics by resource attributes is disabled by default
	defaultPartitionMetricsByResourceAttributesEnabled = false
	// partitioning logs by resource attributes is disabled by default
	defaultPartitionLogsByResourceAttributesEnabled = false
	// partitioning logs by trace id is disabled by default
	defaultPartitionLogsByTraceIDEnabled = false
)

// logSendingQueueBatchWarnings logs warnings when the user set sending_queue::batch
// but the config may lead to MessageTooLarge errors: (1) batch sizer is not "bytes",
// or (2) batch uses bytes sizer but max_size exceeds producer max_message_bytes minus overhead.
func logSendingQueueBatchWarnings(cfg *Config, logger *zap.Logger) {
	if !cfg.userSetSendingQueueBatch {
		// User did not set sending_queue::batch. No possible conflicting settings
		// to warn about.
		return
	}
	if !cfg.QueueBatchConfig.HasValue() {
		return
	}
	queueConfig := cfg.QueueBatchConfig.GetOrInsertDefault()
	if !queueConfig.Batch.HasValue() {
		return
	}
	batch := queueConfig.Batch.Get()
	recommendedMaxSize := cfg.Producer.MaxMessageBytes - kafkaOverheadBytes

	if batch.Sizer != exporterhelper.RequestSizerTypeBytes {
		logger.Warn(
			"sending_queue::batch was explicitly set; for Kafka, set batch::sizer to \"bytes\" so payloads sent stay within Kafka message size limits.",
		)
	}
	if batch.Sizer == exporterhelper.RequestSizerTypeBytes && batch.MaxSize > 0 && int64(recommendedMaxSize) < batch.MaxSize {
		logger.Warn(
			fmt.Sprintf(
				"sending_queue::batch was explicitly set with bytes sizer; set batch::max_size to at most producer.max_message_bytes minus key/header overhead (%d bytes) to avoid MessageTooLarge errors.",
				recommendedMaxSize,
			),
		)
	}
}

// NewFactory creates Kafka exporter factory.
func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithTraces(createTracesExporter, metadata.TracesStability),
		xexporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithProfiles(createProfilesExporter, metadata.ProfilesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
		QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		ClientConfig:     configkafka.NewDefaultClientConfig(),
		Producer:         configkafka.NewDefaultProducerConfig(),
		Logs: SignalConfig{
			Topic:    defaultLogsTopic,
			Encoding: defaultLogsEncoding,
		},
		Metrics: SignalConfig{
			Topic:    defaultMetricsTopic,
			Encoding: defaultMetricsEncoding,
		},
		Traces: SignalConfig{
			Topic:    defaultTracesTopic,
			Encoding: defaultTracesEncoding,
		},
		Profiles: SignalConfig{
			Topic:    defaultProfilesTopic,
			Encoding: defaultProfilesEncoding,
		},
		PartitionMetricsByResourceAttributes: defaultPartitionMetricsByResourceAttributesEnabled,
		PartitionLogsByResourceAttributes:    defaultPartitionLogsByResourceAttributesEnabled,
		PartitionLogsByTraceID:               defaultPartitionLogsByTraceIDEnabled,
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	logSendingQueueBatchWarnings(&oCfg, set.Logger)
	exp := newTracesExporter(oCfg, set)
	return exporterhelper.NewTraces(
		ctx,
		set,
		&oCfg,
		exp.exportData,
		exporterhelperOptions(
			oCfg,
			xexporterhelper.NewTracesQueueBatchSettings(),
			exp.Start, exp.Close,
		)...,
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	exp := newMetricsExporter(oCfg, set)
	return exporterhelper.NewMetrics(
		ctx,
		set,
		&oCfg,
		exp.exportData,
		exporterhelperOptions(
			oCfg,
			xexporterhelper.NewMetricsQueueBatchSettings(),
			exp.Start, exp.Close,
		)...,
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	exp := newLogsExporter(oCfg, set)
	return exporterhelper.NewLogs(
		ctx,
		set,
		&oCfg,
		exp.exportData,
		exporterhelperOptions(
			oCfg,
			xexporterhelper.NewLogsQueueBatchSettings(),
			exp.Start, exp.Close,
		)...,
	)
}

func createProfilesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (xexporter.Profiles, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	exp := newProfilesExporter(oCfg, set)
	return xexporterhelper.NewProfiles(
		ctx,
		set,
		&oCfg,
		exp.exportData,
		exporterhelperOptions(
			oCfg,
			xexporterhelper.NewProfilesQueueBatchSettings(),
			exp.Start, exp.Close,
		)...,
	)
}

func exporterhelperOptions(
	cfg Config,
	qbs xexporterhelper.QueueBatchSettings,
	startFunc component.StartFunc,
	shutdownFunc component.ShutdownFunc,
) []exporterhelper.Option {
	if len(cfg.IncludeMetadataKeys) > 0 {
		qbs.Partitioner = metadataKeysPartitioner{keys: cfg.IncludeMetadataKeys}
	}
	return []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable exporterhelper Timeout, because we cannot pass a Context to the Producer,
		// and will rely on the Producer Timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		xexporterhelper.WithQueueBatch(cfg.QueueBatchConfig, qbs),
		exporterhelper.WithStart(startFunc),
		exporterhelper.WithShutdown(shutdownFunc),
	}
}
