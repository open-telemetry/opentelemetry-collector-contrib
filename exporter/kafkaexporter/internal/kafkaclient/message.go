// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"

// Messages is a collection of messages (with count) to be sent to Kafka.
type Messages struct {
	// Count is the total number of messages across all topics.
	// Populating this field allows the downstream method to preallocate
	// the slice of messages, which can improve performance.
	Count         int
	TopicMessages []TopicMessages
}

// TopicMessages represents a collection of messages for a specific topic.
type TopicMessages struct {
	Topic    string
	Messages []marshaler.Message
}
