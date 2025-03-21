// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
)

var _ component.Config = (*Config)(nil)

// Config defines configuration for Kafka exporter.
type Config struct {
	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	QueueSettings             exporterhelper.QueueConfig   `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	configkafka.ClientConfig  `mapstructure:",squash"`
	Producer                  configkafka.ProducerConfig `mapstructure:"producer"`

	// The name of the kafka topic to export to (default otlp_spans for traces, otlp_metrics for metrics)
	Topic string `mapstructure:"topic"`

	// TopicFromAttribute is the name of the attribute to use as the topic name.
	TopicFromAttribute string `mapstructure:"topic_from_attribute"`

	// Encoding of messages (default "otlp_proto")
	Encoding string `mapstructure:"encoding"`

	// PartitionTracesByID sets the message key of outgoing trace messages to the trace ID.
	// Please note: does not have any effect on Jaeger encoding exporters since Jaeger exporters include
	// trace ID as the message key by default.
	PartitionTracesByID bool `mapstructure:"partition_traces_by_id"`

	PartitionMetricsByResourceAttributes bool `mapstructure:"partition_metrics_by_resource_attributes"`

	PartitionLogsByResourceAttributes bool `mapstructure:"partition_logs_by_resource_attributes"`
}
