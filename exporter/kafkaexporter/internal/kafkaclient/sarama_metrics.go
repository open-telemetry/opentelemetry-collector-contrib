// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"errors"
	"time"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
)

// SaramaProducerMetrics helps to record the metrics defined in the metadata telemetry for Sarama.
type SaramaProducerMetrics struct {
	tb    *metadata.TelemetryBuilder
	attrs attribute.Set
}

// NewSaramaProducerMetrics creates an instance of SaramaProducerMetrics from metadata TelemetryBuilder.
func NewSaramaProducerMetrics(tb *metadata.TelemetryBuilder, attributes ...attribute.KeyValue) SaramaProducerMetrics {
	return SaramaProducerMetrics{tb: tb, attrs: attribute.NewSet(attributes...)}
}

func (spm SaramaProducerMetrics) ReportProducerMetrics(msgs []*sarama.ProducerMessage, err error, t time.Time) {
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	spm.tb.KafkaExporterLatency.Record(
		context.Background(),
		time.Since(t).Milliseconds(),
		metric.WithAttributes(
			attribute.String("outcome", outcome),
		),
		metric.WithAttributeSet(spm.attrs),
	)
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
		s.records++
		s.bytes += int64(m.ByteSize(2))
		perTopicPartition[k] = s
	}
	var prodErrs sarama.ProducerErrors
	if errors.As(err, &prodErrs) {
		for _, err := range prodErrs {
			bytes := int64(err.Msg.ByteSize(2))
			k := topicPartition{topic: err.Msg.Topic, partition: int(err.Msg.Partition)}
			s := perTopicPartition[k]
			s.records--
			s.bytes -= bytes
			perTopicPartition[k] = s
			attrs := []attribute.KeyValue{
				attribute.String("topic", k.topic),
				attribute.Int("partition", k.partition),
				attribute.String("outcome", "failure"),
			}
			spm.tb.KafkaExporterBytesUncompressed.Add(
				context.Background(),
				bytes,
				metric.WithAttributes(attrs...),
				metric.WithAttributeSet(spm.attrs),
			)
			spm.tb.KafkaExporterRecords.Add(
				context.Background(),
				1,
				metric.WithAttributes(attrs...),
				metric.WithAttributeSet(spm.attrs),
			)
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
		spm.tb.KafkaExporterBytesUncompressed.Add(
			context.Background(),
			s.bytes,
			metric.WithAttributes(attrs...),
			metric.WithAttributeSet(spm.attrs),
		)
		spm.tb.KafkaExporterRecords.Add(
			context.Background(),
			s.records,
			metric.WithAttributes(attrs...),
			metric.WithAttributeSet(spm.attrs),
		)
	}
}
