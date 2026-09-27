// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

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

func newOffsetLagBenchmark(
	b *testing.B,
	partitionCount int,
) (*franzConsumer, *componenttest.Telemetry, *kgo.Client, map[string][]int32) {
	b.Helper()
	const topic = "offset-lag-benchmark"

	client, cfg := mustNewFakeCluster(b, kfake.SeedTopics(int32(partitionCount), topic))
	set, telemetry, _ := mustNewSettings(b)
	c, err := newFranzKafkaConsumer(cfg, set, []string{topic}, nil, nil)
	require.NoError(b, err)
	b.Cleanup(c.telemetryBuilder.Shutdown)

	partitions := make([]int32, partitionCount)
	for i := range partitions {
		partitions[i] = int32(i)
	}
	assignments := map[string][]int32{topic: partitions}
	c.assigned(b.Context(), client, assignments)
	return c, telemetry, client, assignments
}

// BenchmarkOffsetLagMetricCollection measures a full metric collection cycle,
// which observes both async gauges (offset lag and current offset).
func BenchmarkOffsetLagMetricCollection(b *testing.B) {
	for _, partitionCount := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("partitions=%d", partitionCount), func(b *testing.B) {
			c, telemetry, _, _ := newOffsetLagBenchmark(b, partitionCount)
			c.mu.RLock()
			for _, pc := range c.assignments {
				pc.offsetLag.Store(1)
				pc.offsetLagReportable.Store(true)
				pc.currentOffset.Store(1)
				pc.currentOffsetReportable.Store(true)
			}
			c.mu.RUnlock()

			// Warm SDK aggregation and reuse its output buffer during measurement.
			var rm metricdata.ResourceMetrics
			require.NoError(b, telemetry.Reader.Collect(b.Context(), &rm))

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := telemetry.Reader.Collect(b.Context(), &rm); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkOffsetLagPartitionReassignment(b *testing.B) {
	for _, partitionCount := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("partitions=%d", partitionCount), func(b *testing.B) {
			c, _, client, assignments := newOffsetLagBenchmark(b, partitionCount)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				c.lost(b.Context(), client, assignments, true)
				c.assigned(b.Context(), client, assignments)
			}
		})
	}
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
