// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadatatest"
)

func TestFranzProducerMetrics(t *testing.T) {
	t.Run("should report the metrics when OnBrokerConnect hook is called", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		fpm := NewFranzProducerMetrics(tb)
		fpm.OnBrokerConnect(kgo.BrokerMetadata{NodeID: 1}, time.Minute, nil, nil)
		fpm.OnBrokerConnect(kgo.BrokerMetadata{NodeID: 1}, time.Minute, nil, errors.New(""))
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
		metadatatest.AssertEqualKafkaBrokerConnects(
			t,
			testTel,
			[]metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
						attribute.String("outcome", "success"),
					),
					Value: 1,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
						attribute.String("outcome", "failure"),
					),
					Value: 1,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
	t.Run("should report the metrics when OnBrokerDisconnect hook is called", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		fpm := NewFranzProducerMetrics(tb)
		fpm.OnBrokerDisconnect(kgo.BrokerMetadata{NodeID: 1}, nil)
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
		metadatatest.AssertEqualKafkaBrokerClosed(
			t,
			testTel,
			[]metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(attribute.String("node_id", "1")),
					Value:      1,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
	t.Run("should report the metrics when OnBrokerWrite hook is called", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		fpm := NewFranzProducerMetrics(tb)
		fpm.OnBrokerWrite(kgo.BrokerMetadata{NodeID: 1}, 0, 0, time.Second/2, time.Second/2, nil)
		fpm.OnBrokerWrite(kgo.BrokerMetadata{NodeID: 1}, 0, 0, 100*time.Second, 0, errors.New(""))
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
		metadatatest.AssertEqualKafkaExporterLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
						attribute.String("outcome", "success"),
					),
					Count:        1,
					Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					BucketCounts: []uint64{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0},
					Min:          metricdata.NewExtrema[int64](1000),
					Max:          metricdata.NewExtrema[int64](1000),
					Sum:          1000,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
						attribute.String("outcome", "failure"),
					),
					Count:        1,
					Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					BucketCounts: []uint64{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1},
					Min:          metricdata.NewExtrema[int64](100000),
					Max:          metricdata.NewExtrema[int64](100000),
					Sum:          100000,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
	t.Run("should report the metrics when OnProduceBatchWritten hook is called", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		fpm := NewFranzProducerMetrics(tb)
		fpm.OnProduceBatchWritten(kgo.BrokerMetadata{NodeID: 1}, "foobar", 1, kgo.ProduceBatchMetrics{
			NumRecords:        10,
			CompressedBytes:   100,
			UncompressedBytes: 1000,
			CompressionType:   1,
		})
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 3)
		metadatatest.AssertEqualKafkaExporterMessages(
			t,
			testTel,
			[]metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
						attribute.String("topic", "foobar"),
						attribute.Int64("partition", 1),
						attribute.String("compression_codec", "gzip"),
						attribute.String("outcome", "success"),
					),
					Value: 10,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
		metadatatest.AssertEqualKafkaExporterBytes(
			t,
			testTel,
			[]metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
						attribute.String("topic", "foobar"),
						attribute.Int64("partition", 1),
						attribute.String("compression_codec", "gzip"),
						attribute.String("outcome", "success"),
					),
					Value: 100,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
		metadatatest.AssertEqualKafkaExporterBytesUncompressed(
			t,
			testTel,
			[]metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
						attribute.String("topic", "foobar"),
						attribute.Int64("partition", 1),
						attribute.String("compression_codec", "gzip"),
						attribute.String("outcome", "success"),
					),
					Value: 1000,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
	t.Run("should report the metrics when OnProduceRecordUnbuffered hook is called", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		fpm := NewFranzProducerMetrics(tb)
		fpm.OnProduceRecordUnbuffered(&kgo.Record{}, nil)
		fpm.OnProduceRecordUnbuffered(&kgo.Record{Topic: "foobar", Partition: 1}, errors.New(""))
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
		metadatatest.AssertEqualKafkaExporterMessages(
			t,
			testTel,
			[]metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("topic", "foobar"),
						attribute.Int64("partition", 1),
						attribute.String("outcome", "failure"),
					),
					Value: 1,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
	t.Run("should report the metrics when OnBrokerThrottle hook is called", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		fpm := NewFranzProducerMetrics(tb)
		fpm.OnBrokerThrottle(kgo.BrokerMetadata{NodeID: 1}, time.Second, false)
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
		metadatatest.AssertEqualKafkaBrokerThrottlingDuration(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
					),
					Count:        1,
					Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					BucketCounts: []uint64{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0},
					Min:          metricdata.NewExtrema[int64](1000),
					Max:          metricdata.NewExtrema[int64](1000),
					Sum:          1000,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
}
