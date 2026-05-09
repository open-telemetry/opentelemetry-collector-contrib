// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadatatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestNewLogsExporter(t *testing.T) {
	for _, tt := range []struct {
		desc   string
		config *Config
		err    error
	}{
		{
			"simple",
			simpleConfig(),
			nil,
		},
		{
			"empty",
			&Config{},
			errNoResolver,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			// test
			_, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), tt.config)

			// verify
			require.Equal(t, tt.err, err)
		})
	}
}

func TestLogExporterStart(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	for _, tt := range []struct {
		desc string
		le   *logExporterImp
		err  error
	}{
		{
			"ok",
			func() *logExporterImp {
				p, _ := newLogsExporter(exportertest.NewNopSettings(metadata.Type), simpleConfig())
				return p
			}(),
			nil,
		},
		{
			"error",
			func() *logExporterImp {
				// prepare
				lb, err := newLoadBalancer(ts.Logger, simpleConfig(), nil, tb)
				require.NoError(t, err)
				p, _ := newLogsExporter(exportertest.NewNopSettings(metadata.Type), simpleConfig())

				lb.res = &mockResolver{
					onStart: func(context.Context) error {
						return errors.New("some expected err")
					},
				}
				p.loadBalancer = lb

				return p
			}(),
			errors.New("some expected err"),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			p := tt.le

			// test
			res := p.Start(t.Context(), componenttest.NewNopHost())
			defer func() {
				require.NoError(t, p.Shutdown(t.Context()))
			}()

			// verify
			require.Equal(t, tt.err, res)
		})
	}
}

func TestLogExporterShutdown(t *testing.T) {
	p, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(t.Context())

	// verify
	assert.NoError(t, res)
}

func TestLogExporterConsumeLogsReturnsStoppingErrorWhenNotStarted(t *testing.T) {
	p, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), simpleConfig())
	require.NoError(t, err)

	err = p.ConsumeLogs(t.Context(), simpleLogs())
	require.ErrorIs(t, err, errExporterIsStopping)
}

func TestConsumeLogs(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}

	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// test
	res := p.ConsumeLogs(t.Context(), simpleLogs())

	// verify
	assert.NoError(t, res)
}

func TestConsumeLogsCentralQueueEnqueuesCompressedByRoutingKey(t *testing.T) {
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	p := &logExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		randomTraceID: func() pcommon.TraceID {
			return pcommon.TraceID{1}
		},
	}
	p.started.Store(true)

	logs := simpleLogs()
	require.NoError(t, p.ConsumeLogs(t.Context(), logs))
	require.Equal(t, 1, p.centralQueue.len())
	require.Positive(t, p.centralQueue.compressedBytes())

	lease, err := p.centralQueue.lease(t.Context())
	require.NoError(t, err)
	defer lease.done()
	require.Equal(t, signalKindLogs, lease.item.signal)
	expectedRoutingKey := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID()
	require.Equal(t, expectedRoutingKey[:], lease.item.routingKey)
	require.Equal(t, len(lease.item.payload), cap(lease.item.payload))

	decoded, err := decodeCentralQueueLogsItem(lease.item, codec)
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(logs, decoded))
}

func TestConsumeLogsCentralQueueCoalescesRandomRoutingBeforeLaneSelection(t *testing.T) {
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	traceIDs := distinctCentralQueueLaneTraceIDs(t, 2, 64)
	var next atomic.Int64
	p := &logExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
			targetCompressedBytes:        1 << 20,
		}),
		centralCodec:          codec,
		ignoreTraceID:         true,
		centralQueueLaneCount: 64,
		randomTraceID: func() pcommon.TraceID {
			index := int(next.Add(1)-1) % len(traceIDs)
			return traceIDs[index]
		},
	}
	p.started.Store(true)

	first := simpleLogWithID(pcommon.TraceID{1})
	second := simpleLogWithID(pcommon.TraceID{2})
	require.NoError(t, p.ConsumeLogs(t.Context(), first))
	require.NoError(t, p.ConsumeLogs(t.Context(), second))
	require.Equal(t, 2, p.centralQueue.len())

	lease, err := p.centralQueue.lease(t.Context())
	require.NoError(t, err)
	defer lease.done()
	require.Len(t, lease.window.items, 2)
	require.Equal(t, centralQueueRandomLogsRoutingKey(), lease.window.routingKey)

	merged := plog.NewLogs()
	for _, item := range lease.window.items {
		decoded, decodeErr := decodeCentralQueueLogsItem(item, codec)
		require.NoError(t, decodeErr)
		mergeLogChunksByMove(merged, decoded)
	}
	require.Equal(t, first.LogRecordCount()+second.LogRecordCount(), merged.LogRecordCount())
}

func TestLogsCentralQueueShutdownCancelsBeforeWaiting(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)

	endpoint := "endpoint-1:4317"
	consumeStarted := make(chan struct{})
	p := &logExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		loadBalancer: &loadBalancer{
			ring: newHashRing([]string{endpoint}),
			exporters: map[string]*wrappedExporter{endpoint: newWrappedExporter(newMockLogsExporter(func(ctx context.Context, _ plog.Logs) error {
				close(consumeStarted)
				<-ctx.Done()
				return ctx.Err()
			}), endpoint)},
			endpointHealth: newEndpointHealthManager(endpointHealthSettings{}),
			res:            &mockResolver{},
		},
		telemetry: tb,
		logger:    ts.Logger,
	}
	p.started.Store(true)
	dispatchCtx, cancel := context.WithCancel(t.Context())
	p.centralCancel = cancel
	p.centralWG.Add(1)
	go p.runCentralQueue(dispatchCtx)

	item, err := newCentralQueueLogsItem([]byte("route-a"), simpleLogs(), codec, time.Now())
	require.NoError(t, err)
	require.NoError(t, p.centralQueue.enqueue(item))
	select {
	case <-consumeStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected central queue consume to start")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer shutdownCancel()
	require.NoError(t, p.Shutdown(shutdownCtx))
}

