// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

var (
	batchSizes     = []int{1, 10}
	partitions     = []int32{1, 2}
	clients        = []string{"Franz"}
	benchmarkCases = []struct {
		name string
		MessageMarking
		configkafka.AutoCommitConfig
	}{
		{
			name:             "AutoCommit OnError=false (default)",
			MessageMarking:   createDefaultConfig().(*Config).MessageMarking,
			AutoCommitConfig: createDefaultConfig().(*Config).ConsumerConfig.AutoCommit,
		},
		{
			name:             "AutoCommit OnError=true",
			MessageMarking:   MessageMarking{After: true, OnError: true},
			AutoCommitConfig: createDefaultConfig().(*Config).ConsumerConfig.AutoCommit,
		},
		{
			name:             "After=true OnError=false",
			MessageMarking:   MessageMarking{After: true},
			AutoCommitConfig: configkafka.AutoCommitConfig{Enable: false, Interval: time.Second},
		},
		{
			name:             "After=true OnError=true",
			MessageMarking:   MessageMarking{After: true, OnError: true},
			AutoCommitConfig: configkafka.AutoCommitConfig{Enable: false, Interval: time.Second},
		},
		{
			name:             "After=false OnError=false",
			MessageMarking:   MessageMarking{After: false},
			AutoCommitConfig: configkafka.AutoCommitConfig{Enable: false, Interval: time.Second},
		},
		{
			name:             "After=false OnError=true",
			MessageMarking:   MessageMarking{After: false, OnError: true},
			AutoCommitConfig: configkafka.AutoCommitConfig{Enable: false, Interval: time.Second},
		},
	}
)

type benchmarkLogsConsumer struct {
	expected int64
	received atomic.Int64
	done     chan struct{}
	once     sync.Once
}

func newBenchmarkLogsConsumer(expected int64) *benchmarkLogsConsumer {
	return &benchmarkLogsConsumer{
		expected: expected,
		done:     make(chan struct{}),
	}
}

func (c *benchmarkLogsConsumer) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	if c.received.Add(int64(logs.LogRecordCount())) >= c.expected {
		c.once.Do(func() { close(c.done) })
	}
	return nil
}

func (*benchmarkLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func newBenchConfigClient(b *testing.B, topic string, partitions int32,
	autoCommit configkafka.AutoCommitConfig,
	messageMarking MessageMarking,
) (*Config, *kgo.Client) {
	client, cfg := mustNewFakeCluster(b, kfake.SeedTopics(partitions, topic))
	cfg.Logs.Topics = []string{topic}
	cfg.Traces.Topics = []string{topic}
	cfg.Metrics.Topics = []string{topic}
	cfg.ConsumerConfig.GroupID = b.Name()
	cfg.ConsumerConfig.InitialOffset = "earliest"
	cfg.ConsumerConfig.AutoCommit = autoCommit
	cfg.MessageMarking = messageMarking
	return cfg, client
}

func runBenchmark(b *testing.B, topic string, data []byte,
	rcv component.Component, client *kgo.Client,
) {
	require.NoError(b,
		rcv.Start(b.Context(), componenttest.NewNopHost()),
	)
	defer func() { require.NoError(b, rcv.Shutdown(b.Context())) }()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client.Produce(b.Context(), &kgo.Record{
				Topic: topic, Value: data,
			}, func(_ *kgo.Record, err error) {
				require.NoError(b, err)
			})
		}
	})
	client.Flush(b.Context())
}

