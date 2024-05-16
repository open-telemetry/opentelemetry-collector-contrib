// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func BenchmarkExporter(b *testing.B) {
	for _, eventType := range []string{"logs", "traces"} {
		for _, tc := range []struct {
			name      string
			batchSize int
		}{
			{name: "small_batch", batchSize: 10},
			{name: "medium_batch", batchSize: 100},
			{name: "large_batch", batchSize: 1000},
			{name: "xlarge_batch", batchSize: 10000},
		} {
			b.Run(fmt.Sprintf("%s/%s", eventType, tc.name), func(b *testing.B) {
				switch eventType {
				case "logs":
					benchmarkLogs(b, tc.batchSize)
				case "traces":
					benchmarkTraces(b, tc.batchSize)
				}
			})
		}
	}
}

func benchmarkLogs(b *testing.B, batchSize int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runnerCfg := prepareBenchmark(b, batchSize)
	exporter, err := runnerCfg.factory.CreateLogsExporter(
		ctx, exportertest.NewNopCreateSettings(), runnerCfg.esCfg,
	)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		logs, _ := runnerCfg.provider.GenerateLogs()
		b.StartTimer()
		exporter.ConsumeLogs(ctx, logs)
		b.StopTimer()
	}
	b.ReportMetric(
		float64(runnerCfg.observedCount.Load())/b.Elapsed().Seconds(),
		"events/s",
	)
	require.NoError(b, exporter.Shutdown(ctx))
	require.Equal(b,
		runnerCfg.generatedCount.Load(),
		runnerCfg.observedCount.Load(),
		"failed to send all logs to backend",
	)
}

func benchmarkTraces(b *testing.B, batchSize int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runnerCfg := prepareBenchmark(b, batchSize)
	exporter, err := runnerCfg.factory.CreateTracesExporter(
		ctx, exportertest.NewNopCreateSettings(), runnerCfg.esCfg,
	)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		traces, _ := runnerCfg.provider.GenerateTraces()
		b.StartTimer()
		exporter.ConsumeTraces(ctx, traces)
		b.StopTimer()
	}
	b.ReportMetric(
		float64(runnerCfg.observedCount.Load())/b.Elapsed().Seconds(),
		"events/s",
	)
	require.NoError(b, exporter.Shutdown(ctx))
	require.Equal(b,
		runnerCfg.generatedCount.Load(),
		runnerCfg.observedCount.Load(),
		"failed to send all traces to backend",
	)
}

type benchRunnerCfg struct {
	factory  exporter.Factory
	provider testbed.DataProvider
	esCfg    *elasticsearchexporter.Config

	generatedCount atomic.Uint64
	observedCount  atomic.Uint64
}

func prepareBenchmark(
	b *testing.B,
	batchSize int,
) *benchRunnerCfg {
	b.Helper()

	cfg := &benchRunnerCfg{}
	receiver := newElasticsearchDataReceiver(b)
	cfg.provider = testbed.NewPerfTestDataProvider(testbed.LoadOptions{ItemsPerBatch: batchSize})
	cfg.provider.SetLoadGeneratorCounters(&cfg.generatedCount)

	cfg.factory = elasticsearchexporter.NewFactory()
	cfg.esCfg = cfg.factory.CreateDefaultConfig().(*elasticsearchexporter.Config)
	cfg.esCfg.Endpoints = []string{receiver.endpoint}
	cfg.esCfg.LogsIndex = TestLogsIndex
	cfg.esCfg.TracesIndex = TestTracesIndex
	cfg.esCfg.Flush.Interval = 10 * time.Millisecond
	cfg.esCfg.NumWorkers = 1

	tc, err := consumer.NewTraces(func(_ context.Context, td ptrace.Traces) error {
		cfg.observedCount.Add(uint64(td.SpanCount()))
		return nil
	})
	require.NoError(b, err)
	mc, err := consumer.NewMetrics(func(_ context.Context, md pmetric.Metrics) error {
		cfg.observedCount.Add(uint64(md.MetricCount()))
		return nil
	})
	require.NoError(b, err)
	lc, err := consumer.NewLogs(func(_ context.Context, ld plog.Logs) error {
		cfg.observedCount.Add(uint64(ld.LogRecordCount()))
		return nil
	})
	require.NoError(b, err)

	require.NoError(b, receiver.Start(tc, mc, lc))
	b.Cleanup(func() { require.NoError(b, receiver.Stop()) })

	return cfg
}