func TestLogsCentralQueueShutdownTimeoutStillRunsTeardown(t *testing.T) {
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	var resolverShutdown atomic.Bool
	p := &logExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		loadBalancer: &loadBalancer{
			res: &mockResolver{onShutdown: func(context.Context) error {
				resolverShutdown.Store(true)
				return nil
			}},
		},
	}
	p.started.Store(true)
	p.centralWG.Add(1)
	t.Cleanup(p.centralWG.Done)

	shutdownCtx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	require.Error(t, p.Shutdown(shutdownCtx))
	require.True(t, resolverShutdown.Load())
}

func TestLogsCentralQueueDropsPermanentExportError(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	endpoint := "endpoint-1:4317"
	var calls atomic.Int64
	p := &logExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		loadBalancer: &loadBalancer{
			ring: newHashRing([]string{endpoint}),
			exporters: map[string]*wrappedExporter{endpoint: newWrappedExporter(newMockLogsExporter(func(context.Context, plog.Logs) error {
				calls.Add(1)
				return consumererror.NewPermanent(errors.New("bad logs"))
			}), endpoint)},
			endpointHealth: newEndpointHealthManager(endpointHealthSettings{}),
		},
		telemetry: tb,
		logger:    ts.Logger,
	}

	ctx, cancel := context.WithCancel(t.Context())
	p.centralWG.Add(1)
	go p.runCentralQueue(ctx)
	item, err := newCentralQueueLogsItem([]byte("route-a"), simpleLogs(), codec, time.Now())
	require.NoError(t, err)
	require.NoError(t, p.centralQueue.enqueue(item))

	require.Eventually(t, func() bool {
		return calls.Load() == 1 && p.centralQueue.len() == 0
	}, 2*time.Second, 20*time.Millisecond)
	cancel()
	waitErr := waitForInflight(t.Context(), &p.centralWG)
	require.NoError(t, waitErr)
}

func TestLogsCentralQueueWindowSkipsInvalidPayload(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	endpoint := "endpoint-1:4317"
	var gotCount int
	p := &logExporterImp{
		centralCodec: codec,
		loadBalancer: &loadBalancer{
			ring: newHashRing([]string{endpoint}),
			exporters: map[string]*wrappedExporter{endpoint: newWrappedExporter(newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
				gotCount = ld.LogRecordCount()
				return nil
			}), endpoint)},
			endpointHealth: newEndpointHealthManager(endpointHealthSettings{}),
		},
		telemetry: tb,
		logger:    ts.Logger,
	}

	valid, err := newCentralQueueLogsItem([]byte("lane-a"), simpleLogs(), codec, time.Now())
	require.NoError(t, err)
	invalid := valid
	invalid.payload = []byte("not-valid-zstd")
	invalid.compressedBytes = len(invalid.payload)

	err = p.consumeCentralQueueLogWindow(t.Context(), centralQueueWindow{
		routingKey: []byte("lane-a"),
		items:      []centralQueueItem{invalid, valid},
	})
	require.NoError(t, err)
	require.Equal(t, valid.count, gotCount)
}

func TestLogsCentralQueueFirstRetryUsesInitialDelay(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	endpoint := "endpoint-1:4317"
	p := &logExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		loadBalancer: &loadBalancer{
			ring: newHashRing([]string{endpoint}),
			exporters: map[string]*wrappedExporter{endpoint: newWrappedExporter(newMockLogsExporter(func(context.Context, plog.Logs) error {
				return status.Error(codes.Unavailable, "backend unavailable")
			}), endpoint)},
			endpointHealth: newEndpointHealthManager(endpointHealthSettings{}),
			res:            &mockResolver{},
		},
		telemetry: tb,
		logger:    ts.Logger,
	}

	ctx, cancel := context.WithCancel(t.Context())
	p.centralWG.Add(1)
	go p.runCentralQueue(ctx)
	item, err := newCentralQueueLogsItem([]byte("route-a"), simpleLogs(), codec, time.Now())
	require.NoError(t, err)
	require.NoError(t, p.centralQueue.enqueue(item))

	requireCentralQueueFirstRetryDelay(t, p.centralQueue)
	cancel()
	require.NoError(t, waitForInflight(t.Context(), &p.centralWG))
}

