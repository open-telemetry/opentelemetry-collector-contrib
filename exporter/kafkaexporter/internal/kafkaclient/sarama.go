// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"errors"
	"fmt"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

// SaramaSyncProducer is a wrapper around the kafkaclient.Producer that implements
// the sarama.SyncProducer interface. This allows us to use the new franz-go
// client while maintaining compatibility with the existing Sarama-based code.
type SaramaSyncProducer struct {
	producer     sarama.SyncProducer
	metadataKeys []string
}

// NewSaramaSyncProducer creates a new SaramaSyncProducer that wraps a kafkaclient.Producer.
func NewSaramaSyncProducer(producer sarama.SyncProducer,
	metadataKeys []string,
) *SaramaSyncProducer {
	return &SaramaSyncProducer{
		producer:     producer,
		metadataKeys: metadataKeys,
	}
}

// ExportData sends multiple messages to the Kafka broker using the underlying producer.
func (p *SaramaSyncProducer) ExportData(ctx context.Context, msgs Messages) error {
	messages := makeSaramaMessages(msgs)
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
func (p *SaramaSyncProducer) Close() error {
	return p.producer.Close()
}

func makeSaramaMessages(messages Messages) []*sarama.ProducerMessage {
	msgs := make([]*sarama.ProducerMessage, 0, messages.Count)
	for _, msg := range messages.TopicMessages {
		for _, message := range msg.Messages {
			msg := &sarama.ProducerMessage{Topic: msg.Topic}
			if message.Key != nil {
				msg.Key = sarama.ByteEncoder(message.Key)
			}
			if message.Value != nil {
				msg.Value = sarama.ByteEncoder(message.Value)
			}
			msgs = append(msgs, msg)
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
