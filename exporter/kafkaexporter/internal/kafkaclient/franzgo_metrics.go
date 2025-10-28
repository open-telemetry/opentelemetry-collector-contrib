// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"net"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
)

// FranzProducerMetrics implements the relevant franz-go hook interfaces to
// record the metrics defined in the metadata telemetry.
type FranzProducerMetrics struct {
	tb *metadata.TelemetryBuilder
}

// NewFranzProducerMetrics creates an instance of FranzProducerMetrics from metadata TelemetryBuilder.
func NewFranzProducerMetrics(tb *metadata.TelemetryBuilder) FranzProducerMetrics {
	return FranzProducerMetrics{tb: tb}
}

var _ kgo.HookBrokerConnect = FranzProducerMetrics{}

func (fpm FranzProducerMetrics) OnBrokerConnect(meta kgo.BrokerMetadata, _ time.Duration, _ net.Conn, err error) {
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	fpm.tb.KafkaBrokerConnects.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String("node_id", kgo.NodeName(meta.NodeID)),
			attribute.String("outcome", outcome),
		),
	)
}

var _ kgo.HookBrokerDisconnect = FranzProducerMetrics{}

func (fpm FranzProducerMetrics) OnBrokerDisconnect(meta kgo.BrokerMetadata, _ net.Conn) {
	fpm.tb.KafkaBrokerClosed.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String("node_id", kgo.NodeName(meta.NodeID)),
		),
	)
}

var _ kgo.HookBrokerThrottle = FranzProducerMetrics{}

func (fpm FranzProducerMetrics) OnBrokerThrottle(meta kgo.BrokerMetadata, throttleInterval time.Duration, _ bool) {
	// KafkaBrokerThrottlingDuration is deprecated in favor of KafkaBrokerThrottlingLatency.
	fpm.tb.KafkaBrokerThrottlingDuration.Record(
		context.Background(),
		throttleInterval.Milliseconds(),
		metric.WithAttributes(
			attribute.String("node_id", kgo.NodeName(meta.NodeID)),
		),
	)
	fpm.tb.KafkaBrokerThrottlingLatency.Record(
		context.Background(),
		throttleInterval.Seconds(),
		metric.WithAttributes(
			attribute.String("node_id", kgo.NodeName(meta.NodeID)),
		),
	)
}

var _ kgo.HookBrokerE2E = FranzProducerMetrics{}

func (fpm FranzProducerMetrics) OnBrokerE2E(meta kgo.BrokerMetadata, _ int16, e2e kgo.BrokerE2E) {
	outcome := "success"
	if e2e.Err() != nil {
		outcome = "failure"
	}
	// KafkaExporterLatency is deprecated in favor of KafkaExporterWriteLatency.
	fpm.tb.KafkaExporterLatency.Record(
		context.Background(),
		e2e.DurationE2E().Milliseconds()+e2e.WriteWait.Milliseconds(),
		metric.WithAttributes(
			attribute.String("node_id", kgo.NodeName(meta.NodeID)),
			attribute.String("outcome", outcome),
		),
	)
	fpm.tb.KafkaExporterWriteLatency.Record(
		context.Background(),
		e2e.DurationE2E().Seconds()+e2e.WriteWait.Seconds(),
		metric.WithAttributes(
			attribute.String("node_id", kgo.NodeName(meta.NodeID)),
			attribute.String("outcome", outcome),
		),
	)
}

var _ kgo.HookProduceBatchWritten = FranzProducerMetrics{}

// OnProduceBatchWritten is called when a batch has been produced.
// https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo#HookProduceBatchWritten
func (fpm FranzProducerMetrics) OnProduceBatchWritten(meta kgo.BrokerMetadata, topic string, partition int32, m kgo.ProduceBatchMetrics) {
	attrs := []attribute.KeyValue{
		attribute.String("node_id", kgo.NodeName(meta.NodeID)),
		attribute.String("topic", topic),
		attribute.Int64("partition", int64(partition)),
		attribute.String("compression_codec", compressionFromCodec(m.CompressionType)),
		attribute.String("outcome", "success"),
	}
	// KafkaExporterMessages is deprecated in favor of KafkaExporterRecords.
	fpm.tb.KafkaExporterMessages.Add(
		context.Background(),
		int64(m.NumRecords),
		metric.WithAttributes(attrs...),
	)
	fpm.tb.KafkaExporterRecords.Add(
		context.Background(),
		int64(m.NumRecords),
		metric.WithAttributes(attrs...),
	)
	fpm.tb.KafkaExporterBytes.Add(
		context.Background(),
		int64(m.CompressedBytes),
		metric.WithAttributes(attrs...),
	)
	fpm.tb.KafkaExporterBytesUncompressed.Add(
		context.Background(),
		int64(m.UncompressedBytes),
		metric.WithAttributes(attrs...),
	)
}

var _ kgo.HookProduceRecordUnbuffered = FranzProducerMetrics{}

// OnProduceRecordUnbuffered records the number of produced messages that were
// not produced due to errors. The successfully produced records is recorded by
// `OnProduceBatchWritten`.
// https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo#HookProduceRecordUnbuffered
func (fpm FranzProducerMetrics) OnProduceRecordUnbuffered(r *kgo.Record, err error) {
	if err == nil {
		return // Covered by OnProduceBatchWritten.
	}
	attrs := []attribute.KeyValue{
		attribute.String("topic", r.Topic),
		attribute.Int64("partition", int64(r.Partition)),
		attribute.String("outcome", "failure"),
	}
	// KafkaExporterMessages is deprecated in favor of KafkaExporterRecords.
	fpm.tb.KafkaExporterMessages.Add(
		context.Background(),
		1,
		metric.WithAttributes(attrs...),
	)
	fpm.tb.KafkaExporterRecords.Add(
		context.Background(),
		1,
		metric.WithAttributes(attrs...),
	)
}

func compressionFromCodec(c uint8) string {
	// CompressionType signifies which algorithm the batch was compressed
	// with.
	//
	// 0 is no compression, 1 is gzip, 2 is snappy, 3 is lz4, and 4 is
	// zstd.
	switch c {
	case 0:
		return "none"
	case 1:
		return "gzip"
	case 2:
		return "snappy"
	case 3:
		return "lz4"
	case 4:
		return "zstd"
	default:
		return "unknown"
	}
}