func TestLogsCentralQueueRetriesOnlyFailedEndpointItem(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	var callsMu sync.Mutex
	calls := map[string]int{}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(context.Context, plog.Logs) error {
			callsMu.Lock()
			calls[endpoint]++
			callsMu.Unlock()
			if endpoint == "endpoint-1:4317" {
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			return nil
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	failedRoute := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	successRoute := findRoutingIDForEndpoint(t, lb.ring, "endpoint-2:4317")
	p := &logExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		loadBalancer: lb,
		logger:       ts.Logger,
		telemetry:    tb,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	p.centralWG.Add(1)
	go p.runCentralQueue(ctx)

	failedItem, err := newCentralQueueLogsItem([]byte(failedRoute), simpleLogs(), codec, time.Now())
	require.NoError(t, err)
	// Keep the retry far enough out that slow CI cannot process it before assertions.
	failedItem.attempt = 10
	require.NoError(t, p.centralQueue.enqueue(failedItem))
	successItem, err := newCentralQueueLogsItem([]byte(successRoute), simpleLogs(), codec, time.Now())
	require.NoError(t, err)
	require.NoError(t, p.centralQueue.enqueue(successItem))

	require.Eventually(t, func() bool {
		callsMu.Lock()
		defer callsMu.Unlock()
		return calls["endpoint-1:4317"] == 1 && calls["endpoint-2:4317"] == 1 && p.centralQueue.len() == 1
	}, time.Second, time.Millisecond)
	cancel()
	require.NoError(t, waitForInflight(t.Context(), &p.centralWG))

	p.centralQueue.mu.Lock()
	items := append([]centralQueueItem(nil), p.centralQueue.items...)
	p.centralQueue.mu.Unlock()
	require.Len(t, items, 1)
	remaining := items[0]
	require.Equal(t, []byte(failedRoute), remaining.routingKey)
	require.Equal(t, 11, remaining.attempt)
	require.NotZero(t, remaining.nextAttemptUnixNano)
}

func TestLogsCentralQueueDoesNotDrainSynchronously(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(context.Context, plog.Logs) error {
			if endpoint == "endpoint-1:4317" {
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			return nil
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	releaseDrain := make(chan struct{})
	var releaseDrainOnce sync.Once
	t.Cleanup(func() { releaseDrainOnce.Do(func() { close(releaseDrain) }) })
	lb.onExporterRemove = func(context.Context, string, *wrappedExporter) error {
		<-releaseDrain
		return nil
	}

	routeForEndpoint1 := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	item, err := newCentralQueueLogsItem([]byte(routeForEndpoint1), simpleLogs(), codec, time.Now())
	require.NoError(t, err)

	p := &logExporterImp{
		centralCodec: codec,
		loadBalancer: lb,
		logger:       ts.Logger,
		telemetry:    tb,
	}

	done := make(chan error, 1)
	go func() {
		done <- p.consumeCentralQueueLogItem(t.Context(), item)
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		releaseDrainOnce.Do(func() { close(releaseDrain) })
	case <-time.After(200 * time.Millisecond):
		releaseDrainOnce.Do(func() { close(releaseDrain) })
		t.Fatal("central log queue consume blocked on removed exporter drain")
	}
}

func TestConsumeLogsEmitsOnlyParentExporterMetrics(t *testing.T) {
	ctx := t.Context()
	shutdownCtx := context.Background() //nolint:usetesting // Context must outlive test for cleanup
	telemetry := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, telemetry.Shutdown(shutdownCtx))
	})

	parentParams := exportertest.NewNopSettings(metadata.Type)
	parentParams.TelemetrySettings = telemetry.NewTelemetrySettings()

	cfg := simpleConfig()
	logsExporter, err := newLogsExporter(parentParams, cfg)
	require.NoError(t, err)

	otlpFactory := otlpexporter.NewFactory()
	var childSettings []exporter.Settings
	logsExporter.loadBalancer.componentFactory = func(createCtx context.Context, endpoint string) (component.Component, error) {
		childCfg := buildExporterConfig(cfg, endpoint)
		childParams := buildExporterSettings(otlpFactory.Type(), parentParams, endpoint)
		childSettings = append(childSettings, childParams)

		return exporterhelper.NewLogs(createCtx, childParams, &childCfg, func(context.Context, plog.Logs) error {
			return nil
		})
	}
	logsExporter.batcher, err = newLogBatcher(parentParams.Logger, parentParams.TelemetrySettings, logBatcherSettings{
		maxRecords:    1,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, logsExporter.consumeBatch)
	require.NoError(t, err)
	logsExporter.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return logsExporter.batcher.Remove(ctx, endpoint, exp)
	}
	wrappedExporter, err := exporterhelper.NewLogs(
		ctx,
		parentParams,
		cfg,
		logsExporter.ConsumeLogs,
		exporterhelper.WithStart(logsExporter.Start),
		exporterhelper.WithShutdown(logsExporter.Shutdown),
		exporterhelper.WithCapabilities(logsExporter.Capabilities()),
	)
	require.NoError(t, err)

	require.NoError(t, wrappedExporter.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, wrappedExporter.Shutdown(shutdownCtx))
	})

	logs := generateSingleLogRecord()
	require.NoError(t, wrappedExporter.ConsumeLogs(ctx, logs))

	metric, err := telemetry.GetMetric("otelcol_exporter_sent_log_records")
	require.NoError(t, err)
	sum, ok := metric.Data.(metricdata.Sum[int64])
	require.True(t, ok)

	exporterKey := attribute.Key("exporter")
	var loadbalancingTotal int64
	for _, dp := range sum.DataPoints {
		attr, found := dp.Attributes.Value(exporterKey)
		require.True(t, found, "exporter attribute must be present")
		if attr.AsString() != parentParams.ID.String() {
			assert.Failf(t, "unexpected exporter attribute", "got %s", attr.AsString())
			continue
		}
		loadbalancingTotal += dp.Value
	}

	assert.Equal(t, int64(logs.LogRecordCount()), loadbalancingTotal)

	require.Eventually(t, func() bool {
		loadbalancerMetric, metricErr := telemetry.GetMetric("otelcol_loadbalancer_backend_outcome")
		if metricErr != nil {
			return false
		}
		lbSum, ok := loadbalancerMetric.Data.(metricdata.Sum[int64])
		if !ok {
			return false
		}
		var totalBackendOutcome int64
		for _, dp := range lbSum.DataPoints {
			totalBackendOutcome += dp.Value
		}
		return totalBackendOutcome == 1
	}, time.Second, 10*time.Millisecond)

	require.Len(t, childSettings, 1)
	assert.IsType(t, metricnoop.NewMeterProvider(), childSettings[0].MeterProvider)
	assert.IsType(t, tracenoop.NewTracerProvider(), childSettings[0].TracerProvider)
}

