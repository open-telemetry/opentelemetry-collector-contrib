// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

// Message is a single Kafka message with its destination topic, key, and value.
type Message struct {
	Topic string
	Key   []byte
	Value []byte
}
