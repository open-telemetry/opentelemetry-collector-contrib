// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
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

type mockHost struct {
	component.Host
	ext map[component.ID]component.Component
}

func (nh *mockHost) GetExtensions() map[component.ID]component.Component {
	return nh.ext
}

func benchmarkLogs(b *testing.B, batchSize int) {
	var generatedCount, observedCount atomic.Uint64

	storage := filestorage.NewFactory()
	storageCfg := storage.CreateDefaultConfig().(*filestorage.Config)
	storageCfg.Directory = b.TempDir()
	componentID := component.NewIDWithName(storage.Type(), "elasticsearch")
	fileExtension, err := storage.CreateExtension(context.Background(),
		extension.CreateSettings{
			ID:                componentID,
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.NewDefaultBuildInfo(),
		},
		storageCfg)
	require.NoError(b, err)

	host := &mockHost{
		ext: map[component.ID]component.Component{
			componentID: fileExtension,
		},
	}

	require.NoError(b, fileExtension.Start(context.Background(), host))
	defer fileExtension.Shutdown(context.Background())

	receiver := newElasticsearchDataReceiver(b)
	factory := elasticsearchexporter.NewFactory()

	cfg := factory.CreateDefaultConfig().(*elasticsearchexporter.Config)
	//cfg.QueueSettings.Enabled = true
	//cfg.QueueSettings.NumConsumers = 100
	//cfg.QueueSettings.QueueSize = 100000
	cfg.PersistentQueueConfig.Enabled = true
	cfg.PersistentQueueConfig.NumConsumers = 100
	cfg.PersistentQueueConfig.QueueSize = 100000
	cfg.PersistentQueueConfig.StorageID = &componentID
	cfg.Endpoints = []string{receiver.endpoint}
	cfg.Flush.Interval = 100 * time.Millisecond
	cfg.Flush.Bytes = 125 * 300
	cfg.NumWorkers = 4

	exporter, err := factory.CreateLogsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		cfg,
	)
	require.NoError(b, err)
	exporter.Start(context.Background(), host)

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
	// FIXME: persistent queue doesn't drain on shutdown
	for {
		if observedCount.Load() >= generatedCount.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(b, exporter.Shutdown(ctx))
	require.Equal(b, int64(generatedCount.Load()), int64(observedCount.Load()), "failed to send all logs to backend")
}