func TestConsumeLogsWithQueueCompressionAndInMemoryQueue(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)

	cfg := simpleConfig()
	cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
	cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionSnappy
	cfg.QueueSettings.CompressInMemory = true

	p, _ := newTestLogsExporter(t, ts, tb, cfg, func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	})

	settings := exportertest.NewNopSettings(metadata.Type)
	settings.TelemetrySettings = ts.TelemetrySettings
	options := []exporterhelper.Option{
		exporterhelper.WithStart(p.Start),
		exporterhelper.WithShutdown(p.Shutdown),
		exporterhelper.WithCapabilities(p.Capabilities()),
	}
	options = append(
		options,
		buildExporterResilienceOptions(
			[]exporterhelper.Option{},
			cfg,
			newQueuePayloadCodecIfEnabled(cfg),
			xexporterhelper.NewLogsQueueBatchSettings(),
		)...,
	)
	wrapped, err := exporterhelper.NewLogs(
		t.Context(),
		settings,
		cfg,
		p.ConsumeLogs,
		options...,
	)
	require.NoError(t, err)

	require.NoError(t, wrapped.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, wrapped.Shutdown(context.WithoutCancel(t.Context())))
	})

	require.NoError(t, wrapped.ConsumeLogs(t.Context(), simpleLogs()))
	require.Eventually(t, func() bool {
		return len(sink.AllLogs()) == 1
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, sink.AllLogs()[0].LogRecordCount())
}

func TestConsumeLogsReroutesEndpointLocalFailure(t *testing.T) {
	ts, tb, telemetry := getTelemetryAssetsWithReader(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)

	calls := map[string]int{}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(context.Context, plog.Logs) error {
			calls[endpoint]++
			if endpoint == "endpoint-1:4317" {
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			return nil
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb
	p.started.Store(true)

	traceIDForEndpoint1 := findTraceIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	err = p.ConsumeLogs(t.Context(), simpleLogWithID(traceIDForEndpoint1))
	require.NoError(t, err)

	assert.Equal(t, 1, calls["endpoint-1:4317"])
	assert.Equal(t, 1, calls["endpoint-2:4317"])
	assert.NotContains(t, lb.exporters, "endpoint-1:4317")
	metadatatest.AssertEqualLoadbalancerBackendQuarantineTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("reason", "unavailable")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualLoadbalancerBackendRerouteTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("signal", "logs"), attribute.String("result", "success"), attribute.String("reason", "unavailable")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestConsumeLogsReroutePreservesPayloadAfterExporterMutation(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)

	var rerouted plog.Logs
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			if endpoint == "endpoint-1:4317" {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("mutated")
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			rerouted = plog.NewLogs()
			ld.CopyTo(rerouted)
			return nil
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb
	p.started.Store(true)

	traceIDForEndpoint1 := findTraceIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	input := simpleLogWithID(traceIDForEndpoint1)
	expected := simpleLogWithID(traceIDForEndpoint1)
	require.NoError(t, p.ConsumeLogs(t.Context(), input))

	require.NoError(t, plogtest.CompareLogs(expected, rerouted))
}

func TestConsumeLogsBatcherFlushDoesNotQuarantineEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)

	lb, err := newLoadBalancer(ts.Logger, cfg, func(context.Context, string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	exp := newWrappedExporter(newMockLogsExporter(func(context.Context, plog.Logs) error {
		return status.Error(codes.Unavailable, "backend unavailable")
	}), "endpoint-1:4317")
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": exp,
	}

	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb

	err = p.consumeBatcherFlush(t.Context(), exp, simpleLogs(), logFlushReasonShutdown)
	require.Error(t, err)
	assert.Contains(t, lb.exporters, "endpoint-1:4317")
	assert.ElementsMatch(t, []string{"endpoint-1:4317", "endpoint-2:4317"}, lb.endpointHealth.eligibleEndpoints())
}

func generateSingleLogRecord() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("test log")
	logRecord.SetTimestamp(pcommon.Timestamp(123))
	return logs
}

func TestConsumeLogsUnexpectedExporterType(t *testing.T) {
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	ts, tb := getTelemetryAssets(t)

	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	err = p.ConsumeLogs(t.Context(), simpleLogs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("unable to export logs, unexpected exporter type: expected exporter.Logs but got %T", newNopMockExporter()))

	require.NoError(t, p.Shutdown(t.Context()))
}

func TestLogBatchWithTwoTraces(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	}

	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	first := simpleLogs()
	second := simpleLogWithID(pcommon.TraceID([16]byte{2, 3, 4, 5}))
	batch := plog.NewLogs()
	firstTgt := batch.ResourceLogs().AppendEmpty()
	first.ResourceLogs().At(0).CopyTo(firstTgt)
	secondTgt := batch.ResourceLogs().AppendEmpty()
	second.ResourceLogs().At(0).CopyTo(secondTgt)

	// test
	err = p.ConsumeLogs(t.Context(), batch)

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.AllLogs(), 2)
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestNoLogsInBatch(t *testing.T) {
	for _, tt := range []struct {
		desc  string
		batch plog.Logs
	}{
		{
			"no resource logs",
			plog.NewLogs(),
		},
		{
			"no instrumentation library logs",
			func() plog.Logs {
				batch := plog.NewLogs()
				batch.ResourceLogs().AppendEmpty()
				return batch
			}(),
		},
		{
			"no logs",
			func() plog.Logs {
				batch := plog.NewLogs()
				batch.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
				return batch
			}(),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res := traceIDFromLogs(tt.batch)
			assert.Equal(t, pcommon.NewTraceIDEmpty(), res)
		})
	}
}

