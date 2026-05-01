// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
)

// brokerKey is the cache key for per-broker attribute sets used in OnBrokerE2E.
// The set of distinct (nodeID, host, outcome) combinations is bounded by
// 2 × number-of-brokers (success + failure), so the cache never grows unboundedly.
type brokerKey struct {
	nodeID  int32
	host    string
	outcome string // "success" | "failure"
}

// FranzProducerMetrics implements the relevant franz-go hook interfaces to
// record the metrics defined in the metadata telemetry.
type FranzProducerMetrics struct {
	tb            *metadata.TelemetryBuilder
	brokerE2EMu   sync.RWMutex
	brokerE2EOpts map[brokerKey]metric.MeasurementOption
}

// NewFranzProducerMetrics creates an instance of FranzProducerMetrics from metadata TelemetryBuilder.
func NewFranzProducerMetrics(tb *metadata.TelemetryBuilder) *FranzProducerMetrics {
	return &FranzProducerMetrics{
		tb:            tb,
		brokerE2EOpts: make(map[brokerKey]metric.MeasurementOption),
	}
}

// brokerE2EOpt returns a cached MeasurementOption for the given key, building
// and storing it on first use.
func (fpm *FranzProducerMetrics) brokerE2EOpt(k brokerKey) metric.MeasurementOption {
	fpm.brokerE2EMu.RLock()
	opt, ok := fpm.brokerE2EOpts[k]
	fpm.brokerE2EMu.RUnlock()
	if ok {
		return opt
	}
	attrs := attribute.NewSet(
		attribute.String("node_id", kgo.NodeName(k.nodeID)),
		attribute.String("server.address", k.host),
		attribute.String("outcome", k.outcome),
	)
	opt = metric.WithAttributeSet(attrs)
	fpm.brokerE2EMu.Lock()
	fpm.brokerE2EOpts[k] = opt
	fpm.brokerE2EMu.Unlock()
	return opt
}

var _ kgo.HookBrokerConnect = (*FranzProducerMetrics)(nil)

func (fpm *FranzProducerMetrics) OnBrokerConnect(meta kgo.BrokerMetadata, _ time.Duration, _ net.Conn, err error) {
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	fpm.tb.KafkaBrokerConnects.Add(
		context.Background(),
		1,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("node_id", kgo.NodeName(meta.NodeID)),
			attribute.String("server.address", meta.Host),
			attribute.String("outcome", outcome),
		)),
	)
}

var _ kgo.HookBrokerDisconnect = (*FranzProducerMetrics)(nil)

func (fpm *FranzProducerMetrics) OnBrokerDisconnect(meta kgo.BrokerMetadata, _ net.Conn) {
	fpm.tb.KafkaBrokerClosed.Add(
		context.Background(),
		1,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("node_id", kgo.NodeName(meta.NodeID)),
			attribute.String("server.address", meta.Host),
		)),
	)
	// Evict cached attribute sets for this broker so that nodes which come
	// and go over time do not cause unbounded growth of brokerE2EOpts.
	fpm.brokerE2EMu.Lock()
	delete(fpm.brokerE2EOpts, brokerKey{nodeID: meta.NodeID, host: meta.Host, outcome: "success"})
	delete(fpm.brokerE2EOpts, brokerKey{nodeID: meta.NodeID, host: meta.Host, outcome: "failure"})
	fpm.brokerE2EMu.Unlock()
}

var _ kgo.HookBrokerThrottle = (*FranzProducerMetrics)(nil)

func (fpm *FranzProducerMetrics) OnBrokerThrottle(meta kgo.BrokerMetadata, throttleInterval time.Duration, _ bool) {
	attrs := attribute.NewSet(
		attribute.String("node_id", kgo.NodeName(meta.NodeID)),
		attribute.String("server.address", meta.Host),
	)
	// KafkaBrokerThrottlingDuration is deprecated in favor of KafkaBrokerThrottlingLatency.
	fpm.tb.KafkaBrokerThrottlingDuration.Record(
		context.Background(),
		throttleInterval.Milliseconds(),
		metric.WithAttributeSet(attrs),
	)
	fpm.tb.KafkaBrokerThrottlingLatency.Record(
		context.Background(),
		throttleInterval.Seconds(),
		metric.WithAttributeSet(attrs),
	)
}

