// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
)

func BenchmarkOnBrokerConnect(b *testing.B) {
	testTel := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
	require.NoError(b, err)
	defer tb.Shutdown()
	fpm := NewFranzProducerMetrics(tb)
	meta := kgo.BrokerMetadata{NodeID: 1, Host: "broker1"}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		fpm.OnBrokerConnect(meta, time.Minute, nil, nil)
	}
}

func BenchmarkOnBrokerDisconnect(b *testing.B) {
	testTel := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
	require.NoError(b, err)
	defer tb.Shutdown()
	fpm := NewFranzProducerMetrics(tb)
	meta := kgo.BrokerMetadata{NodeID: 1, Host: "broker1"}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		fpm.OnBrokerDisconnect(meta, nil)
	}
}

func BenchmarkOnBrokerThrottle(b *testing.B) {
	testTel := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
	require.NoError(b, err)
	defer tb.Shutdown()
	fpm := NewFranzProducerMetrics(tb)
	meta := kgo.BrokerMetadata{NodeID: 1, Host: "broker1"}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		fpm.OnBrokerThrottle(meta, time.Second, false)
	}
}

func BenchmarkOnBrokerE2E(b *testing.B) {
	testTel := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
	require.NoError(b, err)
	defer tb.Shutdown()
	fpm := NewFranzProducerMetrics(tb)
	meta := kgo.BrokerMetadata{NodeID: 1, Host: "broker1"}
	e2e := kgo.BrokerE2E{
		WriteWait:   time.Second / 4,
		TimeToWrite: time.Second / 4,
		ReadWait:    time.Second / 4,
		TimeToRead:  time.Second / 4,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		fpm.OnBrokerE2E(meta, int16(kmsg.Produce), e2e)
	}
}

func BenchmarkOnProduceBatchWritten(b *testing.B) {
	testTel := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
	require.NoError(b, err)
	defer tb.Shutdown()
	fpm := NewFranzProducerMetrics(tb)
	meta := kgo.BrokerMetadata{NodeID: 1, Host: "broker1"}
	batchMetrics := kgo.ProduceBatchMetrics{
		NumRecords:        10,
		CompressedBytes:   100,
		UncompressedBytes: 1000,
		CompressionType:   1,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		fpm.OnProduceBatchWritten(meta, "foobar", 1, batchMetrics)
	}
}

func BenchmarkOnProduceRecordUnbuffered(b *testing.B) {
	testTel := componenttest.NewTelemetry()
	tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
	require.NoError(b, err)
	defer tb.Shutdown()
	fpm := NewFranzProducerMetrics(tb)
	record := &kgo.Record{Topic: "foobar", Partition: 1}
	errFail := errors.New("fail")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		fpm.OnProduceRecordUnbuffered(record, errFail)
	}
}
