// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
)

var batchSizes = []int{1, 10}

// configureExporterBench is like configureExporter but wires up
// FranzProducerMetrics hooks so the metric hot path is exercised.
func configureExporterBench[T any](
	b *testing.B,
	exp *kafkaExporter[T],
	cfg Config,
	topics ...string,
) {
	cluster, kcfg := kafkatest.NewCluster(b, kfake.SeedTopics(1, topics...))
	_ = cluster

	tb, err := metadata.NewTelemetryBuilder(exp.set.TelemetrySettings)
	require.NoError(b, err)
	exp.tb = tb

	client, err := kafka.NewFranzSyncProducer(
		b.Context(), componenttest.NewNopHost(), kcfg,
		cfg.Producer, 1*time.Second, zap.NewNop(),
		kgo.SeedBrokers(kcfg.Brokers...),
		kgo.WithHooks(kafkaclient.NewFranzProducerMetrics(tb)),
	)
	require.NoError(b, err)

	messenger, err := exp.newMessenger(componenttest.NewNopHost())
	require.NoError(b, err)
	exp.messenger = messenger
	exp.producer = kafkaclient.NewFranzSyncProducer(client, cfg.IncludeMetadataKeys)

	b.Cleanup(func() { exp.Close(b.Context()) })
}

func BenchmarkTracesExporter(b *testing.B) {
	const topic = "otlp_spans"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newTracesExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			data := testdata.GenerateTraces(size)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					require.NoError(b, exp.exportData(b.Context(), data))
				}
			})
		})
	}
}

func BenchmarkLogsExporter(b *testing.B) {
	const topic = "otlp_logs"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newLogsExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			data := testdata.GenerateLogs(size)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					require.NoError(b, exp.exportData(b.Context(), data))
				}
			})
		})
	}
}

func BenchmarkMetricsExporter(b *testing.B) {
	const topic = "otlp_metrics"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newMetricsExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			data := testdata.GenerateMetrics(size)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					require.NoError(b, exp.exportData(b.Context(), data))
				}
			})
		})
	}
}

func BenchmarkTracesExporterSeq(b *testing.B) {
	const topic = "otlp_spans"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newTracesExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			data := testdata.GenerateTraces(size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = exp.exportData(b.Context(), data)
			}
		})
	}
}

func BenchmarkLogsExporterSeq(b *testing.B) {
	const topic = "otlp_logs"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newLogsExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			data := testdata.GenerateLogs(size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = exp.exportData(b.Context(), data)
			}
		})
	}
}

func BenchmarkMetricsExporterSeq(b *testing.B) {
	const topic = "otlp_metrics"
	set := exportertest.NewNopSettings(metadata.Type)
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", size), func(b *testing.B) {
			cfg := createDefaultConfig().(*Config)
			exp := newMetricsExporter(*cfg, set)
			configureExporterBench(b, exp, *cfg, topic)
			data := testdata.GenerateMetrics(size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = exp.exportData(b.Context(), data)
			}
		})
	}
}
