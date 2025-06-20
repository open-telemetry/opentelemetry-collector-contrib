// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
)

// SaramaSyncProducer is a wrapper around the kafkaclient.Producer that implements
// the sarama.SyncProducer interface. This allows us to use the new franz-go
// client while maintaining compatibility with the existing Sarama-based code.
type SaramaSyncProducer struct {
	producer     sarama.SyncProducer
	tb           *metadata.TelemetryBuilder
	metadataKeys []string
}

// NewSaramaSyncProducer creates a new SaramaSyncProducer that wraps a kafkaclient.Producer.
func NewSaramaSyncProducer(
	producer sarama.SyncProducer,
	tb *metadata.TelemetryBuilder,
	metadataKeys []string,
) *SaramaSyncProducer {
	return &SaramaSyncProducer{
		producer:     producer,
		tb:           tb,
		metadataKeys: metadataKeys,
	}
}

// ExportData sends multiple messages to the Kafka broker using the underlying producer.
func (p *SaramaSyncProducer) ExportData(ctx context.Context, msgs Messages) (err error) {
	messages := makeSaramaMessages(msgs)
	setMessageHeaders(ctx, messages, p.metadataKeys,
		func(key string, value []byte) sarama.RecordHeader {
			return sarama.RecordHeader{Key: []byte(key), Value: value}
		},
		func(m *sarama.ProducerMessage) []sarama.RecordHeader { return m.Headers },
		func(m *sarama.ProducerMessage, h []sarama.RecordHeader) { m.Headers = h },
	)
	defer reportProducerMetric(p.tb, messages, err, time.Now())
	if err = p.producer.SendMessages(messages); err != nil {
		err = wrapKafkaProducerError(err)
	}
	return
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

func reportProducerMetric(tb *metadata.TelemetryBuilder, msgs []*sarama.ProducerMessage, err error, t time.Time) {
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	tb.KafkaExporterLatency.Record(context.Background(), time.Since(t).Milliseconds(), metric.WithAttributes(
		attribute.String("outcome", outcome),
	))
	type topicPartition struct {
		topic     string
		partition int
	}
	type stats struct {
		records int64
		bytes   int64
	}
	perTopicPartition := make(map[topicPartition]stats, len(msgs))
	for _, m := range msgs {
		k := topicPartition{topic: m.Topic, partition: int(m.Partition)}
		s := perTopicPartition[k]
		s.records += 1
		s.bytes += int64(m.ByteSize(2))
		perTopicPartition[k] = s
	}
	var prodErrs sarama.ProducerErrors
	if errors.As(err, &prodErrs) {
		for _, err := range prodErrs {
			bytes := int64(err.Msg.ByteSize(2))
			k := topicPartition{topic: err.Msg.Topic, partition: int(err.Msg.Partition)}
			s := perTopicPartition[k]
			s.records -= 1
			s.bytes -= bytes
			perTopicPartition[k] = s
			attrs := []attribute.KeyValue{
				attribute.String("topic", k.topic),
				attribute.Int("partition", k.partition),
				attribute.String("outcome", "failure"),
			}
			tb.KafkaExporterBytesUncompressed.Add(context.Background(), bytes, metric.WithAttributes(attrs...))
			tb.KafkaExporterRecords.Add(context.Background(), 1, metric.WithAttributes(attrs...))
		}
		// All failed records are already reported, all the remaining records should have succeeded.
		outcome = "success"
	}
	for tp, s := range perTopicPartition {
		attrs := []attribute.KeyValue{
			attribute.String("topic", tp.topic),
			attribute.Int("partition", tp.partition),
			attribute.String("outcome", outcome),
		}
		tb.KafkaExporterBytesUncompressed.Add(context.Background(), s.bytes, metric.WithAttributes(attrs...))
		tb.KafkaExporterRecords.Add(context.Background(), s.records, metric.WithAttributes(attrs...))
	}
}
