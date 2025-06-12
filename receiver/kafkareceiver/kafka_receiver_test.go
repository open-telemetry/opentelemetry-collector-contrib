// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadatatest"
)

func init() {
	// Disable the go-metrics registry, as there's a goroutine leak in the Sarama
	// code that uses it. See this stale issue: https://github.com/IBM/sarama/issues/1321
	//
	// Sarama docs suggest setting UseNilMetrics to true to disable metrics if they
	// are not needed, which is the case here. We only disable in tests to avoid
	// affecting other components that rely on go-metrics.
	metrics.UseNilMetrics = true
}

func TestReceiver(t *testing.T) {
	t.Parallel()
	kafkaClient, receiverConfig := mustNewFakeCluster(t, kfake.SeedTopics(1, "otlp_spans"))

	// Send some traces to the otlp_spans topic.
	traces := testdata.GenerateTraces(5)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	results := kafkaClient.ProduceSync(context.Background(), &kgo.Record{
		Topic: "otlp_spans",
		Value: data,
	})
	require.NoError(t, results.FirstErr())

	// Wait for message to be consumed.
	received := make(chan consumerArgs[ptrace.Traces], 1)
	mustNewTracesReceiver(t, receiverConfig, newChannelTracesConsumer(received))
	args := <-received
	assert.NoError(t, ptracetest.CompareTraces(traces, args.data))
}

func TestReceiver_Headers_Metadata(t *testing.T) {
	t.Parallel()
	for name, testcase := range map[string]struct {
		headers  []kgo.RecordHeader
		expected map[string][]string
	}{
		"no headers": {},
		"single header": {
			headers: []kgo.RecordHeader{
				{Key: "key1", Value: []byte("value1")},
			},
			expected: map[string][]string{
				"key1": {"value1"},
			},
		},
		"multiple headers": {
			headers: []kgo.RecordHeader{
				{Key: "key1", Value: []byte("value1")},
				{Key: "key2", Value: []byte("value2")},
			},
			expected: map[string][]string{
				"key1": {"value1"},
				"key2": {"value2"},
			},
		},
		"single header multiple values": {
			headers: []kgo.RecordHeader{
				{Key: "key1", Value: []byte("value1")},
				{Key: "key1", Value: []byte("value2")},
			},
			expected: map[string][]string{
				"key1": {"value1", "value2"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			kafkaClient, receiverConfig := mustNewFakeCluster(t, kfake.SeedTopics(1, "otlp_spans"))

			// Send some traces to the otlp_spans topic, including headers.
			traces := testdata.GenerateTraces(1)
			data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
			require.NoError(t, err)
			results := kafkaClient.ProduceSync(context.Background(), &kgo.Record{
				Topic:   "otlp_spans",
				Value:   data,
				Headers: testcase.headers,
			})
			require.NoError(t, results.FirstErr())

			// Wait for message to be consumed.
			received := make(chan consumerArgs[ptrace.Traces], 1)
			mustNewTracesReceiver(t, receiverConfig, newChannelTracesConsumer(received))
			args := <-received
			info := client.FromContext(args.ctx)
			for key, values := range testcase.expected {
				assert.Equal(t, values, info.Metadata.Get(key))
			}
		})
	}
}

func TestReceiver_Headers_HeaderExtraction(t *testing.T) {
	t.Parallel()
	for _, enabled := range []bool{false, true} {
		name := "enabled"
		if !enabled {
			name = "disabled"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			kafkaClient, receiverConfig := mustNewFakeCluster(t, kfake.SeedTopics(1, "otlp_spans"))

			// Send some traces to the otlp_spans topic, including headers.
			traces := testdata.GenerateTraces(1)
			data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
			require.NoError(t, err)
			results := kafkaClient.ProduceSync(context.Background(), &kgo.Record{
				Topic: "otlp_spans",
				Value: data,
				Headers: []kgo.RecordHeader{{
					Key:   "extracted",
					Value: []byte("value1"),
				}, {
					Key:   "extracted",
					Value: []byte("value2"),
				}, {
					Key:   "not_extracted",
					Value: []byte("value3"),
				}},
			})
			require.NoError(t, results.FirstErr())

			// Wait for message to be consumed.
			received := make(chan consumerArgs[ptrace.Traces], 1)
			receiverConfig.HeaderExtraction.ExtractHeaders = enabled
			receiverConfig.HeaderExtraction.Headers = []string{"extracted"}
			mustNewTracesReceiver(t, receiverConfig, newChannelTracesConsumer(received))
			args := <-received

			resource := args.data.ResourceSpans().At(0).Resource()
			value, ok := resource.Attributes().Get("kafka.header.extracted")
			if enabled {
				require.True(t, ok)
				assert.Equal(t, "value1", value.Str()) // only first value is extracted
			} else {
				require.False(t, ok)
			}
		})
	}
}

