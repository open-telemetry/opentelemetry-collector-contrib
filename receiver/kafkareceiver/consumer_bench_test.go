// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
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
	clients        = []string{"Sarama", "Franz"}
	benchmarkCases = []struct {
		name string
		MessageMarking
		configkafka.AutoCommitConfig
	}{
		{
			name:             "AutoCommit OnError=false (default)",
			MessageMarking:   createDefaultConfig().(*Config).MessageMarking,
			AutoCommitConfig: createDefaultConfig().(*Config).AutoCommit,
		},
		{
			name:             "AutoCommit OnError=true",
			MessageMarking:   MessageMarking{After: true, OnError: true},
			AutoCommitConfig: createDefaultConfig().(*Config).AutoCommit,
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

func newBenchConfigClient(b *testing.B, topic string, partitions int32,
	autoCommit configkafka.AutoCommitConfig,
	messageMarking MessageMarking,
) (*Config, *kgo.Client) {
	client, cfg := mustNewFakeCluster(b, kfake.SeedTopics(partitions, topic))
	cfg.Logs.Topic = topic
	cfg.Traces.Topic = topic
	cfg.Metrics.Topic = topic
	cfg.GroupID = b.Name()
	cfg.InitialOffset = "earliest"
	cfg.AutoCommit = autoCommit
	cfg.MessageMarking = messageMarking
	return cfg, client
}

func runBenchmark(b *testing.B, topic string, data []byte,
	rcv component.Component, client *kgo.Client,
) {
	require.NoError(b,
		rcv.Start(context.Background(), componenttest.NewNopHost()),
	)
	defer func() { require.NoError(b, rcv.Shutdown(context.Background())) }()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client.Produce(context.Background(), &kgo.Record{
				Topic: topic, Value: data,
			}, func(_ *kgo.Record, err error) {
				require.NoError(b, err)
			})
		}
	})
	client.Flush(context.Background())
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
						setFranzGo(b, client == "Franz")
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
						setFranzGo(b, client == "Franz")
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
				suffix := fmt.Sprintf("/%s/batch_%d/partitions_%d", tc.name, size, p)
				b.Run("Sarama"+suffix, func(b *testing.B) {
					defer sink.Reset()
					setFranzGo(b, false)
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
