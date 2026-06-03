// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func newBenchFranzConsumer(b *testing.B) *franzConsumer {
	b.Helper()
	set := receivertest.NewNopSettings(metadata.Type)
	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	require.NoError(b, err)
	return &franzConsumer{
		settings:         set,
		telemetryBuilder: tb,
	}
}

var benchBrokerMeta = kgo.BrokerMetadata{
	NodeID: 1,
	Host:   "broker-1",
	Port:   9092,
}

func BenchmarkOnBrokerConnect(b *testing.B) {
	c := newBenchFranzConsumer(b)
	conn := &net.TCPConn{}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.OnBrokerConnect(benchBrokerMeta, time.Millisecond, conn, nil)
	}
}

func BenchmarkOnBrokerDisconnect(b *testing.B) {
	c := newBenchFranzConsumer(b)
	conn := &net.TCPConn{}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.OnBrokerDisconnect(benchBrokerMeta, conn)
	}
}

func BenchmarkOnBrokerThrottle(b *testing.B) {
	c := newBenchFranzConsumer(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.OnBrokerThrottle(benchBrokerMeta, 100*time.Millisecond, false)
	}
}

func BenchmarkOnBrokerRead(b *testing.B) {
	c := newBenchFranzConsumer(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.OnBrokerRead(benchBrokerMeta, 1, 1024, time.Millisecond, 5*time.Millisecond, nil)
	}
}

func BenchmarkOnFetchBatchRead(b *testing.B) {
	c := newBenchFranzConsumer(b)
	metrics := kgo.FetchBatchMetrics{
		CompressedBytes:   4096,
		UncompressedBytes: 8192,
		NumRecords:        100,
		CompressionType:   1, // gzip
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.OnFetchBatchRead(benchBrokerMeta, "test-topic", 0, metrics)
	}
}

func BenchmarkOnFetchRecordUnbuffered(b *testing.B) {
	set := receivertest.NewNopSettings(metadata.Type)
	// Enable the optional metric so the telemetry builder creates it.
	set.TelemetrySettings = componenttest.NewNopTelemetrySettings()
	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	require.NoError(b, err)
	c := franzConsumerWithOptionalHooks{&franzConsumer{
		settings:         set,
		telemetryBuilder: tb,
	}}
	record := &kgo.Record{
		Topic:     "test-topic",
		Partition: 0,
		Timestamp: time.Now().Add(-50 * time.Millisecond),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.OnFetchRecordUnbuffered(record, true)
	}
}
