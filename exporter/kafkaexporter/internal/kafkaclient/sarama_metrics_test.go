// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadatatest"
)

func TestSaramaProducerMetrics(t *testing.T) {
	t.Run("should only report latency with empty message list", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		spm := NewSaramaProducerMetrics(tb)
		spm.ReportProducerMetrics(context.TODO(), nil, nil, time.Now().Add(-time.Minute))
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 2)
		metadatatest.AssertEqualKafkaExporterLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[int64]{
				{
					Attributes:   attribute.NewSet(attribute.String("outcome", "success")),
					Count:        1,
					Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
					Min:          metricdata.NewExtrema[int64](60000),
					Max:          metricdata.NewExtrema[int64](60000),
					Sum:          60000,
				},
			},
			metricdatatest.IgnoreTimestamp(),
			metricdatatest.IgnoreValue(),
		)
		metadatatest.AssertEqualKafkaExporterWriteLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[float64]{
				{
					Attributes:   attribute.NewSet(attribute.String("outcome", "success")),
					Count:        1,
					Bounds:       []float64{0, 0.005, 0.010, 0.025, 0.050, 0.075, 0.100, 0.250, 0.500, 0.750, 1, 2.5, 5, 7.5, 10, 25, 50, 75, 100},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0},
					Min:          metricdata.NewExtrema(60.0),
					Max:          metricdata.NewExtrema(60.0),
					Sum:          60.0,
				},
			},
			metricdatatest.IgnoreTimestamp(),
			metricdatatest.IgnoreValue(),
		)
	})
	t.Run("should only report success outcome for metrics when no error provided", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		spm := NewSaramaProducerMetrics(tb)
		spm.ReportProducerMetrics(context.TODO(), makeSaramaMessages(Messages{
			TopicMessages: []TopicMessages{
				{Topic: "foo", Messages: []marshaler.Message{
					{Key: []byte("k1"), Value: []byte("v1")},
					{Key: []byte("k2"), Value: []byte("v2")},
				}},
				{Topic: "bar", Messages: []marshaler.Message{
					{Key: []byte("k1"), Value: []byte("v1")},
				}},
			},
		}), nil, time.Now().Add(-time.Minute))
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 5)
		metadatatest.AssertEqualKafkaExporterLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[int64]{
				{
					Attributes:   attribute.NewSet(attribute.String("outcome", "success")),
					Count:        1,
					Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
					Min:          metricdata.NewExtrema[int64](60000),
					Max:          metricdata.NewExtrema[int64](60000),
					Sum:          60000,
				},
			},
			metricdatatest.IgnoreTimestamp(),
			metricdatatest.IgnoreValue(),
		)
		metadatatest.AssertEqualKafkaExporterWriteLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[float64]{
				{
					Attributes:   attribute.NewSet(attribute.String("outcome", "success")),
					Count:        1,
					Bounds:       []float64{0, 0.005, 0.010, 0.025, 0.050, 0.075, 0.100, 0.250, 0.500, 0.750, 1, 2.5, 5, 7.5, 10, 25, 50, 75, 100},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0},
					Min:          metricdata.NewExtrema(60.0),
					Max:          metricdata.NewExtrema(60.0),
					Sum:          60.0,
				},
			},
			metricdatatest.IgnoreTimestamp(),
			metricdatatest.IgnoreValue(),
		)
		metadatatest.AssertEqualKafkaExporterMessages(
			t,
			testTel,
			[]metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 2,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"),
						attribute.String("topic", "bar"),
						attribute.Int("partition", 0),
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
						attribute.String("outcome", "success"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 2,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"),
						attribute.String("topic", "bar"),
						attribute.Int("partition", 0),
					),
					Value: 1,
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
						attribute.String("outcome", "success"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 80,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"),
						attribute.String("topic", "bar"),
						attribute.Int("partition", 0),
					),
					Value: 40,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
	t.Run("should only report failure outcome for metrics when unknown error provided", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		spm := NewSaramaProducerMetrics(tb)
		spm.ReportProducerMetrics(context.TODO(), makeSaramaMessages(Messages{
			TopicMessages: []TopicMessages{
				{Topic: "foo", Messages: []marshaler.Message{
					{Key: []byte("k1"), Value: []byte("v1")},
					{Key: []byte("k2"), Value: []byte("v2")},
				}},
				{Topic: "bar", Messages: []marshaler.Message{
					{Key: []byte("k1"), Value: []byte("v1")},
				}},
			},
		}), errors.New("unknown"), time.Now().Add(-time.Minute))
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 5)
		metadatatest.AssertEqualKafkaExporterLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[int64]{
				{
					Attributes:   attribute.NewSet(attribute.String("outcome", "failure")),
					Count:        1,
					Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
					Min:          metricdata.NewExtrema[int64](60000),
					Max:          metricdata.NewExtrema[int64](60000),
					Sum:          60000,
				},
			},
			metricdatatest.IgnoreTimestamp(),
			metricdatatest.IgnoreValue(),
		)
		metadatatest.AssertEqualKafkaExporterWriteLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[float64]{
				{
					Attributes:   attribute.NewSet(attribute.String("outcome", "failure")),
					Count:        1,
					Bounds:       []float64{0, 0.005, 0.010, 0.025, 0.050, 0.075, 0.100, 0.250, 0.500, 0.750, 1, 2.5, 5, 7.5, 10, 25, 50, 75, 100},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0},
					Min:          metricdata.NewExtrema(60.0),
					Max:          metricdata.NewExtrema(60.0),
					Sum:          60.0,
				},
			},
			metricdatatest.IgnoreTimestamp(),
			metricdatatest.IgnoreValue(),
		)
		metadatatest.AssertEqualKafkaExporterMessages(
			t,
			testTel,
			[]metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "failure"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 2,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "failure"),
						attribute.String("topic", "bar"),
						attribute.Int("partition", 0),
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
						attribute.String("outcome", "failure"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 2,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "failure"),
						attribute.String("topic", "bar"),
						attribute.Int("partition", 0),
					),
					Value: 1,
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
						attribute.String("outcome", "failure"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 80,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "failure"),
						attribute.String("topic", "bar"),
						attribute.Int("partition", 0),
					),
					Value: 40,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
	t.Run("should report partial failure outcome for metrics when sarama.ProducerErrors error provided", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		defer tb.Shutdown()
		spm := NewSaramaProducerMetrics(tb)
		spm.ReportProducerMetrics(context.TODO(), makeSaramaMessages(Messages{
			TopicMessages: []TopicMessages{
				{Topic: "foo", Messages: []marshaler.Message{
					{Key: []byte("k1"), Value: []byte("v1")},
					{Key: []byte("k2"), Value: []byte("v2")},
				}},
				{Topic: "bar", Messages: []marshaler.Message{
					{Key: []byte("k1"), Value: []byte("v1")},
				}},
			},
		}), sarama.ProducerErrors{
			{
				Msg: &sarama.ProducerMessage{
					Topic: "foo",
					Key:   sarama.ByteEncoder([]byte("k1")),
					Value: sarama.ByteEncoder([]byte("v1")),
				},
			},
		}, time.Now().Add(-time.Minute))
		var rm metricdata.ResourceMetrics
		err = testTel.Reader.Collect(context.Background(), &rm)
		require.NoError(t, err)
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 5)
		metadatatest.AssertEqualKafkaExporterLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[int64]{
				{
					Attributes:   attribute.NewSet(attribute.String("outcome", "failure")),
					Count:        1,
					Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
					Min:          metricdata.NewExtrema[int64](60000),
					Max:          metricdata.NewExtrema[int64](60000),
					Sum:          60000,
				},
			},
			metricdatatest.IgnoreTimestamp(),
			metricdatatest.IgnoreValue(),
		)
		metadatatest.AssertEqualKafkaExporterWriteLatency(
			t,
			testTel,
			[]metricdata.HistogramDataPoint[float64]{
				{
					Attributes:   attribute.NewSet(attribute.String("outcome", "failure")),
					Count:        1,
					Bounds:       []float64{0, 0.005, 0.010, 0.025, 0.050, 0.075, 0.100, 0.250, 0.500, 0.750, 1, 2.5, 5, 7.5, 10, 25, 50, 75, 100},
					BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0},
					Min:          metricdata.NewExtrema(60.0),
					Max:          metricdata.NewExtrema(60.0),
					Sum:          60.0,
				},
			},
			metricdatatest.IgnoreTimestamp(),
			metricdatatest.IgnoreValue(),
		)
		metadatatest.AssertEqualKafkaExporterMessages(
			t,
			testTel,
			[]metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 1,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "failure"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 1,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"),
						attribute.String("topic", "bar"),
						attribute.Int("partition", 0),
					),
					Value: 1,
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
						attribute.String("outcome", "success"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 40,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "failure"),
						attribute.String("topic", "foo"),
						attribute.Int("partition", 0),
					),
					Value: 40,
				},
				{
					Attributes: attribute.NewSet(
						attribute.String("outcome", "success"),
						attribute.String("topic", "bar"),
						attribute.Int("partition", 0),
					),
					Value: 40,
				},
			},
			metricdatatest.IgnoreTimestamp(),
		)
	})
}
