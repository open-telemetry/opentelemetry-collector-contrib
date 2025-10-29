// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
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
		err = testTel.Reader.Collect(t.Context(), &rm)
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
		err = testTel.Reader.Collect(t.Context(), &rm)
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
	t.Run("should report the metrics when OnBrokerE2E hook is called", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		fpm := NewFranzProducerMetrics(tb)
		fpm.OnBrokerE2E(kgo.BrokerMetadata{NodeID: 1}, 0, kgo.BrokerE2E{
			WriteWait:   time.Second / 4,
			TimeToWrite: time.Second / 4,
			ReadWait:    time.Second / 4,
			TimeToRead:  time.Second / 4,
			WriteErr:    nil,
			ReadErr:     nil,
		})
		fpm.OnBrokerE2E(kgo.BrokerMetadata{NodeID: 1}, 0, kgo.BrokerE2E{
			WriteWait:   time.Second * 25,
			TimeToWrite: time.Second * 25,
			ReadWait:    time.Second * 25,
			TimeToRead:  time.Second * 25,
			WriteErr:    errors.New(""),
			ReadErr:     nil,
		})
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(t.Context(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 2)
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
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0},
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
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
					Min:          metricdata.NewExtrema[int64](100000),
					Max:          metricdata.NewExtrema[int64](100000),
					Sum:          100000,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
		metadatatest.AssertEqualKafkaExporterWriteLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[float64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
						attribute.String("outcome", "success"),
					),
					Count:        1,
					Bounds:       []float64{0, 0.005, 0.010, 0.025, 0.050, 0.075, 0.100, 0.250, 0.500, 0.750, 1, 2.5, 5, 7.5, 10, 25, 50, 75, 100},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0},
					Min:          metricdata.NewExtrema(1.0),
					Max:          metricdata.NewExtrema(1.0),
					Sum:          1.0,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
						attribute.String("outcome", "failure"),
					),
					Count:        1,
					Bounds:       []float64{0, 0.005, 0.010, 0.025, 0.050, 0.075, 0.100, 0.250, 0.500, 0.750, 1, 2.5, 5, 7.5, 10, 25, 50, 75, 100},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0},
					Min:          metricdata.NewExtrema(100.0),
					Max:          metricdata.NewExtrema(100.0),
					Sum:          100.0,
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
		err = testTel.Reader.Collect(t.Context(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 4)
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
		metadatatest.AssertEqualKafkaExporterRecords(
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
		err = testTel.Reader.Collect(t.Context(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 2)
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
		metadatatest.AssertEqualKafkaExporterRecords(
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
		err = testTel.Reader.Collect(t.Context(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 2)
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
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0},
					Min:          metricdata.NewExtrema[int64](1000),
					Max:          metricdata.NewExtrema[int64](1000),
					Sum:          1000,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
		metadatatest.AssertEqualKafkaBrokerThrottlingLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[float64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("node_id", "1"),
					),
					Count:        1,
					Bounds:       []float64{0, 0.005, 0.010, 0.025, 0.050, 0.075, 0.100, 0.250, 0.500, 0.750, 1, 2.5, 5, 7.5, 10, 25, 50, 75, 100},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0},
					Min:          metricdata.NewExtrema(1.0),
					Max:          metricdata.NewExtrema(1.0),
					Sum:          1.0,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
}