func TestReceiver_ConsumeError(t *testing.T) {
	t.Parallel()
	for name, testcase := range map[string]struct {
		err         error
		shouldRetry bool
	}{
		"retryable error": {
			err:         exporterhelper.ErrQueueIsFull,
			shouldRetry: true,
		},
		"permanent error": {
			err: consumererror.NewPermanent(errors.New("failed to consume")),
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			kafkaClient, receiverConfig := mustNewFakeCluster(t, kfake.SeedTopics(1, "otlp_spans"))

			// Send some traces to the otlp_spans topic.
			traces := testdata.GenerateTraces(1)
			data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
			require.NoError(t, err)
			results := kafkaClient.ProduceSync(context.Background(),
				&kgo.Record{Topic: "otlp_spans", Value: data},
			)
			require.NoError(t, results.FirstErr())

			var calls atomic.Int64
			consumer := newTracesConsumer(func(context.Context, ptrace.Traces) error {
				calls.Add(1)
				return testcase.err
			})

			// Wait for messages to be consumed.
			receiverConfig.ErrorBackOff.Enabled = true
			receiverConfig.ErrorBackOff.InitialInterval = 10 * time.Millisecond
			receiverConfig.ErrorBackOff.MaxInterval = 10 * time.Millisecond
			receiverConfig.ErrorBackOff.MaxElapsedTime = 500 * time.Millisecond
			mustNewTracesReceiver(t, receiverConfig, consumer)

			if testcase.shouldRetry {
				assert.Eventually(
					t, func() bool { return calls.Load() > 1 },
					10*time.Second, 100*time.Millisecond,
				)
			} else {
				assert.Eventually(
					t, func() bool { return calls.Load() == 1 },
					10*time.Second, 100*time.Millisecond,
				)
				// Verify that no retries have been attempted.
				time.Sleep(100 * time.Millisecond)
				assert.Equal(t, int64(1), calls.Load())
			}
		})
	}
}

