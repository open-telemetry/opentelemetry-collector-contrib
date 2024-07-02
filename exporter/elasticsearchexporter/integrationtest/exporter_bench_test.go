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
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// BenchmarkExporterFlushItems benchmarks exporter flush triggered by flush batch size settings, e.g. min_size_items.
func BenchmarkExporterFlushItems(b *testing.B) {
	updateESCfg := func(esCfg *elasticsearchexporter.Config) {
		esCfg.BatcherConfig.MinSizeItems = 100 // has to be smaller than the smallest batch size, otherwise it will block
		esCfg.BatcherConfig.MaxSizeItems = 500
		esCfg.BatcherConfig.FlushTimeout = time.Hour
	}
	for _, eventType := range []string{"logs", "traces"} {
		for _, mappingMode := range []string{"none", "ecs", "raw"} {
			for _, tc := range []struct {
				name      string
				batchSize int
			}{
				{name: "medium_batch", batchSize: 100},
				{name: "large_batch", batchSize: 1000},
				{name: "xlarge_batch", batchSize: 10000},
			} {
				b.Run(fmt.Sprintf("%s/%s/%s", eventType, mappingMode, tc.name), func(b *testing.B) {
					switch eventType {
					case "logs":
						benchmarkLogs(b, tc.batchSize, mappingMode, updateESCfg)
					case "traces":
						benchmarkTraces(b, tc.batchSize, mappingMode, updateESCfg)
					}
				})
			}
		}
	}
}

func benchmarkLogs(b *testing.B, batchSize int, mappingMode string, updateESCfg func(*elasticsearchexporter.Config)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exporterSettings := exportertest.NewNopSettings()
	exporterSettings.TelemetrySettings.Logger = zaptest.NewLogger(b, zaptest.Level(zap.WarnLevel))
	runnerCfg := prepareBenchmark(b, batchSize, mappingMode)
	updateESCfg(runnerCfg.esCfg)
	exporter, err := runnerCfg.factory.CreateLogsExporter(
		ctx, exporterSettings, runnerCfg.esCfg,
	)
	require.NoError(b, err)
	require.NoError(b, exporter.Start(ctx, componenttest.NewNopHost()))

	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		logs, _ := runnerCfg.provider.GenerateLogs()
		b.StartTimer()
		require.NoError(b, exporter.ConsumeLogs(ctx, logs))
		b.StopTimer()
	}
	require.NoError(b, exporter.Shutdown(ctx))
	reportMetrics(b, runnerCfg)
}

func benchmarkTraces(b *testing.B, batchSize int, mappingMode string, updateESCfg func(*elasticsearchexporter.Config)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exporterSettings := exportertest.NewNopSettings()
	exporterSettings.TelemetrySettings.Logger = zaptest.NewLogger(b, zaptest.Level(zap.WarnLevel))
	runnerCfg := prepareBenchmark(b, batchSize, mappingMode)
	updateESCfg(runnerCfg.esCfg)
	exporter, err := runnerCfg.factory.CreateTracesExporter(
		ctx, exporterSettings, runnerCfg.esCfg,
	)
	require.NoError(b, err)
	require.NoError(b, exporter.Start(ctx, componenttest.NewNopHost()))

	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		traces, _ := runnerCfg.provider.GenerateTraces()
		b.StartTimer()
		require.NoError(b, exporter.ConsumeTraces(ctx, traces))
		b.StopTimer()
	}
	require.NoError(b, exporter.Shutdown(ctx))
	reportMetrics(b, runnerCfg)
}

type benchRunnerCfg struct {
	factory  exporter.Factory
	provider testbed.DataProvider
	esCfg    *elasticsearchexporter.Config

	generatedCount   atomic.Uint64
	observedDocCount atomic.Int64

	*counters
}

func prepareBenchmark(
	b *testing.B,
	batchSize int,
	mappingMode string,
) *benchRunnerCfg {
	b.Helper()

	cfg := &benchRunnerCfg{
		counters: &counters{},
	}
	// Benchmarks don't decode the bulk requests to avoid allocations to pollute the results.
	receiver := newElasticsearchDataReceiver(b, false /* DecodeBulkRequest */, cfg.counters)
	cfg.provider = testbed.NewPerfTestDataProvider(testbed.LoadOptions{ItemsPerBatch: batchSize})
	cfg.provider.SetLoadGeneratorCounters(&cfg.generatedCount)

	cfg.factory = elasticsearchexporter.NewFactory()
	cfg.esCfg = cfg.factory.CreateDefaultConfig().(*elasticsearchexporter.Config)
	cfg.esCfg.Mapping.Mode = mappingMode
	cfg.esCfg.Endpoints = []string{receiver.endpoint}
	cfg.esCfg.LogsIndex = TestLogsIndex
	cfg.esCfg.TracesIndex = TestTracesIndex
	cfg.esCfg.NumWorkers = 1
	cfg.esCfg.QueueSettings.Enabled = false

	tc, err := consumer.NewTraces(func(_ context.Context, traces ptrace.Traces) error {
		cfg.observedDocCount.Add(int64(traces.SpanCount()))
		return nil
	})
	require.NoError(b, err)
	mc, err := consumer.NewMetrics(func(_ context.Context, metrics pmetric.Metrics) error {
		cfg.observedDocCount.Add(int64(metrics.DataPointCount()))
		return nil
	})
	require.NoError(b, err)
	lc, err := consumer.NewLogs(func(_ context.Context, logs plog.Logs) error {
		cfg.observedDocCount.Add(int64(logs.LogRecordCount()))
		return nil
	})
	require.NoError(b, err)

	require.NoError(b, receiver.Start(tc, mc, lc))
	b.Cleanup(func() { require.NoError(b, receiver.Stop()) })

	return cfg
}

func reportMetrics(b *testing.B, runnerCfg *benchRunnerCfg) {
	b.ReportMetric(
		float64(runnerCfg.generatedCount.Load())/b.Elapsed().Seconds(),
		"events/s",
	)
	b.ReportMetric(
		float64(runnerCfg.observedDocCount.Load())/b.Elapsed().Seconds(),
		"docs/s",
	)
	b.ReportMetric(
		float64(runnerCfg.observedBulkRequests.Load())/b.Elapsed().Seconds(),
		"bulkReqs/s",
	)
}