// runEndToEndLogsBenchmark measures from producing b.N Kafka records until
// downstream confirms that every log was consumed. benchmarkLogsConsumer is
// used instead of consumertest.LogsSink because its completion channel avoids
// polling delay and prevents the timer from stopping while the receiver is
// still catching up.
func runEndToEndLogsBenchmark(
	b *testing.B,
	topic string,
	data []byte,
	rcv component.Component,
	client *kgo.Client,
	sink *benchmarkLogsConsumer,
	partitionCount int,
) {
	// Complete receiver startup and partition assignment before timing so group
	// coordination does not affect the steady-state comparison.
	require.NoError(b, rcv.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		b.StopTimer()
		require.NoError(b, rcv.Shutdown(b.Context()))
	}()

	franz, ok := rcv.(*franzConsumer)
	require.True(b, ok)
	require.Eventually(b, func() bool {
		franz.mu.RLock()
		defer franz.mu.RUnlock()
		return len(franz.assignments) == partitionCount
	}, 5*time.Second, 10*time.Millisecond)

	waitCtx, cancel := context.WithTimeout(b.Context(), 30*time.Second)
	defer cancel()
	// Produce callbacks run concurrently, so report their first error from the
	// benchmark goroutine instead of calling require from a callback.
	produceErr := make(chan error, 1)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client.Produce(b.Context(), &kgo.Record{
				Topic: topic,
				Value: data,
			}, func(_ *kgo.Record, err error) {
				if err != nil {
					select {
					case produceErr <- err:
					default:
					}
				}
			})
		}
	})
	if err := client.Flush(waitCtx); err != nil {
		b.StopTimer()
		require.NoError(b, err)
	}
	select {
	case err := <-produceErr:
		b.StopTimer()
		require.NoError(b, err)
	default:
	}
	// Flush only confirms that Kafka accepted every record. The completion
	// channel confirms that the receiver also delivered every log downstream.
	select {
	case <-sink.done:
	case <-waitCtx.Done():
		b.StopTimer()
		require.NoError(b, context.Cause(waitCtx))
	}
	b.StopTimer()
}

func BenchmarkTracesReceiver(b *testing.B) {
	const topic = "otlp_traces_bench"
	var marshaler ptrace.ProtoMarshaler
	logger := zaptest.NewLogger(b, zaptest.Level(zap.ErrorLevel))
	set := receivertest.NewNopSettings(metadata.Type)
	set.Logger = logger
	var sink consumertest.TracesSink

	for _, tc := range benchmarkCases {
		for _, size := range batchSizes {
			data, err := marshaler.MarshalTraces(testdata.GenerateTraces(size))
			require.NoError(b, err)
			for _, p := range partitions {
				for _, client := range clients {
					name := fmt.Sprintf("%s/%s/batch_%d/partitions_%d", client, tc.name, size, p)
					b.Run(name, func(b *testing.B) {
						defer sink.Reset()
						cfg, client := newBenchConfigClient(b, topic, p,
							tc.AutoCommitConfig, tc.MessageMarking,
						)
						rcv, err := newTracesReceiver(cfg, set, &sink)
						require.NoError(b, err)

						runBenchmark(b, topic, data, rcv, client)
						b.ReportMetric(float64(sink.SpanCount())/b.Elapsed().Seconds(), "spans/s")
					})
				}
			}
		}
	}
}

func BenchmarkLogsReceiver(b *testing.B) {
	const topic = "otlp_logs_bench"
	var marshaler plog.ProtoMarshaler
	logger := zaptest.NewLogger(b, zaptest.Level(zap.ErrorLevel))
	set := receivertest.NewNopSettings(metadata.Type)
	set.Logger = logger
	var sink consumertest.LogsSink
	for _, tc := range benchmarkCases {
		for _, size := range batchSizes {
			data, err := marshaler.MarshalLogs(testdata.GenerateLogs(size))
			require.NoError(b, err)
			for _, p := range partitions {
				for _, client := range clients {
					name := fmt.Sprintf("%s/%s/batch_%d/partitions_%d", client, tc.name, size, p)
					b.Run(name, func(b *testing.B) {
						defer sink.Reset()
						cfg, client := newBenchConfigClient(b, topic, p,
							tc.AutoCommitConfig, tc.MessageMarking,
						)
						rcv, err := newLogsReceiver(cfg, set, &sink)
						require.NoError(b, err)

						runBenchmark(b, topic, data, rcv, client)
						b.ReportMetric(float64(sink.LogRecordCount())/b.Elapsed().Seconds(), "logs/s")
					})
				}
			}
		}
	}
}