func TestLogsWithoutTraceID(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	// test
	err = p.ConsumeLogs(t.Context(), simpleLogWithoutID())

	// verify
	assert.NoError(t, err)
	require.NoError(t, p.Shutdown(t.Context()))
	assert.Len(t, sink.AllLogs(), 1)
}

func TestConsumeLogsIgnoreTraceIDUsesRandomRoutingKey(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := twoEndpointLogConfig()
	cfg.LogRouting.IgnoreTraceID = true

	calls := map[string]int{}
	p, lb := newTestLogsExporter(t, ts, tb, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(context.Context, plog.Logs) error {
			calls[endpoint]++
			return nil
		}), nil
	})
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	traceIDForEndpoint1 := findTraceIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	randomTraceIDForEndpoint2 := findTraceIDForEndpoint(t, lb.ring, "endpoint-2:4317")
	p.randomTraceID = func() pcommon.TraceID {
		return randomTraceIDForEndpoint2
	}

	require.NoError(t, p.ConsumeLogs(t.Context(), simpleLogWithID(traceIDForEndpoint1)))

	assert.Zero(t, calls["endpoint-1:4317"])
	assert.Equal(t, 1, calls["endpoint-2:4317"])
}

func TestConsumeLogsIgnoreTraceIDCentralByteBatchDoesNotSplitByTraceID(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := twoEndpointLogConfig()
	cfg.LogRouting.IgnoreTraceID = true
	enableCentralQueueByteBatchingForTest(cfg)

	var calls atomic.Int64
	var records atomic.Int64
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			assert.Equal(t, "endpoint-2:4317", endpoint)
			calls.Add(1)
			records.Add(int64(ld.LogRecordCount()))
			return nil
		}), nil
	}

	p, lb := newTestLogsExporter(t, ts, tb, cfg, componentFactory)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()
	traceIDForEndpoint2 := findTraceIDForEndpoint(t, lb.ring, "endpoint-2:4317")
	p.randomTraceID = func() pcommon.TraceID {
		return traceIDForEndpoint2
	}

	input := logsWithTraceIDs([16]byte{1}, [16]byte{2}, [16]byte{3})
	require.NoError(t, p.ConsumeLogs(t.Context(), input))

	assert.Equal(t, int64(1), calls.Load())
	assert.Equal(t, int64(3), records.Load())
}

func TestConsumeLogsIgnoreTraceIDWithoutCentralByteBatchingKeepsTraceSplit(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := twoEndpointLogConfig()
	cfg.LogRouting.IgnoreTraceID = true

	var calls atomic.Int64
	var records atomic.Int64
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			assert.Equal(t, "endpoint-2:4317", endpoint)
			calls.Add(1)
			records.Add(int64(ld.LogRecordCount()))
			return nil
		}), nil
	}

	p, lb := newTestLogsExporter(t, ts, tb, cfg, componentFactory)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()
	traceIDForEndpoint2 := findTraceIDForEndpoint(t, lb.ring, "endpoint-2:4317")
	p.randomTraceID = func() pcommon.TraceID {
		return traceIDForEndpoint2
	}

	input := logsWithTraceIDs([16]byte{1}, [16]byte{2}, [16]byte{3})
	require.NoError(t, p.ConsumeLogs(t.Context(), input))

	assert.Equal(t, int64(3), calls.Load())
	assert.Equal(t, int64(3), records.Load())
}

func TestConsumeLogsRecordsBackendRequestMetrics(t *testing.T) {
	ts, tb, telemetry := getTelemetryAssetsWithReader(t)
	cfg := twoEndpointLogConfig()
	endpoint := "endpoint-1:4317"

	sent := plog.NewLogs()
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			if endpoint == "endpoint-1:4317" {
				ld.CopyTo(sent)
			}
			return nil
		}), nil
	}

	p, lb := newTestLogsExporter(t, ts, tb, cfg, componentFactory)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	traceIDForEndpoint1 := findTraceIDForEndpoint(t, lb.ring, endpoint)
	require.NoError(t, p.ConsumeLogs(t.Context(), simpleLogWithID(traceIDForEndpoint1)))

	assert.Equal(t, 1, sent.LogRecordCount())
	assertBackendRequestMetrics(
		t,
		telemetry,
		backendRequestSignalLogs,
		endpoint,
		serializedLogsSize(sent),
		int64(sent.LogRecordCount()),
	)
}

func TestConsumeLogsCentralByteBatchReroutesWholeMergedBatch(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := twoEndpointLogConfig()
	cfg.LogRouting.IgnoreTraceID = true
	enableCentralQueueByteBatchingForTest(cfg)
	enableEndpointHealth(cfg)

	calls := map[string]int{}
	records := map[string]int{}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			calls[endpoint]++
			records[endpoint] += ld.LogRecordCount()
			if endpoint == "endpoint-1:4317" {
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			return nil
		}), nil
	}

	p, lb := newTestLogsExporter(t, ts, tb, cfg, componentFactory)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()
	traceIDForEndpoint1 := findTraceIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	p.randomTraceID = func() pcommon.TraceID {
		return traceIDForEndpoint1
	}

	input := logsWithTraceIDs([16]byte{1}, [16]byte{2}, [16]byte{3}, [16]byte{4})
	require.NoError(t, p.ConsumeLogs(t.Context(), input))

	assert.Equal(t, 1, calls["endpoint-1:4317"])
	assert.Equal(t, 1, calls["endpoint-2:4317"])
	assert.Equal(t, 4, records["endpoint-1:4317"])
	assert.Equal(t, 4, records["endpoint-2:4317"])
}

