// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

var _ component.Config = (*Config)(nil)

var errLogsPartitionExclusive = errors.New(
	"partition_logs_by_resource_attributes and partition_logs_by_trace_id cannot both be enabled",
)

var (
	errTopicMetadataKeyNotIncluded        = errors.New("topic_from_metadata_key must be present in sending_queue::batch::partition::metadata_keys if batching is enabled")
	errBatchPartitionMetadataKeysRequired = errors.New("sending_queue::batch::partition::metadata_keys must be configured when include_metadata_keys is set and batching is enabled")
	errIncludeMetadataKeysNotPartitioned  = errors.New("sending_queue::batch::partition::metadata_keys must include all include_metadata_keys values")
)

// Config defines configuration for Kafka exporter.
type Config struct {
	TimeoutSettings           exporterhelper.TimeoutConfig                             `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	QueueBatchConfig          configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	configkafka.ClientConfig  `mapstructure:",squash"`
	Producer                  configkafka.ProducerConfig `mapstructure:"producer"`

	// Logs holds configuration about how logs should be sent to Kafka.
	Logs SignalConfig `mapstructure:"logs"`

	// Metrics holds configuration about how metrics should be sent to Kafka.
	Metrics SignalConfig `mapstructure:"metrics"`

	// Traces holds configuration about how traces should be sent to Kafka.
	Traces SignalConfig `mapstructure:"traces"`

	// Profiles holds configuration about how profiles should be sent to Kafka.
	Profiles SignalConfig `mapstructure:"profiles"`

	// IncludeMetadataKeys indicates the receiver's client metadata keys to propagate as Kafka message headers.
	IncludeMetadataKeys []string `mapstructure:"include_metadata_keys"`

	// TopicFromAttribute is the name of the attribute to use as the topic name.
	TopicFromAttribute string `mapstructure:"topic_from_attribute"`

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

	// PartitionLogsByTraceID controls partitioning of log messages by trace ID only.
	// When enabled, the exporter splits incoming logs per TraceID (using SplitLogs)
	// and sets the Kafka message key to the 16-byte hex string of that TraceID.
	// If a LogRecord has an empty TraceID, the key may be empty and partition
	// selection falls back to the Kafka client’s default strategy. Resource
	// attributes are not used for the key when this option is enabled.
	PartitionLogsByTraceID bool `mapstructure:"partition_logs_by_trace_id"`
}

func (c *Config) Validate() error {
	if c.PartitionLogsByResourceAttributes && c.PartitionLogsByTraceID {
		return errLogsPartitionExclusive
	}
	if err := validateBatchPartitionerKeys(c); err != nil {
		return err
	}
	return nil
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
	//  - "otlp_profiles" for profiles
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

// validateBatchPartitionerKeys validates the partition keys if sending_queue::batch is enabled.
// The exporter relies on a few client metadata keys to be present, if configured, in the final
// batch that needs to be exported, however, since batching removes all client metadata keys by
// default we need to ensure proper partitioning is configured to keep the required metadata.
func validateBatchPartitionerKeys(c *Config) error {
	if !isBatchingEnabled(c.QueueBatchConfig) {
		return nil
	}

	partitionMetadataKeys := c.QueueBatchConfig.Get().Batch.Get().Partition.MetadataKeys
	partitionMetadataKeySet := make(map[string]struct{}, len(partitionMetadataKeys))
	for _, key := range partitionMetadataKeys {
		partitionMetadataKeySet[key] = struct{}{}
	}

	// Validate if include_metadata_keys are included in partition keys
	if len(c.IncludeMetadataKeys) != 0 {
		if len(partitionMetadataKeys) == 0 {
			return errBatchPartitionMetadataKeysRequired
		}
		for _, includeKey := range c.IncludeMetadataKeys {
			if _, ok := partitionMetadataKeySet[includeKey]; !ok {
				return fmt.Errorf("%w: missing %q from sending_queue::batch::partition::metadata_keys=%v",
					errIncludeMetadataKeysNotPartitioned,
					includeKey,
					partitionMetadataKeys,
				)
			}
		}
	}

	// Validate if topic_from_metadata_key is included in partition_keys
	if err := validateTopicFromMetadataKey(c.Logs.TopicFromMetadataKey, partitionMetadataKeySet); err != nil {
		return fmt.Errorf("logs::topic_from_metadata_key: %w", err)
	}
	if err := validateTopicFromMetadataKey(c.Metrics.TopicFromMetadataKey, partitionMetadataKeySet); err != nil {
		return fmt.Errorf("metrics::topic_from_metadata_key: %w", err)
	}
	if err := validateTopicFromMetadataKey(c.Traces.TopicFromMetadataKey, partitionMetadataKeySet); err != nil {
		return fmt.Errorf("traces::topic_from_metadata_key: %w", err)
	}
	if err := validateTopicFromMetadataKey(c.Profiles.TopicFromMetadataKey, partitionMetadataKeySet); err != nil {
		return fmt.Errorf("profiles::topic_from_metadata_key: %w", err)
	}

	return nil
}

func isBatchingEnabled(queueBatchConfig configoptional.Optional[exporterhelper.QueueBatchConfig]) bool {
	if !queueBatchConfig.HasValue() {
		return false
	}

	return queueBatchConfig.Get().Batch.HasValue()
}

func validateTopicFromMetadataKey(topicFromMetadataKey string, partitionKeysSet map[string]struct{}) error {
	if topicFromMetadataKey == "" {
		return nil
	}
	if _, ok := partitionKeysSet[topicFromMetadataKey]; !ok {
		return fmt.Errorf("%w: %q not found in partition keys=%v",
			errTopicMetadataKeyNotIncluded,
			topicFromMetadataKey,
			slices.Collect(maps.Keys(partitionKeysSet)),
		)
	}
	return nil
}