func BenchmarkLogsReceiverPartitionProcessing(b *testing.B) {
	const (
		topic          = "otlp_logs_partition_processing_bench"
		partitionCount = 4
		payloadSize    = 10
	)
	defaultConfig := createDefaultConfig().(*Config)
	// Compare steady-state logs throughput between legacy and independent
	// processing. Keeping the workload and partition count fixed isolates the
	// cost of per-partition workers and mailboxes.
	cases := []struct {
		mode                string
		autocommit          string
		autoCommitConfig    configkafka.AutoCommitConfig
		partitionProcessing PartitionProcessing
	}{
		{
			mode:             "legacy",
			autocommit:       "enabled",
			autoCommitConfig: defaultConfig.ConsumerConfig.AutoCommit,
		},
		{
			mode:             "independent",
			autocommit:       "enabled",
			autoCommitConfig: defaultConfig.ConsumerConfig.AutoCommit,
			partitionProcessing: PartitionProcessing{
				Independent:        true,
				MaxBufferedBatches: 1,
			},
		},
		{
			mode:       "legacy",
			autocommit: "disabled",
			autoCommitConfig: configkafka.AutoCommitConfig{
				Enable:   false,
				Interval: time.Second,
			},
		},
	}

	var marshaler plog.ProtoMarshaler
	data, err := marshaler.MarshalLogs(testdata.GenerateLogs(payloadSize))
	require.NoError(b, err)
	logger := zaptest.NewLogger(b, zaptest.Level(zap.ErrorLevel))
	set := receivertest.NewNopSettings(metadata.Type)
	set.Logger = logger

	for _, tc := range cases {
		b.Run(fmt.Sprintf("mode=%s/autocommit=%s", tc.mode, tc.autocommit), func(b *testing.B) {
			cfg, client := newBenchConfigClient(
				b,
				topic,
				partitionCount,
				tc.autoCommitConfig,
				defaultConfig.MessageMarking,
			)
			cfg.PartitionProcessing = tc.partitionProcessing
			expectedLogs := int64(b.N) * payloadSize
			sink := newBenchmarkLogsConsumer(expectedLogs)
			rcv, err := newLogsReceiver(cfg, set, sink)
			require.NoError(b, err)

			runEndToEndLogsBenchmark(
				b,
				topic,
				data,
				rcv,
				client,
				sink,
				partitionCount,
			)
			b.ReportMetric(float64(expectedLogs)/b.Elapsed().Seconds(), "logs/s")
		})
	}
}

func BenchmarkMetricsReceiver(b *testing.B) {
	const topic = "otlp_metrics_bench"
	var marshaler pmetric.ProtoMarshaler
	logger := zaptest.NewLogger(b, zaptest.Level(zap.ErrorLevel))
	set := receivertest.NewNopSettings(metadata.Type)
	set.Logger = logger
	var sink consumertest.MetricsSink
	for _, tc := range benchmarkCases {
		for _, size := range batchSizes {
			data, err := marshaler.MarshalMetrics(testdata.GenerateMetrics(size))
			require.NoError(b, err)
			for _, p := range partitions {
				for _, client := range clients {
					name := fmt.Sprintf("%s/%s/batch_%d/partitions_%d", client, tc.name, size, p)
					b.Run(name, func(b *testing.B) {
						defer sink.Reset()
						cfg, client := newBenchConfigClient(b, topic, p,
							tc.AutoCommitConfig, tc.MessageMarking,
						)
						rcv, err := newMetricsReceiver(cfg, set, &sink)
						require.NoError(b, err)

						runBenchmark(b, topic, data, rcv, client)
						b.ReportMetric(float64(sink.DataPointCount())/b.Elapsed().Seconds(), "metrics/s")
					})
				}
			}
		}
	}
}