func TestConsumeLogsCentralByteBatchHighCardinalityTraceIDsUseOneBackendRequest(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := twoEndpointLogConfig()
	cfg.LogRouting.IgnoreTraceID = true
	enableCentralQueueByteBatchingForTest(cfg)

	var calls atomic.Int64
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			assert.Equal(t, "endpoint-2:4317", endpoint)
			assert.Equal(t, 64, ld.LogRecordCount())
			calls.Add(1)
			return nil
		}), nil
	}

	p, lb := newTestLogsExporter(t, ts, tb, cfg, componentFactory)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()
	traceIDForEndpoint2 := findTraceIDForEndpoint(t, lb.ring, "endpoint-2:4317")
	p.randomTraceID = func() pcommon.TraceID {
		return traceIDForEndpoint2
	}

	ids := make([]pcommon.TraceID, 64)
	for i := range ids {
		ids[i][15] = byte(i + 1)
	}
	require.NoError(t, p.ConsumeLogs(t.Context(), logsWithTraceIDs(ids...)))

	assert.Equal(t, int64(1), calls.Load())
}

func BenchmarkConsumeLogsRandomRoutingRequestAmplification(b *testing.B) {
	ids := make([]pcommon.TraceID, 256)
	for i := range ids {
		ids[i][15] = byte(i + 1)
	}
	input := logsWithTraceIDs(ids...)
	serializedBytes := serializedLogsSize(input)

	for _, tc := range []struct {
		name              string
		centralByteBatch  bool
		expectedSendCount int64
	}{
		{name: "legacy_split", centralByteBatch: false, expectedSendCount: 256},
		{name: "central_byte_batch", centralByteBatch: true, expectedSendCount: 1},
	} {
		b.Run(tc.name, func(b *testing.B) {
			ts, tb := getTelemetryAssets(b)
			cfg := twoEndpointLogConfig()
			cfg.LogRouting.IgnoreTraceID = true
			if tc.centralByteBatch {
				enableCentralQueueByteBatchingForTest(cfg)
			}

			var sendCount atomic.Int64
			componentFactory := func(_ context.Context, _ string) (component.Component, error) {
				return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
					sendCount.Add(1)
					return nil
				}), nil
			}

			p, lb := newTestLogsExporter(b, ts, tb, cfg, componentFactory)
			require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
			b.Cleanup(func() {
				require.NoError(b, p.Shutdown(context.WithoutCancel(b.Context())))
			})
			traceIDForEndpoint1 := findTraceIDForEndpoint(b, lb.ring, "endpoint-1:4317")
			p.randomTraceID = func() pcommon.TraceID {
				return traceIDForEndpoint1
			}

			b.SetBytes(serializedBytes)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				require.NoError(b, p.ConsumeLogs(b.Context(), input))
			}
			b.StopTimer()

			requestsPerInput := float64(sendCount.Load()) / float64(b.N)
			b.ReportMetric(requestsPerInput, "backend_requests/input")
			require.Equal(b, float64(tc.expectedSendCount), requestsPerInput)
		})
	}
}

func TestGroupLogsByEndpointKeepsEmptyTraceLogsTogetherPerScope(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), nil, tb)
	require.NoError(t, err)

	first := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	second := newWrappedExporter(newNopMockLogsExporter(), "endpoint-2:4317")
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": first,
		"endpoint-2:4317": second,
	}

	p, err := newLogsExporter(ts, simpleConfig())
	require.NoError(t, err)
	p.loadBalancer = lb

	batches, groupErr := p.groupLogsByEndpoint(sharedScopeLogsWithoutTraceIDs("first", "second"))
	require.NoError(t, groupErr)
	require.Len(t, batches, 1)
	for _, batch := range batches {
		assert.Equal(t, 2, batch.logs.LogRecordCount())
	}
}

func TestGroupLogsByEndpointIgnoreTraceIDUsesRandomRoutingKey(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := twoEndpointLogConfig()
	cfg.LogRouting.IgnoreTraceID = true
	lb, err := newLoadBalancer(ts.Logger, cfg, nil, tb)
	require.NoError(t, err)

	first := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	second := newWrappedExporter(newNopMockLogsExporter(), "endpoint-2:4317")
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": first,
		"endpoint-2:4317": second,
	}

	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb

	traceIDForEndpoint1 := findTraceIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	randomTraceIDForEndpoint2 := findTraceIDForEndpoint(t, lb.ring, "endpoint-2:4317")
	p.randomTraceID = func() pcommon.TraceID {
		return randomTraceIDForEndpoint2
	}

	batches, groupErr := p.groupLogsByEndpoint(simpleLogWithID(traceIDForEndpoint1))
	require.NoError(t, groupErr)
	require.Len(t, batches, 1)
	require.Contains(t, batches, "endpoint-2:4317")
	assert.Equal(t, batches["endpoint-2:4317"].exp, second)
	assert.Equal(t, 1, batches["endpoint-2:4317"].logs.LogRecordCount())
}

func TestConsumeLogLegacyPathIgnoresStoppingGate(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	logsConsumed := atomic.Int64{}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), nil, tb)
	require.NoError(t, err)

	exp := newWrappedExporter(newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
		logsConsumed.Add(int64(ld.LogRecordCount()))
		return nil
	}), "endpoint-1:4317")
	exp.markStopping()

	lb.ring = newHashRing([]string{"endpoint-1"})
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": exp,
	}

	p, err := newLogsExporter(ts, simpleConfig())
	require.NoError(t, err)
	p.loadBalancer = lb

	err = p.consumeLog(t.Context(), simpleLogs())
	require.NoError(t, err)
	assert.Equal(t, int64(1), logsConsumed.Load())
}