func TestReceiver_InternalTelemetry(t *testing.T) {
	t.Parallel()
	kafkaClient, receiverConfig := mustNewFakeCluster(t, kfake.SeedTopics(1, "otlp_spans"))

	// Send some traces to the otlp_spans topic.
	traces := testdata.GenerateTraces(1)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	results := kafkaClient.ProduceSync(context.Background(),
		&kgo.Record{Topic: "otlp_spans", Value: data},
		&kgo.Record{Topic: "otlp_spans", Value: data},
		&kgo.Record{Topic: "otlp_spans", Value: data},
		&kgo.Record{Topic: "otlp_spans", Value: data},
		&kgo.Record{Topic: "otlp_spans", Value: []byte("junk")},
	)
	require.NoError(t, results.FirstErr())

	// Wait for messages to be consumed.
	received := make(chan consumerArgs[ptrace.Traces], 1)
	set, tel, observedLogs := mustNewSettings(t)
	f := NewFactory()
	r, err := f.CreateTraces(context.Background(), set, receiverConfig, newChannelTracesConsumer(received))
	require.NoError(t, err)
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		assert.NoError(t, r.Shutdown(context.Background()))
	})
	for range 4 {
		<-received
	}

	// There should be one failed message due to the invalid third message payload.
	// It may not be available immediately, as the receiver may not have processed it yet.
	assert.Eventually(t, func() bool {
		_, getMetricErr := tel.GetMetric("otelcol_kafka_receiver_unmarshal_failed_spans")
		return getMetricErr == nil
	}, 10*time.Second, 100*time.Millisecond)
	metadatatest.AssertEqualKafkaReceiverUnmarshalFailedSpans(t, tel, []metricdata.DataPoint[int64]{{
		Value: 1,
		Attributes: attribute.NewSet(
			attribute.String("name", set.ID.String()),
			attribute.String("topic", "otlp_spans"),
			attribute.String("partition", "0"),
		),
	}}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())

	// After receiving messages, the internal metrics should be updated.
	metadatatest.AssertEqualKafkaReceiverPartitionStart(t, tel, []metricdata.DataPoint[int64]{{
		Value:      1,
		Attributes: attribute.NewSet(attribute.String("name", set.ID.Name())),
	}}, metricdatatest.IgnoreTimestamp())

	metadatatest.AssertEqualKafkaReceiverMessages(t, tel, []metricdata.DataPoint[int64]{{
		Value: 5,
		Attributes: attribute.NewSet(
			attribute.String("name", set.ID.String()),
			attribute.String("topic", "otlp_spans"),
			attribute.String("partition", "0"),
		),
	}}, metricdatatest.IgnoreTimestamp())

	// Shut down and check that the partition close metric is updated.
	err = r.Shutdown(context.Background())
	require.NoError(t, err)
	metadatatest.AssertEqualKafkaReceiverPartitionClose(t, tel, []metricdata.DataPoint[int64]{{
		Value: 1,
		Attributes: attribute.NewSet(
			attribute.String("name", set.ID.Name()),
		),
	}}, metricdatatest.IgnoreTimestamp())

	observedErrorLogs := observedLogs.FilterLevelExact(zapcore.ErrorLevel)
	logEntries := observedErrorLogs.All()
	assert.Len(t, logEntries, 2)
	assert.Equal(t, "failed to unmarshal message", logEntries[0].Message)
	assert.Equal(t, "failed to consume message, skipping due to message_marking config", logEntries[1].Message)

	metadatatest.AssertEqualKafkaReceiverCurrentOffset(t, tel, []metricdata.DataPoint[int64]{{
		Value: 4, // offset of the final message
		Attributes: attribute.NewSet(
			attribute.String("name", set.ID.String()),
			attribute.String("topic", "otlp_spans"),
			attribute.String("partition", "0"),
		),
	}}, metricdatatest.IgnoreTimestamp())

	metadatatest.AssertEqualKafkaReceiverOffsetLag(t, tel, []metricdata.DataPoint[int64]{{
		Value: 0,
		Attributes: attribute.NewSet(
			attribute.String("name", set.ID.String()),
			attribute.String("topic", "otlp_spans"),
			attribute.String("partition", "0"),
		),
	}}, metricdatatest.IgnoreTimestamp())
}

func TestReceiver_MessageMarking(t *testing.T) {
	t.Parallel()
	for name, testcase := range map[string]struct {
		markAfter  bool
		markErrors bool

		errorShouldRestart bool
	}{
		"mark_before": {
			markAfter: false,
		},
		"mark_after_success": {
			markAfter:          true,
			errorShouldRestart: true,
		},
		"mark_after_all": {
			markAfter:  true,
			markErrors: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			kafkaClient, receiverConfig := mustNewFakeCluster(t, kfake.SeedTopics(1, "otlp_spans"))

			// Send some invalid data to the otlp_spans topic so unmarshaling fails,
			// and then send some valid data to show that the invalid data does not
			// block the consumer.
			traces := testdata.GenerateTraces(1)
			data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
			require.NoError(t, err)
			results := kafkaClient.ProduceSync(context.Background(),
				&kgo.Record{Topic: "otlp_spans", Value: []byte("junk")},
				&kgo.Record{Topic: "otlp_spans", Value: data},
			)
			require.NoError(t, results.FirstErr())

			var calls atomic.Int64
			consumer := newTracesConsumer(func(_ context.Context, received ptrace.Traces) error {
				calls.Add(1)
				return ptracetest.CompareTraces(traces, received)
			})

			// Only mark messages after consuming, including for errors.
			receiverConfig.MessageMarking.After = testcase.markAfter
			receiverConfig.MessageMarking.OnError = testcase.markErrors
			set, tel, observedLogs := mustNewSettings(t)
			f := NewFactory()
			r, err := f.CreateTraces(context.Background(), set, receiverConfig, consumer)
			require.NoError(t, err)
			require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
			t.Cleanup(func() {
				assert.NoError(t, r.Shutdown(context.Background()))
			})

			if testcase.errorShouldRestart {
				// Verify that the consumer restarts at least once.
				assert.Eventually(t, func() bool {
					m, err := tel.GetMetric("otelcol_kafka_receiver_partition_start")
					require.NoError(t, err)

					dataPoints := m.Data.(metricdata.Sum[int64]).DataPoints
					assert.Len(t, dataPoints, 1)
					return dataPoints[0].Value > 5
				}, time.Second, 100*time.Millisecond, "unmarshal error should restart consumer")

				// The invalid message should block the consumer.
				assert.Zero(t, calls.Load())

				observedErrorLogs := observedLogs.FilterLevelExact(zapcore.ErrorLevel)
				logEntries := observedErrorLogs.All()
				require.NotEmpty(t, logEntries)
				for _, entry := range logEntries {
					assert.Equal(t, "failed to unmarshal message", entry.Message)
				}
			} else {
				assert.Eventually(t, func() bool {
					return calls.Load() == 1
				}, time.Second, 100*time.Millisecond, "unmarshal error should not block consumption")

				// Verify that the consumer did not restart.
				metadatatest.AssertEqualKafkaReceiverPartitionStart(t, tel, []metricdata.DataPoint[int64]{{
					Value:      1,
					Attributes: attribute.NewSet(attribute.String("name", set.ID.Name())),
				}}, metricdatatest.IgnoreTimestamp())

				observedErrorLogs := observedLogs.FilterLevelExact(zapcore.ErrorLevel)
				logEntries := observedErrorLogs.All()
				require.Len(t, logEntries, 2)
				assert.Equal(t, "failed to unmarshal message", logEntries[0].Message)
				assert.Equal(t,
					"failed to consume message, skipping due to message_marking config",
					logEntries[1].Message,
				)
			}
		})
	}
}

