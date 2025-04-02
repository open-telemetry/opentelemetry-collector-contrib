// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func BenchmarkExporter(b *testing.B) {
	for _, eventType := range []string{"logs", "metrics", "traces"} {
		for _, mappingMode := range []string{"none", "ecs", "raw", "otel"} {
			for _, tc := range []struct {
				name      string
				batchSize int
			}{
				{name: "small_batch", batchSize: 10},
				{name: "medium_batch", batchSize: 100},
				{name: "large_batch", batchSize: 1000},
				{name: "xlarge_batch", batchSize: 10000},
			} {
				b.Run(fmt.Sprintf("%s/%s/%s", eventType, mappingMode, tc.name), func(b *testing.B) {
					switch eventType {
					case "logs":
						benchmarkLogs(b, tc.batchSize, mappingMode)
					case "metrics":
						benchmarkMetrics(b, tc.batchSize, mappingMode)
					case "traces":
						benchmarkTraces(b, tc.batchSize, mappingMode)
					}
				})
			}
		}
	}
}

func benchmarkLogs(b *testing.B, batchSize int, mappingMode string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exporterSettings := exportertest.NewNopSettings(metadata.Type)
	exporterSettings.TelemetrySettings.Logger = zaptest.NewLogger(b, zaptest.Level(zap.WarnLevel))
	runnerCfg := prepareBenchmark(b, batchSize, mappingMode)
	exporter, err := runnerCfg.factory.CreateLogs(
		ctx, exporterSettings, runnerCfg.esCfg,
	)
	require.NoError(b, err)
	require.NoError(b, exporter.Start(ctx, componenttest.NewNopHost()))

	logs, _ := runnerCfg.provider.GenerateLogs()
	logs.MarkReadOnly()
	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		require.NoError(b, exporter.ConsumeLogs(ctx, logs))
		b.StopTimer()
	}
	b.ReportMetric(
		float64(runnerCfg.generatedCount.Load())/b.Elapsed().Seconds(),
		"events/s",
	)
	require.NoError(b, exporter.Shutdown(ctx))
}

func benchmarkMetrics(b *testing.B, batchSize int, mappingMode string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exporterSettings := exportertest.NewNopSettings(metadata.Type)
	exporterSettings.TelemetrySettings.Logger = zaptest.NewLogger(b, zaptest.Level(zap.WarnLevel))
	runnerCfg := prepareBenchmark(b, batchSize, mappingMode)
	exporter, err := runnerCfg.factory.CreateMetrics(
		ctx, exporterSettings, runnerCfg.esCfg,
	)
	require.NoError(b, err)
	require.NoError(b, exporter.Start(ctx, componenttest.NewNopHost()))

	metrics, _ := runnerCfg.provider.GenerateMetrics()
	metrics.MarkReadOnly()
	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		require.NoError(b, exporter.ConsumeMetrics(ctx, metrics))
		b.StopTimer()
	}
	b.ReportMetric(
		float64(runnerCfg.generatedCount.Load())/b.Elapsed().Seconds(),
		"events/s",
	)
	require.NoError(b, exporter.Shutdown(ctx))
}

func benchmarkTraces(b *testing.B, batchSize int, mappingMode string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exporterSettings := exportertest.NewNopSettings(metadata.Type)
	exporterSettings.TelemetrySettings.Logger = zaptest.NewLogger(b, zaptest.Level(zap.WarnLevel))
	runnerCfg := prepareBenchmark(b, batchSize, mappingMode)
	exporter, err := runnerCfg.factory.CreateTraces(
		ctx, exporterSettings, runnerCfg.esCfg,
	)
	require.NoError(b, err)
	require.NoError(b, exporter.Start(ctx, componenttest.NewNopHost()))

	traces, _ := runnerCfg.provider.GenerateTraces()
	traces.MarkReadOnly()
	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		require.NoError(b, exporter.ConsumeTraces(ctx, traces))
		b.StopTimer()
	}
	b.ReportMetric(
		float64(runnerCfg.generatedCount.Load())/b.Elapsed().Seconds(),
		"events/s",
	)
	require.NoError(b, exporter.Shutdown(ctx))
}

type benchRunnerCfg struct {
	factory  exporter.Factory
	provider testbed.DataProvider
	esCfg    *elasticsearchexporter.Config

	generatedCount atomic.Uint64
}

func prepareBenchmark(
	b *testing.B,
	batchSize int,
	mappingMode string,
) *benchRunnerCfg {
	b.Helper()

	cfg := &benchRunnerCfg{}
	// Benchmarks don't decode the bulk requests to avoid allocations to pollute the results.
	receiver := newElasticsearchDataReceiver(b, withDecodeBulkRequest(false))
	cfg.provider = testbed.NewPerfTestDataProvider(testbed.LoadOptions{ItemsPerBatch: batchSize})
	cfg.provider.SetLoadGeneratorCounters(&cfg.generatedCount)

	cfg.factory = elasticsearchexporter.NewFactory()
	cfg.esCfg = cfg.factory.CreateDefaultConfig().(*elasticsearchexporter.Config)
	cfg.esCfg.Mapping.Mode = mappingMode
	cfg.esCfg.Endpoints = []string{receiver.endpoint}
	cfg.esCfg.LogsIndex = TestLogsIndex
	cfg.esCfg.MetricsIndex = TestMetricsIndex
	cfg.esCfg.TracesIndex = TestTracesIndex
	cfg.esCfg.Flush.Interval = 10 * time.Millisecond
	cfg.esCfg.NumWorkers = 1

	tc, err := consumer.NewTraces(func(context.Context, ptrace.Traces) error {
		return nil
	})
	require.NoError(b, err)
	mc, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error {
		return nil
	})
	require.NoError(b, err)
	lc, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
		return nil
	})
	require.NoError(b, err)

	require.NoError(b, receiver.Start(tc, mc, lc))
	b.Cleanup(func() { require.NoError(b, receiver.Stop()) })

	return cfg
}