func TestEnqueueEndpointBatchesReroutesStoppingExporterOnce(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	logsConsumed := atomic.Int64{}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), nil, tb)
	require.NoError(t, err)

	stoppingExp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	stoppingExp.markStopping()
	liveExp := newWrappedExporter(newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
		logsConsumed.Add(int64(ld.LogRecordCount()))
		return nil
	}), "endpoint-2:4317")

	lb.ring = newHashRing([]string{"endpoint-2"})
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-2:4317": liveExp,
	}

	p, err := newLogsExporter(ts, simpleConfig())
	require.NoError(t, err)
	p.loadBalancer = lb
	p.batcher, err = newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    1,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, p.consumeBatch)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.batcher.Shutdown(t.Context()))
	}()

	err = p.enqueueEndpointBatches(t.Context(), map[string]*endpointBatch{
		"endpoint-1:4317": {
			logs: simpleLogs(),
			exp:  stoppingExp,
		},
	}, true)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return logsConsumed.Load() == 1
	}, time.Second, 10*time.Millisecond)
}

func TestEnqueueEndpointBatchesReturnsStoppingErrorAfterSecondCollision(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), nil, tb)
	require.NoError(t, err)

	initialExp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	initialExp.markStopping()
	reroutedExp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-2:4317")
	reroutedExp.markStopping()

	lb.ring = newHashRing([]string{"endpoint-2"})
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-2:4317": reroutedExp,
	}

	p, err := newLogsExporter(ts, simpleConfig())
	require.NoError(t, err)
	p.loadBalancer = lb
	p.batcher, err = newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    1,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, p.consumeBatch)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.batcher.Shutdown(t.Context()))
	}()

	err = p.enqueueEndpointBatches(t.Context(), map[string]*endpointBatch{
		"endpoint-1:4317": {
			logs: simpleLogs(),
			exp:  initialExp,
		},
	}, true)
	require.ErrorIs(t, err, errLogBatcherExporterStopping)
}

func TestInsertLogRecordKeepsDistinctDroppedAttributeCounts(t *testing.T) {
	dest := plog.NewLogs()

	srcA := simpleLogs()
	rlA := srcA.ResourceLogs().At(0)
	slA := rlA.ScopeLogs().At(0)
	rlA.Resource().SetDroppedAttributesCount(1)
	slA.Scope().SetDroppedAttributesCount(2)

	srcB := simpleLogs()
	rlB := srcB.ResourceLogs().At(0)
	slB := rlB.ScopeLogs().At(0)
	rlB.Resource().SetDroppedAttributesCount(3)
	slB.Scope().SetDroppedAttributesCount(4)

	insertLogRecord(dest, rlA, slA, slA.LogRecords().At(0))
	insertLogRecord(dest, rlB, slB, slB.LogRecords().At(0))

	require.Equal(t, 2, dest.ResourceLogs().Len())
	require.Equal(t, uint32(1), dest.ResourceLogs().At(0).Resource().DroppedAttributesCount())
	require.Equal(t, uint32(3), dest.ResourceLogs().At(1).Resource().DroppedAttributesCount())
	require.Equal(t, uint32(2), dest.ResourceLogs().At(0).ScopeLogs().At(0).Scope().DroppedAttributesCount())
	require.Equal(t, uint32(4), dest.ResourceLogs().At(1).ScopeLogs().At(0).Scope().DroppedAttributesCount())
}

// this test validates that exporter is can concurrently change the endpoints while consuming logs.
func TestConsumeLogs_ConcurrentResolverChange(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	consumeStarted := make(chan struct{})
	consumeDone := make(chan struct{})

	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		// imitate a slow exporter
		te := &mockLogsExporter{Component: mockComponent{}}
		te.consumelogsfn = func(_ context.Context, _ plog.Logs) error {
			close(consumeStarted)
			time.Sleep(50 * time.Millisecond)
			return te.consumeErr
		}
		return te, nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	endpoints := []string{"endpoint-1"}
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb
	p.batcher, err = newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    1,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, p.consumeBatch)
	require.NoError(t, err)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	go func() {
		assert.NoError(t, p.ConsumeLogs(t.Context(), simpleLogs()))
		close(consumeDone)
	}()

	// update endpoint while consuming logs
	<-consumeStarted
	endpoints = []string{"endpoint-2"}
	endpoint, err := lb.res.resolve(t.Context())
	require.NoError(t, err)
	require.Equal(t, endpoints, endpoint)
	<-consumeDone
}

