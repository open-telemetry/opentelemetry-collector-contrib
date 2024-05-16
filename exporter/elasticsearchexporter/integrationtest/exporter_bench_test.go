// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func BenchmarkLogsExporter(b *testing.B) {
	for _, tc := range []struct {
		name      string
		batchSize int
	}{
		{name: "small_batch", batchSize: 10},
		{name: "medium_batch", batchSize: 100},
		{name: "large_batch", batchSize: 1000},
		{name: "xlarge_batch", batchSize: 10000},
	} {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkLogs(b, tc.batchSize)
		})
	}
}

func benchmarkLogs(b *testing.B, batchSize int) {
	var generatedCount, observedCount atomic.Uint64

	receiver := newElasticsearchDataReceiver(b)
	factory := elasticsearchexporter.NewFactory()

	cfg := factory.CreateDefaultConfig().(*elasticsearchexporter.Config)
	cfg.Endpoints = []string{receiver.endpoint}
	cfg.Flush.Interval = 10 * time.Millisecond
	cfg.NumWorkers = 1

	exporter, err := factory.CreateLogsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		cfg,
	)
	require.NoError(b, err)

	provider := testbed.NewPerfTestDataProvider(testbed.LoadOptions{ItemsPerBatch: batchSize})
	provider.SetLoadGeneratorCounters(&generatedCount)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logsConsumer, err := consumer.NewLogs(func(_ context.Context, ld plog.Logs) error {
		observedCount.Add(uint64(ld.LogRecordCount()))
		return nil
	})
	require.NoError(b, err)

	require.NoError(b, receiver.Start(nil, nil, logsConsumer))
	defer func() { require.NoError(b, receiver.Stop()) }()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		logs, _ := provider.GenerateLogs()
		b.StartTimer()
		require.NoError(b, exporter.ConsumeLogs(ctx, logs))
	}
	require.NoError(b, exporter.Shutdown(ctx))
	require.Equal(b, generatedCount.Load(), observedCount.Load(), "failed to send all logs to backend")
}
