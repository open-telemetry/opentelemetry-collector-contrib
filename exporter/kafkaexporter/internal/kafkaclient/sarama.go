// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"errors"
	"fmt"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/consumer/consumererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
)

// SaramaSyncProducer is a wrapper around the kafkaclient.Producer that implements
// the sarama.SyncProducer interface. This allows us to use the new franz-go
// client while maintaining compatibility with the existing Sarama-based code.
type SaramaSyncProducer[T any] struct {
	producer     sarama.SyncProducer
	messenger    Messenger[T]
	metadataKeys []string
}

// NewSaramaSyncProducer creates a new SaramaSyncProducer that wraps a kafkaclient.Producer.
func NewSaramaSyncProducer[T any](producer sarama.SyncProducer,
	marshaler Messenger[T],
	metadataKeys []string,
) *SaramaSyncProducer[T] {
	return &SaramaSyncProducer[T]{
		producer:     producer,
		messenger:    marshaler,
		metadataKeys: metadataKeys,
	}
}

// ExportData sends multiple messages to the Kafka broker using the underlying producer.
func (p *SaramaSyncProducer[T]) ExportData(ctx context.Context, data T) error {
	messages, err := partitionMessages(ctx,
		data, p.messenger, makeSaramaMessages,
	)
	if err != nil || len(messages) == 0 {
		return wrapKafkaProducerError(err)
	}
	setMessageHeaders(ctx, messages, p.metadataKeys,
		func(key string, value []byte) sarama.RecordHeader {
			return sarama.RecordHeader{Key: []byte(key), Value: value}
		},
		func(m *sarama.ProducerMessage) []sarama.RecordHeader { return m.Headers },
		func(m *sarama.ProducerMessage, h []sarama.RecordHeader) { m.Headers = h },
	)
	if err := p.producer.SendMessages(messages); err != nil {
		return wrapKafkaProducerError(err)
	}
	return nil
}

// Close shuts down the producer and flushes any remaining messages.
// It implements the sarama.SyncProducer interface.
func (p *SaramaSyncProducer[T]) Close() error {
	return p.producer.Close()
}

func makeSaramaMessages(messages []marshaler.Message, topic string) []*sarama.ProducerMessage {
	msgs := make([]*sarama.ProducerMessage, len(messages))
	for i, message := range messages {
		msgs[i] = &sarama.ProducerMessage{Topic: topic}
		if message.Key != nil {
			msgs[i].Key = sarama.ByteEncoder(message.Key)
		}
		if message.Value != nil {
			msgs[i].Value = sarama.ByteEncoder(message.Value)
		}
	}
	return msgs
}

type kafkaErrors struct {
	count int
	err   string
}

func (ke kafkaErrors) Error() string {
	return fmt.Sprintf("Failed to deliver %d messages due to %s", ke.count, ke.err)
}

func wrapKafkaProducerError(err error) error {
	var prodErr sarama.ProducerErrors
	if !errors.As(err, &prodErr) || len(prodErr) == 0 {
		return err
	}
	var areConfigErrs bool
	var confErr sarama.ConfigurationError
	for _, producerErr := range prodErr {
		if areConfigErrs = errors.As(producerErr.Err, &confErr); !areConfigErrs {
			break
		}
	}
	if areConfigErrs {
		return consumererror.NewPermanent(confErr)
	}

	return kafkaErrors{len(prodErr), prodErr[0].Err.Error()}
}