func TestRollingUpdatesWhenConsumeLogs(t *testing.T) {
	// this test is based on the discussion in the following issue for this exporter:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	// prepare

	// simulate rolling updates, the dns resolver should resolve in the following order
	// ["127.0.0.1"] -> ["127.0.0.1", "127.0.0.2"] -> ["127.0.0.2"]
	ts, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	mu := sync.Mutex{}
	var lastResolved []string
	res.onChange(func(s []string) {
		mu.Lock()
		lastResolved = s
		mu.Unlock()
	})

	counter := &atomic.Int64{}
	resolve := [][]net.IPAddr{
		{
			{IP: net.IPv4(127, 0, 0, 1)},
		}, {
			{IP: net.IPv4(127, 0, 0, 1)},
			{IP: net.IPv4(127, 0, 0, 2)},
		}, {
			{IP: net.IPv4(127, 0, 0, 2)},
		},
	}
	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			if counter.Load() <= 2 {
				return resolve[counter.Load()], nil
			}

			return resolve[2], nil
		},
	}
	res.resInterval = 100 * time.Millisecond

	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{Hostname: "service-1", Port: ""}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = res
	p.loadBalancer = lb
	p.batcher, err = newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    1,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, p.consumeBatch)
	require.NoError(t, err)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}

	counter1 := &atomic.Int64{}
	counter2 := &atomic.Int64{}
	id1 := "127.0.0.1:4317"
	id2 := "127.0.0.2:4317"
	unreachableCh := make(chan struct{})
	defaultExporters := map[string]*wrappedExporter{
		id1: newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			counter1.Add(1)
			counter.Add(1)
			// simulate an unreachable backend
			<-unreachableCh
			return nil
		}), id1),
		id2: newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			counter2.Add(1)
			return nil
		}), id2),
	}

	// test
	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()
	// ensure using default exporters
	lb.updateLock.Lock()
	lb.exporters = defaultExporters
	lb.updateLock.Unlock()
	lb.res.onChange(func(_ []string) {
		lb.updateLock.Lock()
		lb.exporters = defaultExporters
		lb.updateLock.Unlock()
	})

	ctx, cancel := context.WithCancel(t.Context())
	var waitWG sync.WaitGroup
	// keep consuming traces every 2ms
	consumeCh := make(chan struct{})
	go func(ctx context.Context) {
		ticker := time.NewTicker(2 * time.Millisecond)
		for {
			select {
			case <-ctx.Done():
				consumeCh <- struct{}{}
				return
			case <-ticker.C:
				waitWG.Go(func() {
					err := p.ConsumeLogs(ctx, randomLogs())
					assert.True(t, err == nil || errors.Is(err, context.Canceled))
				})
			}
		}
	}(ctx)

	// give limited but enough time to rolling updates. otherwise this test
	// will still pass due to the unreacheableCh that is used to simulate
	// unreachable backends.
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		require.Positive(tt, counter1.Load())
		require.Positive(tt, counter2.Load())
	}, 3*time.Second, 100*time.Millisecond)
	cancel()
	<-consumeCh

	// verify
	mu.Lock()
	require.Contains(t, lastResolved, "127.0.0.2")
	mu.Unlock()

	close(unreachableCh)
	waitWG.Wait()
}

func randomLogs() plog.Logs {
	return simpleLogWithID(random())
}

func simpleLogs() plog.Logs {
	return simpleLogWithID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
}

func twoEndpointLogConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1:4317", "endpoint-2:4317"}}),
		},
	}
}

func enableCentralQueueByteBatchingForTest(cfg *Config) {
	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.Sizer = exporterhelper.RequestSizerTypeBytes
	queueCfg.QueueSize = 1 << 20
	queueCfg.NumConsumers = 4
	queueCfg.Batch = configoptional.Some(exporterhelper.BatchConfig{
		Sizer:        exporterhelper.RequestSizerTypeBytes,
		MinSize:      256,
		MaxSize:      1 << 20,
		FlushTimeout: time.Second,
	})
	cfg.QueueSettings.Enabled = true
	cfg.QueueSettings.QueueConfig = configoptional.Some(queueCfg)
}

func simpleLogWithID(id pcommon.TraceID) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty().SetTraceID(id)

	return logs
}

func findTraceIDForEndpoint(tb testing.TB, ring *hashRing, endpoint string) pcommon.TraceID {
	tb.Helper()

	for i := range 4096 {
		var traceID pcommon.TraceID
		copy(traceID[:], strconv.Itoa(i))
		if ring.endpointFor(traceID[:]) == endpoint {
			return traceID
		}
	}

	require.FailNow(tb, "failed to find trace id for endpoint", "endpoint=%s", endpoint)
	return pcommon.TraceID{}
}

func distinctCentralQueueLaneTraceIDs(t *testing.T, count, laneCount int) []pcommon.TraceID {
	t.Helper()
	traceIDs := make([]pcommon.TraceID, 0, count)
	seen := make(map[string]struct{}, count)
	for i := 1; len(traceIDs) < count && i < 10_000; i++ {
		var traceID pcommon.TraceID
		copy(traceID[:], strconv.Itoa(i))
		laneKey := string(centralQueueLaneRoutingKey(signalKindLogs, traceID[:], laneCount))
		if _, ok := seen[laneKey]; ok {
			continue
		}
		seen[laneKey] = struct{}{}
		traceIDs = append(traceIDs, traceID)
	}
	require.Len(t, traceIDs, count)
	return traceIDs
}

func sharedResourceScopeLog(body string) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	for i := range 8 {
		rl.Resource().Attributes().PutStr(fmt.Sprintf("resource-%d", i), "shared-resource-value")
	}
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("shared-scope")
	for i := range 8 {
		sl.Scope().Attributes().PutStr(fmt.Sprintf("scope-%d", i), "shared-scope-value")
	}
	rec := sl.LogRecords().AppendEmpty()
	rec.Body().SetStr(body)
	rec.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
	return logs
}

func sharedScopeLogsWithoutTraceIDs(bodies ...string) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("resource", "shared")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("shared-scope")
	for _, body := range bodies {
		rec := sl.LogRecords().AppendEmpty()
		rec.Body().SetStr(body)
	}
	return logs
}

func simpleLogWithoutID() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty()

	return logs
}

type mockLogsExporter struct {
	component.Component
	consumelogsfn func(ctx context.Context, ld plog.Logs) error
	consumeErr    error
}

func (*mockLogsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *mockLogsExporter) Shutdown(context.Context) error {
	e.consumeErr = errors.New("exporter is shut down")
	return nil
}

func (e *mockLogsExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if e.consumelogsfn == nil {
		return e.consumeErr
	}
	return e.consumelogsfn(ctx, ld)
}

type mockComponent struct {
	component.StartFunc
	component.ShutdownFunc
}

func newMockLogsExporter(consumelogsfn func(ctx context.Context, ld plog.Logs) error) exporter.Logs {
	return &mockLogsExporter{
		Component:     mockComponent{},
		consumelogsfn: consumelogsfn,
	}
}

func newNopMockLogsExporter() exporter.Logs {
	return &mockLogsExporter{Component: mockComponent{}}
}
