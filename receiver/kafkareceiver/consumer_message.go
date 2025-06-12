// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"iter"
	"time"

	"github.com/IBM/sarama"
)

// kafkaMessage provides a generic interface for Kafka messages that abstracts
// over both Sarama and Franz-go record types.
type kafkaMessage interface {
	value() []byte
	headers() messageHeaders
	topic() string
	partition() int32
	offset() int64
	timestamp() time.Time
}

type header struct {
	key   string
	value []byte
}

// messageHeaders provides a generic interface for accessing Kafka message headers.
type messageHeaders interface {
	get(key string) (string, bool)
	all() iter.Seq[header]
}

// saramaMessage wraps a Sarama ConsumerMessage to implement KafkaMessage interface.
type saramaMessage struct {
	msg *sarama.ConsumerMessage
}

func wrapSaramaMsg(message *sarama.ConsumerMessage) saramaMessage {
	return saramaMessage{msg: message}
}

func (w saramaMessage) value() []byte {
	return w.msg.Value
}

func (w saramaMessage) headers() messageHeaders {
	return saramaHeaders{headers: w.msg.Headers}
}

func (w saramaMessage) topic() string {
	return w.msg.Topic
}

func (w saramaMessage) partition() int32 {
	return w.msg.Partition
}

func (w saramaMessage) offset() int64 {
	return w.msg.Offset
}

func (w saramaMessage) timestamp() time.Time {
	return w.msg.Timestamp
}

type saramaHeaders struct {
	headers []*sarama.RecordHeader
}

func (h saramaHeaders) get(key string) (string, bool) {
	for _, header := range h.headers {
		if string(header.Key) == key {
			return string(header.Value), true
		}
	}
	return "", false
}

func (h saramaHeaders) all() iter.Seq[header] {
	return func(yield func(header) bool) {
		for _, hdr := range h.headers {
			if !yield(header{key: string(hdr.Key), value: hdr.Value}) {
				return
			}
		}
	}
}