func TestNewLogsReceiver(t *testing.T) {
	t.Parallel()
	kafkaClient, receiverConfig := mustNewFakeCluster(t, kfake.SeedTopics(1, "otlp_logs"))

	var sink consumertest.LogsSink
	receiverConfig.HeaderExtraction.ExtractHeaders = true
	receiverConfig.HeaderExtraction.Headers = []string{"key1"}
	set, tel, _ := mustNewSettings(t)
	r, err := newLogsReceiver(receiverConfig, set, &sink)
	require.NoError(t, err)

	// Send some logs to the otlp_logs topic.
	logs := testdata.GenerateLogs(1)
	data, err := (&plog.ProtoMarshaler{}).MarshalLogs(logs)
	require.NoError(t, err)
	results := kafkaClient.ProduceSync(context.Background(),
		&kgo.Record{
			Topic: "otlp_logs",
			Value: data,
			Headers: []kgo.RecordHeader{
				{Key: "key1", Value: []byte("value1")},
			},
		},
		&kgo.Record{Topic: "otlp_logs", Value: []byte("junk")},
	)
	require.NoError(t, results.FirstErr())

	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, r.Shutdown(context.Background()))
	})

	// There should be one failed message due to the invalid message payload.
	// It may not be available immediately, as the receiver may not have processed it yet.
	assert.Eventually(t, func() bool {
		_, err := tel.GetMetric("otelcol_kafka_receiver_unmarshal_failed_log_records")
		return err == nil
	}, 10*time.Second, 100*time.Millisecond)
	metadatatest.AssertEqualKafkaReceiverUnmarshalFailedLogRecords(t, tel, []metricdata.DataPoint[int64]{{
		Value: 1,
		Attributes: attribute.NewSet(
			attribute.String("name", set.ID.String()),
			attribute.String("topic", "otlp_logs"),
			attribute.String("partition", "0"),
		),
	}}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())

	// There should be one successfully processed batch of logs.
	assert.Len(t, sink.AllLogs(), 1)
	_, ok := sink.AllLogs()[0].ResourceLogs().At(0).Resource().Attributes().Get("kafka.header.key1")
	require.True(t, ok)
}

