// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

var _ component.Config = (*Config)(nil)

// Config defines configuration for Kafka exporter.
type Config struct {
	TimeoutSettings           exporterhelper.TimeoutConfig    `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	configkafka.ClientConfig  `mapstructure:",squash"`
	Producer                  configkafka.ProducerConfig `mapstructure:"producer"`

	// Logs holds configuration about how logs should be sent to Kafka.
	Logs SignalConfig `mapstructure:"logs"`

	// Metrics holds configuration about how metrics should be sent to Kafka.
	Metrics SignalConfig `mapstructure:"metrics"`

	// Traces holds configuration about how traces should be sent to Kafka.
	Traces SignalConfig `mapstructure:"traces"`

	// Topic holds the name of the Kafka topic to which data should be exported.
	//
	// Topic has no default. If explicitly specified, it will take precedence over
	// the default values of logs::topic, metrics::topic, and traces::topic.
	//
	// Deprecated [v0.124.0]: use logs::topic, metrics::topic, and traces::topic instead.
	Topic string `mapstructure:"topic"`

	// IncludeMetadataKeys indicates the receiver's client metadata keys to propagate as Kafka message headers.
	IncludeMetadataKeys []string `mapstructure:"include_metadata_keys"`

	// TopicFromAttribute is the name of the attribute to use as the topic name.
	TopicFromAttribute string `mapstructure:"topic_from_attribute"`

	// Encoding holds the encoding of Kafka message values.
	//
	// Encoding has no default. If explicitly specified, it will take precedence over
	// the default values of logs::encoding, metrics::encoding, and traces::encoding.
	//
	// Deprecated [v0.124.0]: use logs::encoding, metrics::encoding, and traces::encoding instead.
	Encoding string `mapstructure:"encoding"`

	// PartitionTracesByID sets the message key of outgoing trace messages to the trace ID.
	//
	// NOTE: this does not have any effect for Jaeger encodings. Jaeger encodings always use
	// use the trace ID for the message key.
	PartitionTracesByID bool `mapstructure:"partition_traces_by_id"`

	// PartitionMetricsByResourceAttributes controls the partitioning of metrics messages by
	// resource. If this is true, then the message key will be set to a hash of the resource's
	// identifying attributes.
	PartitionMetricsByResourceAttributes bool `mapstructure:"partition_metrics_by_resource_attributes"`

	// PartitionLogsByResourceAttributes controls the partitioning of logs messages by resource.
	// If this is true, then the message key will be set to a hash of the resource's identifying
	// attributes.
	PartitionLogsByResourceAttributes bool `mapstructure:"partition_logs_by_resource_attributes"`
}

func (c *Config) Unmarshal(conf *confmap.Conf) error {
	if err := conf.Unmarshal(c); err != nil {
		return err
	}
	// Check if deprecated fields have been explicitly set,
	// in which case they should be used instead of signal-
	// specific defaults.
	var zeroConfig Config
	if err := conf.Unmarshal(&zeroConfig); err != nil {
		return err
	}
	if c.Topic != "" {
		if zeroConfig.Logs.Topic == "" {
			c.Logs.Topic = c.Topic
		}
		if zeroConfig.Metrics.Topic == "" {
			c.Metrics.Topic = c.Topic
		}
		if zeroConfig.Traces.Topic == "" {
			c.Traces.Topic = c.Topic
		}
	}
	if c.Encoding != "" {
		if zeroConfig.Logs.Encoding == "" {
			c.Logs.Encoding = c.Encoding
		}
		if zeroConfig.Metrics.Encoding == "" {
			c.Metrics.Encoding = c.Encoding
		}
		if zeroConfig.Traces.Encoding == "" {
			c.Traces.Encoding = c.Encoding
		}
	}
	return conf.Unmarshal(c)
}

// SignalConfig holds signal-specific configuration for the Kafka exporter.
type SignalConfig struct {
	// Topic holds the name of the Kafka topic to which messages of the
	// signal type should be produced.
	//
	// The default depends on the signal type:
	//  - "otlp_spans" for traces
	//  - "otlp_metrics" for metrics
	//  - "otlp_logs" for logs
	Topic string `mapstructure:"topic"`

	// TopicFromMetadataKey holds the name of the metadata key to use as the
	// topic name for this signal type. If this is set, it takes precedence
	// over the topic name set in the topic field.
	TopicFromMetadataKey string `mapstructure:"topic_from_metadata_key"`

	// Encoding holds the encoding of messages for the signal type.
	//
	// Defaults to "otlp_proto".
	Encoding string `mapstructure:"encoding"`
}