var _ kgo.HookBrokerE2E = (*FranzProducerMetrics)(nil)

func (fpm *FranzProducerMetrics) OnBrokerE2E(meta kgo.BrokerMetadata, key int16, e2e kgo.BrokerE2E) {
	// Do not pollute producer metrics with non-produce requests
	if kmsg.Key(key) != kmsg.Produce {
		return
	}

	outcome := "success"
	if e2e.Err() != nil {
		outcome = "failure"
	}
	opt := fpm.brokerE2EOpt(brokerKey{nodeID: meta.NodeID, host: meta.Host, outcome: outcome})
	// KafkaExporterLatency is deprecated in favor of KafkaExporterWriteLatency.
	fpm.tb.KafkaExporterLatency.Record(
		context.Background(),
		e2e.DurationE2E().Milliseconds()+e2e.WriteWait.Milliseconds(),
		opt,
	)
	fpm.tb.KafkaExporterWriteLatency.Record(
		context.Background(),
		e2e.DurationE2E().Seconds()+e2e.WriteWait.Seconds(),
		opt,
	)
}

var _ kgo.HookProduceBatchWritten = (*FranzProducerMetrics)(nil)

// OnProduceBatchWritten is called when a batch has been produced.
// https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo#HookProduceBatchWritten
func (fpm *FranzProducerMetrics) OnProduceBatchWritten(meta kgo.BrokerMetadata, topic string, partition int32, m kgo.ProduceBatchMetrics) {
	attrs := attribute.NewSet(
		attribute.String("node_id", kgo.NodeName(meta.NodeID)),
		attribute.String("server.address", meta.Host),
		attribute.String("topic", topic),
		attribute.Int64("partition", int64(partition)),
		attribute.String("compression_codec", compressionFromCodec(m.CompressionType)),
		attribute.String("outcome", "success"),
	)
	opt := metric.WithAttributeSet(attrs)
	// KafkaExporterMessages is deprecated in favor of KafkaExporterRecords.
	fpm.tb.KafkaExporterMessages.Add(
		context.Background(),
		int64(m.NumRecords),
		opt,
	)
	fpm.tb.KafkaExporterRecords.Add(
		context.Background(),
		int64(m.NumRecords),
		opt,
	)
	fpm.tb.KafkaExporterBytes.Add(
		context.Background(),
		int64(m.CompressedBytes),
		opt,
	)
	fpm.tb.KafkaExporterBytesUncompressed.Add(
		context.Background(),
		int64(m.UncompressedBytes),
		opt,
	)
}

var _ kgo.HookProduceRecordUnbuffered = (*FranzProducerMetrics)(nil)

// OnProduceRecordUnbuffered records the number of produced messages that were
// not produced due to errors. The successfully produced records is recorded by
// `OnProduceBatchWritten`.
// https://pkg.go.dev/github.com/twmb/franz-go/pkg/kgo#HookProduceRecordUnbuffered
func (fpm *FranzProducerMetrics) OnProduceRecordUnbuffered(r *kgo.Record, err error) {
	if err == nil {
		return // Covered by OnProduceBatchWritten.
	}
	opt := metric.WithAttributeSet(attribute.NewSet(
		attribute.String("topic", r.Topic),
		attribute.Int64("partition", int64(r.Partition)),
		attribute.String("outcome", "failure"),
	))
	// KafkaExporterMessages is deprecated in favor of KafkaExporterRecords.
	fpm.tb.KafkaExporterMessages.Add(
		context.Background(),
		1,
		opt,
	)
	fpm.tb.KafkaExporterRecords.Add(
		context.Background(),
		1,
		opt,
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
