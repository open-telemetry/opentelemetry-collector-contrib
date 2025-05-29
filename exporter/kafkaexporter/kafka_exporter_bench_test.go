// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"context"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
)

var clientType = func() string {
	if ct := os.Getenv("KAFKA_CLIENT_TYPE"); ct == "" {
		return ct // Use the value from the environment variable
	}
	return clientTypeSarama // Default to Sarama if not set
}()

// NOTE(marclop) These benchmarks target the default localhost:9092 Kafka broker.
// You can run them against a real Kafka cluster or a docker container:
// docker run --rm -d -p 9092:9092 --name broker apache/kafka:4.0.0
// docker exec --workdir /opt/kafka/bin/ -it broker ./kafka-topics.sh --bootstrap-server localhost:9092 --create --topic otlp_logs
// docker exec --workdir /opt/kafka/bin/ -it broker ./kafka-topics.sh --bootstrap-server localhost:9092 --create --topic otlp_metrics
// docker exec --workdir /opt/kafka/bin/ -it broker ./kafka-topics.sh --bootstrap-server localhost:9092 --create --topic otlp_spans

func BenchmarkLogs(b *testing.B) {
	runBenchmarkLogs(b, clientType)
}

func runBenchmarkLogs(b *testing.B, clientType string) {
	config := createDefaultConfig().(*Config)
	config.ProtocolVersion = "2.3.0"
	config.ClientType = clientType

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
	runBenchmarkMetrics(b, clientType)
}

func runBenchmarkMetrics(b *testing.B, clientType string) {
	config := createDefaultConfig().(*Config)
	config.ProtocolVersion = "2.3.0"
	config.ClientType = clientType

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
	runBenchmarkTraces(b, clientType)
}

func runBenchmarkTraces(b *testing.B, clientType string) {
	config := createDefaultConfig().(*Config)
	config.ProtocolVersion = "2.3.0"
	config.ClientType = clientType

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
