// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
)

func configureBenchmark(tb testing.TB) {
	if os.Getenv("BENCHMARK_KAFKA") == "" {
		tb.Skip("Skipping Kafka benchmarks, set BENCHMARK_KAFKA to any value run them, and optionally USE_FRANZ_GO to use franz-go client")
	}
	tb.Helper()
	if ct := os.Getenv("USE_FRANZ_GO"); ct != "" {
		require.NoError(tb,
			featuregate.GlobalRegistry().Set(franzGoClientFeatureGateName, true),
		)
		tb.Cleanup(func() {
			require.NoError(tb,
				featuregate.GlobalRegistry().Set(franzGoClientFeatureGateName, false),
			)
		})
	}
}

// NOTE(marclop) These benchmarks target the default localhost:9092 Kafka broker.
// You can run them against a real Kafka cluster or a docker container:
// docker run --rm -d -p 9092:9092 --name broker apache/kafka:4.0.0
// docker exec --workdir /opt/kafka/bin/ -it broker ./kafka-topics.sh --bootstrap-server localhost:9092 --create --topic otlp_logs
// docker exec --workdir /opt/kafka/bin/ -it broker ./kafka-topics.sh --bootstrap-server localhost:9092 --create --topic otlp_metrics
// docker exec --workdir /opt/kafka/bin/ -it broker ./kafka-topics.sh --bootstrap-server localhost:9092 --create --topic otlp_spans

func BenchmarkLogs(b *testing.B) {
	configureBenchmark(b)
	runBenchmarkLogs(b)
}

func runBenchmarkLogs(b *testing.B) {
	config := createDefaultConfig().(*Config)
	config.ProtocolVersion = "2.3.0"

	exp := newLogsExporter(*config, exportertest.NewNopSettings(metadata.Type))
	b.Cleanup(func() { exp.Close(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := exp.Start(ctx, componenttest.NewNopHost())
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		data := testdata.GenerateLogs(10)
		for p.Next() {
			err := exp.exportData(ctx, data)
			require.NoError(b, err)
		}
	})
}

func BenchmarkMetrics(b *testing.B) {
	configureBenchmark(b)
	runBenchmarkMetrics(b)
}

func runBenchmarkMetrics(b *testing.B) {
	config := createDefaultConfig().(*Config)
	config.ProtocolVersion = "2.3.0"

	exp := newMetricsExporter(*config, exportertest.NewNopSettings(metadata.Type))
	b.Cleanup(func() { exp.Close(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := exp.Start(ctx, componenttest.NewNopHost())
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		data := testdata.GenerateMetrics(10)
		for p.Next() {
			err := exp.exportData(ctx, data)
			require.NoError(b, err)
		}
	})
}

func BenchmarkTraces(b *testing.B) {
	configureBenchmark(b)
	runBenchmarkTraces(b)
}

func runBenchmarkTraces(b *testing.B) {
	config := createDefaultConfig().(*Config)
	config.ProtocolVersion = "2.3.0"

	exp := newTracesExporter(*config, exportertest.NewNopSettings(metadata.Type))
	b.Cleanup(func() { exp.Close(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := exp.Start(ctx, componenttest.NewNopHost())
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		data := testdata.GenerateTraces(10)
		for p.Next() {
			err := exp.exportData(ctx, data)
			require.NoError(b, err)
		}
	})
}