func TestNewMetricsReceiver(t *testing.T) {
	t.Parallel()
	kafkaClient, receiverConfig := mustNewFakeCluster(t, kfake.SeedTopics(1, "otlp_metrics"))

	var sink consumertest.MetricsSink
	receiverConfig.HeaderExtraction.ExtractHeaders = true
	receiverConfig.HeaderExtraction.Headers = []string{"key1"}
	set, tel, _ := mustNewSettings(t)
	r, err := newMetricsReceiver(receiverConfig, set, &sink)
	require.NoError(t, err)

	// Send some metrics to the otlp_metrics topic.
	metrics := testdata.GenerateMetrics(1)
	data, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(metrics)
	require.NoError(t, err)
	results := kafkaClient.ProduceSync(context.Background(),
		&kgo.Record{
			Topic: "otlp_metrics",
			Value: data,
			Headers: []kgo.RecordHeader{
				{Key: "key1", Value: []byte("value1")},
			},
		},
		&kgo.Record{Topic: "otlp_metrics", Value: []byte("junk")},
	)
	require.NoError(t, results.FirstErr())

	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, r.Shutdown(context.Background()))
	})

	// There should be one failed message due to the invalid message payload.
	// It may not be available immediately, as the receiver may not have processed it yet.
	assert.Eventually(t, func() bool {
		_, err := tel.GetMetric("otelcol_kafka_receiver_unmarshal_failed_metric_points")
		return err == nil
	}, 10*time.Second, 100*time.Millisecond)
	metadatatest.AssertEqualKafkaReceiverUnmarshalFailedMetricPoints(t, tel, []metricdata.DataPoint[int64]{{
		Value: 1,
		Attributes: attribute.NewSet(
			attribute.String("name", set.ID.String()),
			attribute.String("topic", "otlp_metrics"),
			attribute.String("partition", "0"),
		),
	}}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())

	// There should be one successfully processed batch of metrics.
	assert.Len(t, sink.AllMetrics(), 1)
	_, ok := sink.AllMetrics()[0].ResourceMetrics().At(0).Resource().Attributes().Get("kafka.header.key1")
	require.True(t, ok)
}

func mustNewTracesReceiver(tb testing.TB, cfg *Config, nextConsumer consumer.Traces) {
	tb.Helper()

	f := NewFactory()
	r, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nextConsumer)
	require.NoError(tb, err)
	require.NoError(tb, r.Start(context.Background(), componenttest.NewNopHost()))
	tb.Cleanup(func() {
		assert.NoError(tb, r.Shutdown(context.Background()))
	})
}

func mustNewSettings(tb testing.TB) (receiver.Settings, *componenttest.Telemetry, *observer.ObservedLogs) {
	zapCore, observedLogs := observer.New(zapcore.DebugLevel)
	set := receivertest.NewNopSettings(metadata.Type)
	tel := componenttest.NewTelemetry()
	tb.Cleanup(func() {
		assert.NoError(tb, tel.Shutdown(context.Background()))
	})
	set.TelemetrySettings = tel.NewTelemetrySettings()
	set.Logger = zap.New(zapCore)
	return set, tel, observedLogs
}

// consumerArgs holds the context and data passed to the consumer function.
type consumerArgs[T any] struct {
	ctx  context.Context
	data T
}

func newChannelTracesConsumer(ch chan<- consumerArgs[ptrace.Traces]) consumer.Traces {
	return newTracesConsumer(func(ctx context.Context, data ptrace.Traces) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- consumerArgs[ptrace.Traces]{ctx: ctx, data: data}:
		}
		return nil
	})
}

func newTracesConsumer(f consumer.ConsumeTracesFunc) consumer.Traces {
	consumer, _ := consumer.NewTraces(f)
	return consumer
}

// mustNewFakeCluster creates a new fake Kafka cluster with the given options,
// and returns a kgo.Client for operating on the cluster, and a receiver config.
func mustNewFakeCluster(tb testing.TB, opts ...kfake.Opt) (*kgo.Client, *Config) {
	cluster, clientConfig := kafkatest.NewCluster(tb, opts...)
	kafkaClient := mustNewClient(tb, cluster)
	tb.Cleanup(func() { deleteConsumerGroups(tb, kafkaClient) })

	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig = clientConfig
	cfg.InitialOffset = "earliest"
	cfg.MaxFetchWait = 10 * time.Millisecond
	return kafkaClient, cfg
}

func mustNewClient(tb testing.TB, cluster *kfake.Cluster) *kgo.Client {
	client, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	require.NoError(tb, err)
	tb.Cleanup(client.Close)
	return client
}

// deleteConsumerGroups deletes all consumer groups in the cluster.
//
// It is necessary to call this to exit the group goroutines in the kfake cluster.
func deleteConsumerGroups(tb testing.TB, client *kgo.Client) {
	adminClient := kadm.NewClient(client)
	groups, err := adminClient.ListGroups(context.Background())
	assert.NoError(tb, err)
	_, err = adminClient.DeleteGroups(context.Background(), groups.Groups()...)
	assert.NoError(tb, err)
}
