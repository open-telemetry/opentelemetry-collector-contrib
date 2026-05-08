// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"slices"
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
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadatatest"
)

func TestLogBatcherReroutesEndpointLocalFlushFailure(t *testing.T) {
	ts, tb, telemetry := getTelemetryAssetsWithReader(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.LogBatcher = LogBatcherConfig{Enabled: true, MaxRecords: 1, MaxBytes: 1 << 20, FlushInterval: time.Hour}
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}})

	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			switch endpoint {
			case "endpoint-1:4317":
				endpoint1Calls.Add(1)
				return status.Error(codes.Unavailable, "backend down")
			case "endpoint-2:4317":
				endpoint2Calls.Add(1)
			}
			return nil
		}), nil
	}, tb)
	require.NoError(t, err)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}
	p.loadBalancer.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	p.started.Store(true)
	defer func() { _ = p.Shutdown(context.WithoutCancel(t.Context())) }()

	route := findTraceIDForEndpointsBeforeAndAfterReroute(t, p.loadBalancer.ring, "endpoint-1:4317", "endpoint-2:4317")
	require.NoError(t, p.ConsumeLogs(t.Context(), simpleLogWithID(route)))

	require.Eventually(t, func() bool {
		return endpoint1Calls.Load() == 1 && endpoint2Calls.Load() == 1
	}, 2*time.Second, 20*time.Millisecond)
	metadatatest.AssertEqualLoadbalancerBackendRerouteTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("signal", "logs"), attribute.String("result", "success"), attribute.String("reason", "unavailable")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestCompressedLogBatcherReroutesEndpointLocalFlushFailure(t *testing.T) {
	ts, tb, telemetry := getTelemetryAssetsWithReader(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.LogBatcher = LogBatcherConfig{
		Enabled:            true,
		MaxRecords:         1,
		MaxBytes:           1 << 20,
		FlushInterval:      time.Hour,
		PayloadCompression: QueuePayloadCompressionZstd,
	}
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}})

	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			switch endpoint {
			case "endpoint-1:4317":
				endpoint1Calls.Add(1)
				return status.Error(codes.Unavailable, "backend down")
			case "endpoint-2:4317":
				endpoint2Calls.Add(1)
			}
			return nil
		}), nil
	}, tb)
	require.NoError(t, err)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}
	p.loadBalancer.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	p.started.Store(true)
	defer func() { _ = p.Shutdown(context.WithoutCancel(t.Context())) }()

	route := findTraceIDForEndpointsBeforeAndAfterReroute(t, p.loadBalancer.ring, "endpoint-1:4317", "endpoint-2:4317")
	require.NoError(t, p.ConsumeLogs(t.Context(), simpleLogWithID(route)))

	require.Eventually(t, func() bool {
		return endpoint1Calls.Load() == 1 && endpoint2Calls.Load() == 1
	}, 2*time.Second, 20*time.Millisecond)
	metadatatest.AssertEqualLoadbalancerBackendRerouteTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("signal", "logs"), attribute.String("result", "success"), attribute.String("reason", "unavailable")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestLogBatcherFlushExporterStoppingIsRerouteable(t *testing.T) {
	ts, _, telemetry := getTelemetryAssetsWithReader(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)

	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)
	stoppingExp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	stoppingExp.markStopping()

	err = p.consumeBatcherFlush(t.Context(), stoppingExp, simpleLogs(), logFlushReasonSize)

	var rerouteable logBatcherRerouteableError
	require.ErrorAs(t, err, &rerouteable)
	rerouteable.RecordReroute(t.Context(), nil)
	metadatatest.AssertEqualLoadbalancerBackendRerouteTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("signal", "logs"), attribute.String("result", "success"), attribute.String("reason", "exporter_stopping")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestCompressedLogBatcherFlushReroutesStoppingExporter(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var reroutedRecords atomic.Int64
	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:         100,
		maxBytes:           1 << 20,
		flushInterval:      time.Hour,
		payloadCompression: QueuePayloadCompressionZstd,
	}, func(context.Context, *wrappedExporter, plog.Logs, string) error {
		t.Fatal("send should not be called when compressed flush can reroute a stopping exporter")
		return nil
	}, func(_ context.Context, ld plog.Logs, reason string) error {
		require.Equal(t, logFlushReasonResolverChange, reason)
		reroutedRecords.Add(int64(ld.LogRecordCount()))
		return nil
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context()))) }()

	exp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, simpleLogs()))
	exp.markStopping()
	require.NoError(t, batcher.Remove(t.Context(), "endpoint-1:4317", exp))

	require.Equal(t, int64(1), reroutedRecords.Load())
}

func TestLogBatcherRerouteTargetEndpointLocalFailureMarksTargetUnhealthy(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.LogBatcher = LogBatcherConfig{Enabled: true, MaxRecords: 1, MaxBytes: 1 << 20, FlushInterval: time.Hour}
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2", "endpoint-3"}})

	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			switch endpoint {
			case "endpoint-1:4317":
				endpoint1Calls.Add(1)
				return status.Error(codes.Unavailable, "backend down")
			case "endpoint-2:4317":
				endpoint2Calls.Add(1)
				return status.Error(codes.Unavailable, "backend down")
			}
			return nil
		}), nil
	}, tb)
	require.NoError(t, err)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}
	p.loadBalancer.onBackendChanges([]string{"endpoint-1", "endpoint-2", "endpoint-3"})
	p.started.Store(true)
	defer func() { _ = p.Shutdown(context.WithoutCancel(t.Context())) }()

	route := findTraceIDForEndpointsBeforeAndAfterReroute(t, p.loadBalancer.ring, "endpoint-1:4317", "endpoint-2:4317", "endpoint-3:4317")
	require.NoError(t, p.ConsumeLogs(t.Context(), simpleLogWithID(route)))

	require.Eventuallyf(t, func() bool {
		eligible := p.loadBalancer.endpointHealth.eligibleEndpoints()
		return !slices.Contains(eligible, "endpoint-1:4317") && !slices.Contains(eligible, "endpoint-2:4317")
	}, 2*time.Second, 20*time.Millisecond, "eligible=%v endpoint1=%d endpoint2=%d", p.loadBalancer.endpointHealth.eligibleEndpoints(), endpoint1Calls.Load(), endpoint2Calls.Load())
}

func TestLogBatcherQueuedDataDuringCleanupReroutesAwayFromStoppedExporter(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.LogBatcher = LogBatcherConfig{Enabled: true, MaxRecords: 1, MaxBytes: 1 << 20, FlushInterval: time.Hour}
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}})

	firstFlushStarted := make(chan struct{})
	releaseFirstFlush := make(chan struct{})
	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			switch endpoint {
			case "endpoint-1:4317":
				if endpoint1Calls.Add(1) == 1 {
					close(firstFlushStarted)
					<-releaseFirstFlush
				}
				return status.Error(codes.Unavailable, "backend down")
			case "endpoint-2:4317":
				endpoint2Calls.Add(1)
			}
			return nil
		}), nil
	}, tb)
	require.NoError(t, err)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}
	p.loadBalancer.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	p.started.Store(true)
	defer func() { _ = p.Shutdown(context.WithoutCancel(t.Context())) }()

	route := findTraceIDForEndpoint(t, p.loadBalancer.ring, "endpoint-1:4317")
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- p.ConsumeLogs(t.Context(), simpleLogWithID(route))
	}()
	<-firstFlushStarted
	require.NoError(t, p.ConsumeLogs(t.Context(), simpleLogWithID(route)))
	close(releaseFirstFlush)
	require.NoError(t, <-firstDone)

	require.Eventually(t, func() bool {
		return endpoint1Calls.Load() == 1 && endpoint2Calls.Load() == 2
	}, 2*time.Second, 20*time.Millisecond)
}

func findTraceIDForEndpointsBeforeAndAfterReroute(t *testing.T, ring *hashRing, firstEndpoint, rerouteEndpoint string, additionalRerouteEndpoints ...string) pcommon.TraceID {
	t.Helper()

	rerouteEndpoints := append([]string{rerouteEndpoint}, additionalRerouteEndpoints...)
	rerouteRing := newHashRing(rerouteEndpoints)
	for i := range 4096 {
		var traceID pcommon.TraceID
		copy(traceID[:], strconv.Itoa(i))
		if ring.endpointFor(traceID[:]) == firstEndpoint && rerouteRing.endpointFor(traceID[:]) == rerouteEndpoint {
			return traceID
		}
	}

	require.FailNow(t, "failed to find trace id for endpoints", "first=%s reroute=%s", firstEndpoint, rerouteEndpoint)
	return pcommon.TraceID{}
}

func TestLogBatcherMergesSameBackendOnShutdown(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	p, _ := newTestLogsExporter(t, ts, tb, simpleConfig(), func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	})

	p.batcher, _ = newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    100,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, p.consumeBatch)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, p.ConsumeLogs(t.Context(), logsWithTraceIDs([16]byte{1}, [16]byte{2})))
	require.NoError(t, p.Shutdown(t.Context()))

	require.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 2, sink.AllLogs()[0].LogRecordCount())
}

func TestLogBatcherFlushesOnTimeout(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	p, _ := newTestLogsExporter(t, ts, tb, simpleConfig(), func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	})

	p.batcher, _ = newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    100,
		maxBytes:      1 << 20,
		flushInterval: 25 * time.Millisecond,
	}, p.consumeBatch)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, p.Shutdown(t.Context())) }()
	require.NoError(t, p.ConsumeLogs(t.Context(), simpleLogs()))

	require.Eventually(t, func() bool {
		return len(sink.AllLogs()) == 1
	}, time.Second, 10*time.Millisecond)
}

func TestLogBatcherFlushesOnMaxBytes(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	p, _ := newTestLogsExporter(t, ts, tb, simpleConfig(), func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	})

	first := sizedLogWithID(pcommon.TraceID([16]byte{1}), 512)
	maxBytes := (&plog.ProtoMarshaler{}).LogsSize(first) + 1

	p.batcher, _ = newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    100,
		maxBytes:      maxBytes,
		flushInterval: time.Hour,
	}, p.consumeBatch)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, p.Shutdown(t.Context())) }()
	require.NoError(t, p.ConsumeLogs(t.Context(), mergeLogs(first, sizedLogWithID(pcommon.TraceID([16]byte{2}), 512))))

	require.Eventually(t, func() bool {
		return len(sink.AllLogs()) == 1 && sink.AllLogs()[0].LogRecordCount() == 2
	}, time.Second, 10*time.Millisecond)
}

func TestLogBatcherMaxBytesUsesMergedPayloadSize(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	p, _ := newTestLogsExporter(t, ts, tb, simpleConfig(), func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	})

	first := sharedResourceScopeLog("first")
	second := sharedResourceScopeLog("second")
	sizer := &plog.ProtoMarshaler{}
	separateBytes := sizer.LogsSize(first) + sizer.LogsSize(second)
	merged := mergeLogs(sharedResourceScopeLog("first"), sharedResourceScopeLog("second"))
	mergedBytes := sizer.LogsSize(merged)
	require.Greater(t, separateBytes, mergedBytes)

	p.batcher, _ = newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    100,
		maxBytes:      mergedBytes + 1,
		flushInterval: time.Hour,
	}, p.consumeBatch)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, p.ConsumeLogs(t.Context(), mergeLogs(sharedResourceScopeLog("first"), sharedResourceScopeLog("second"))))
	assert.Empty(t, sink.AllLogs())
	require.NoError(t, p.Shutdown(t.Context()))
	require.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 2, sink.AllLogs()[0].LogRecordCount())
}

func TestCompressedLogBatcherChunkDoesNotRetainUncompressedPayload(t *testing.T) {
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	logs := compressibleLogs(16000, 2048)
	chunk, err := newCompressedLogBatcherChunk(&plog.ProtoMarshaler{}, codec, logBatcherRequest{
		logs:               logs,
		enqueuedAtUnixNano: time.Now().UnixNano(),
	})
	require.NoError(t, err)

	require.Equal(t, logs.LogRecordCount(), chunk.records)
	require.Greater(t, chunk.uncompressedBytes, chunk.compressedBytes*100)
	require.Len(t, chunk.payload, chunk.compressedBytes)
	require.Equal(t, len(chunk.payload), cap(chunk.payload), "compressed payload must not retain the uncompressed marshal buffer")

	decoded, err := decodeCompressedLogBatcherChunk(&plog.ProtoUnmarshaler{}, codec, chunk)
	require.NoError(t, err)
	require.Equal(t, logs.LogRecordCount(), decoded.LogRecordCount())
}

func TestCompressedLogBatcherFlushPreservesLogsAndSplitsByMaxRecords(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var mu sync.Mutex
	var batchSizes []int
	var bodies []string

	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:         3,
		maxBytes:           1 << 20,
		flushInterval:      time.Hour,
		payloadCompression: QueuePayloadCompressionZstd,
	}, func(_ context.Context, _ *wrappedExporter, ld plog.Logs, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		batchSizes = append(batchSizes, ld.LogRecordCount())
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			rl := ld.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				records := rl.ScopeLogs().At(j).LogRecords()
				for k := 0; k < records.Len(); k++ {
					bodies = append(bodies, records.At(k).Body().Str())
				}
			}
		}
		return nil
	})
	require.NoError(t, err)

	exp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, sharedScopeLogsWithoutTraceIDs("one", "two")))
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, sharedScopeLogsWithoutTraceIDs("three", "four")))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(batchSizes) == 2
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, batcher.Shutdown(t.Context()))

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []int{2, 2}, batchSizes)
	require.ElementsMatch(t, []string{"one", "two", "three", "four"}, bodies)
}

func TestCompressedLogBatcherMaxBytesUsesMergedPayloadSize(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var calls atomic.Int64

	first := sharedResourceScopeLog("first")
	second := sharedResourceScopeLog("second")
	sizer := &plog.ProtoMarshaler{}
	separateBytes := sizer.LogsSize(first) + sizer.LogsSize(second)
	mergedBytes := sizer.LogsSize(mergeLogs(sharedResourceScopeLog("first"), sharedResourceScopeLog("second")))
	require.Greater(t, separateBytes, mergedBytes)

	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:         100,
		maxBytes:           mergedBytes + 1,
		flushInterval:      time.Hour,
		payloadCompression: QueuePayloadCompressionZstd,
	}, func(context.Context, *wrappedExporter, plog.Logs, string) error {
		calls.Add(1)
		return nil
	})
	require.NoError(t, err)
	shutdown := false
	t.Cleanup(func() {
		if !shutdown {
			require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
		}
	})

	exp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, sharedResourceScopeLog("first")))
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, sharedResourceScopeLog("second")))
	require.Never(t, func() bool {
		return calls.Load() > 0
	}, 50*time.Millisecond, 5*time.Millisecond)

	require.NoError(t, batcher.Shutdown(t.Context()))
	shutdown = true
	require.Equal(t, int64(1), calls.Load())
}

func TestCompressedLogBatcherFlushSplitsOnMaxBytes(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var mu sync.Mutex
	var batchSizes []int
	var bodies []string

	sizer := &plog.ProtoMarshaler{}
	firstTwo := mergeLogs(sharedResourceScopeLog("one"), sharedResourceScopeLog("two"))
	allThree := mergeLogs(mergeLogs(sharedResourceScopeLog("one"), sharedResourceScopeLog("two")), sharedResourceScopeLog("three"))
	maxBytes := sizer.LogsSize(firstTwo) + 1
	require.Greater(t, sizer.LogsSize(allThree), maxBytes)

	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:         100,
		maxBytes:           maxBytes,
		flushInterval:      time.Hour,
		payloadCompression: QueuePayloadCompressionZstd,
	}, func(_ context.Context, _ *wrappedExporter, ld plog.Logs, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		batchSizes = append(batchSizes, ld.LogRecordCount())
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			rl := ld.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				records := rl.ScopeLogs().At(j).LogRecords()
				for k := 0; k < records.Len(); k++ {
					bodies = append(bodies, records.At(k).Body().Str())
				}
			}
		}
		return nil
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, sharedResourceScopeLog("one")))
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, sharedResourceScopeLog("two")))
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, sharedResourceScopeLog("three")))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(batchSizes) == 2
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []int{2, 1}, batchSizes)
	require.ElementsMatch(t, []string{"one", "two", "three"}, bodies)
}

func TestCompressedLogBatcherFlushRecordsDecodeErrors(t *testing.T) {
	telemetry := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, telemetry.Shutdown(context.WithoutCancel(t.Context())))
	})
	telemetrySettings := telemetry.NewTelemetrySettings()
	batcherTelemetry, err := newLogBatcherTelemetry(telemetrySettings)
	require.NoError(t, err)
	t.Cleanup(batcherTelemetry.shutdown)

	backend := &backendLogBatcher{
		endpoint:     "endpoint-1:4317",
		logger:       telemetrySettings.Logger,
		settings:     logBatcherSettings{maxRecords: 100, maxBytes: 1 << 20, flushInterval: time.Hour},
		telemetry:    batcherTelemetry,
		payloadCodec: newQueuePayloadCodec(QueuePayloadCompressionZstd),
		send: func(context.Context, *wrappedExporter, plog.Logs, string) error {
			t.Fatal("send should not be called when a compressed chunk cannot be decoded")
			return nil
		},
	}
	t.Cleanup(func() { require.NoError(t, backend.payloadCodec.Close()) })

	pending := []compressedLogBatcherChunk{{
		payload:               []byte("not-a-valid-compressed-payload"),
		records:               2,
		uncompressedBytes:     100,
		compressedBytes:       30,
		oldestEnqueueUnixNano: time.Now().UnixNano(),
	}}
	pendingRecords := 2
	pendingBytes := 100
	pendingCompressedBytes := 30
	timer := time.NewTimer(time.Hour)
	require.True(t, timer.Stop())
	var timerC <-chan time.Time

	err = backend.flushCompressed(
		t.Context(),
		&plog.ProtoMarshaler{},
		&plog.ProtoUnmarshaler{},
		newCompressedLogBatcherSizeState(),
		&pending,
		&pendingRecords,
		&pendingBytes,
		&pendingCompressedBytes,
		logFlushReasonSize,
		timer,
		&timerC,
	)
	require.Error(t, err)
	require.Zero(t, pendingRecords)
	require.Zero(t, pendingBytes)
	require.Zero(t, pendingCompressedBytes)

	require.Eventually(t, func() bool {
		flushErrors, flushErr := telemetry.GetMetric("otelcol_loadbalancer_log_batch_flush_errors")
		droppedRecords, droppedErr := telemetry.GetMetric("otelcol_loadbalancer_log_batch_dropped_records")
		if flushErr != nil || droppedErr != nil {
			return false
		}
		flushTotal, flushOK := sumInt64Metric(flushErrors)
		droppedTotal, droppedOK := sumInt64Metric(droppedRecords)
		return flushOK && droppedOK && flushTotal == 1 && droppedTotal == 2
	}, time.Second, 10*time.Millisecond)
}

func TestCompressedLogBatcherEnqueueStoresCompressedRequests(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	sendStarted := make(chan struct{}, 1)
	unblockSend := make(chan struct{})

	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:         1,
		maxBytes:           1 << 20,
		flushInterval:      time.Hour,
		payloadCompression: QueuePayloadCompressionZstd,
	}, func(context.Context, *wrappedExporter, plog.Logs, string) error {
		select {
		case sendStarted <- struct{}{}:
		default:
		}
		<-unblockSend
		return nil
	})
	require.NoError(t, err)
	defer func() {
		close(unblockSend)
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	}()

	exp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, simpleLogs()))
	select {
	case <-sendStarted:
	case <-time.After(time.Second):
		t.Fatal("expected first batch to block in send")
	}

	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, compressibleLogs(16, 1024)))

	batcher.mu.RLock()
	backend := batcher.backends["endpoint-1:4317"]
	batcher.mu.RUnlock()
	require.NotNil(t, backend)

	select {
	case req := <-backend.requests:
		require.Zero(t, req.logs.LogRecordCount())
		require.Equal(t, 16, req.compressedChunk.records)
		require.Positive(t, req.compressedChunk.compressedBytes)
		require.Less(t, req.compressedChunk.compressedBytes, req.compressedChunk.uncompressedBytes)
	case <-time.After(time.Second):
		t.Fatal("expected queued request to stay in compressed form while backend send is blocked")
	}
}

func TestCompressedLogBatcherSnapshotTracksCompressedPendingBytes(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:         20000,
		maxBytes:           64 << 20,
		flushInterval:      time.Hour,
		payloadCompression: QueuePayloadCompressionZstd,
	}, func(context.Context, *wrappedExporter, plog.Logs, string) error {
		return nil
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context()))) }()

	exp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, compressibleLogs(256, 1024)))

	require.Eventually(t, func() bool {
		for _, pending := range batcher.snapshotPending().pending {
			if pending.endpoint == "endpoint-1:4317" && pending.records == 256 {
				return pending.compressedBytes > 0 && pending.bytes > pending.compressedBytes
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

func TestLogBatcherDoesNotBlockOtherBackends(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	blockFirst := make(chan struct{})
	firstStarted := make(chan struct{}, 1)
	var secondCalls atomic.Int64
	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    1,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, func(ctx context.Context, exp *wrappedExporter, ld plog.Logs, reason string) error {
		_ = reason
		exp.consumeWG.Add(1)
		defer exp.consumeWG.Done()
		return exp.ConsumeLogs(ctx, ld)
	})
	require.NoError(t, err)
	defer func() {
		close(blockFirst)
		require.NoError(t, batcher.Shutdown(t.Context()))
	}()

	firstExporter := newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
		firstStarted <- struct{}{}
		<-blockFirst
		return nil
	}), "endpoint-1:4317")
	secondExporter := newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
		secondCalls.Add(1)
		return nil
	}), "endpoint-2:4317")

	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", firstExporter, simpleLogs()))
	<-firstStarted
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-2:4317", secondExporter, simpleLogs()))

	require.Eventually(t, func() bool {
		return secondCalls.Load() == 1
	}, time.Second, 10*time.Millisecond)
}

func TestLogBatcherShutdownWaitsForInflightEnqueue(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var calls atomic.Int64
	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    100,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, func(ctx context.Context, exp *wrappedExporter, ld plog.Logs, reason string) error {
		_ = ctx
		_ = exp
		_ = reason
		calls.Add(int64(ld.LogRecordCount()))
		return nil
	})
	require.NoError(t, err)

	backend, err := batcher.acquireBackend("endpoint-1:4317", newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317"))
	require.NoError(t, err)

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- batcher.Shutdown(t.Context())
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("shutdown returned before inflight enqueue finished: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	backend.requests <- logBatcherRequest{kind: logBatcherRequestEnqueue, logs: simpleLogs()}
	backend.inflight.Done()

	require.NoError(t, <-shutdownDone)
	assert.Equal(t, int64(1), calls.Load())
}

func TestLogBatcherShutdownRespectsContextWhileWaitingForInflight(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    100,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, func(context.Context, *wrappedExporter, plog.Logs, string) error {
		return nil
	})
	require.NoError(t, err)

	backend, err := batcher.acquireBackend("endpoint-1:4317", newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317"))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.ErrorIs(t, batcher.Shutdown(ctx), context.Canceled)
	backend.inflight.Done()
	require.Eventually(t, func() bool {
		select {
		case <-backend.done:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
}

func TestLogBatcherEnqueueAfterShutdownReturnsStoppingError(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    100,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, func(context.Context, *wrappedExporter, plog.Logs, string) error {
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, batcher.Shutdown(t.Context()))

	err = batcher.Enqueue(
		t.Context(),
		"endpoint-1:4317",
		newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317"),
		simpleLogs(),
	)
	require.ErrorIs(t, err, errLogBatcherExporterStopping)
	snapshot := batcher.snapshotPending()
	assert.Empty(t, snapshot.pending)
	assert.Zero(t, snapshot.maxOldestAgeMillis)
}

func TestLogBatcherRecordsPendingOldestAgeAndFlushAge(t *testing.T) {
	telemetry := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, telemetry.Shutdown(context.WithoutCancel(t.Context())))
	})

	batcher, err := newLogBatcher(componenttest.NewNopTelemetrySettings().Logger, telemetry.NewTelemetrySettings(), logBatcherSettings{
		maxRecords:    100,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, func(context.Context, *wrappedExporter, plog.Logs, string) error {
		return nil
	})
	require.NoError(t, err)

	exp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, simpleLogs()))
	time.Sleep(25 * time.Millisecond)

	pendingMetric, err := telemetry.GetMetric("otelcol_loadbalancer_log_batch_pending_oldest_record_age")
	require.NoError(t, err)
	pendingGauge, ok := pendingMetric.Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Positive(t, findGaugePointValue(t, pendingGauge.DataPoints, attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"))))

	maxMetric, err := telemetry.GetMetric("otelcol_loadbalancer_log_batch_pending_oldest_record_age_max")
	require.NoError(t, err)
	maxGauge, ok := maxMetric.Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Positive(t, maxGauge.DataPoints[0].Value)

	require.NoError(t, batcher.Shutdown(t.Context()))

	flushMetric, err := telemetry.GetMetric("otelcol_loadbalancer_log_batch_flush_oldest_record_age")
	require.NoError(t, err)
	flushHistogram, ok := flushMetric.Data.(metricdata.Histogram[int64])
	require.True(t, ok)
	require.Len(t, flushHistogram.DataPoints, 1)
	require.Equal(t, uint64(1), flushHistogram.DataPoints[0].Count)
	require.Positive(t, flushHistogram.DataPoints[0].Sum)
}

func TestLogBatcherHandleRequestUsesAcceptanceTimeForOldestAge(t *testing.T) {
	backend := &backendLogBatcher{
		endpoint:  "endpoint-1:4317",
		settings:  logBatcherSettings{maxRecords: 100, maxBytes: 1 << 20, flushInterval: time.Hour},
		requests:  make(chan logBatcherRequest, 1),
		done:      make(chan struct{}),
		telemetry: &logBatcherTelemetry{},
	}

	pending := plog.NewLogs()
	pendingRecords := 0
	pendingBytes := 0
	sizer := &plog.ProtoMarshaler{}
	var nextReq *logBatcherRequest
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	var timerC <-chan time.Time

	start := time.Now()
	stopped := backend.handleRequest(
		logBatcherRequest{
			kind:               logBatcherRequestEnqueue,
			logs:               simpleLogs(),
			enqueuedAtUnixNano: start.UnixNano(),
		},
		sizer,
		&pending,
		&pendingRecords,
		&pendingBytes,
		&nextReq,
		timer,
		&timerC,
	)
	require.False(t, stopped)

	stored := time.Unix(0, backend.oldestEnqueue.Load())
	require.False(t, stored.Before(start), "oldest timestamp should reflect request enqueue time")
}

func TestLogBatcherHandleRequestUsesOldestDrainedEnqueueTime(t *testing.T) {
	backend := &backendLogBatcher{
		endpoint:  "endpoint-1:4317",
		settings:  logBatcherSettings{maxRecords: 100, maxBytes: 1 << 20, flushInterval: time.Hour},
		requests:  make(chan logBatcherRequest, 1),
		done:      make(chan struct{}),
		telemetry: &logBatcherTelemetry{},
	}

	older := time.Now().Add(-80 * time.Millisecond).UnixNano()
	newer := time.Now().Add(-20 * time.Millisecond).UnixNano()
	backend.requests <- logBatcherRequest{
		kind:               logBatcherRequestEnqueue,
		logs:               simpleLogs(),
		enqueuedAtUnixNano: older,
	}

	pending := plog.NewLogs()
	pendingRecords := 0
	pendingBytes := 0
	sizer := &plog.ProtoMarshaler{}
	var nextReq *logBatcherRequest
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	var timerC <-chan time.Time

	stopped := backend.handleRequest(
		logBatcherRequest{
			kind:               logBatcherRequestEnqueue,
			logs:               simpleLogs(),
			enqueuedAtUnixNano: newer,
		},
		sizer,
		&pending,
		&pendingRecords,
		&pendingBytes,
		&nextReq,
		timer,
		&timerC,
	)
	require.False(t, stopped)
	require.Equal(t, 2, pendingRecords)
	require.Equal(t, older, backend.oldestEnqueue.Load())
}

func TestLogBatcherPendingAgeIncludesTimeBufferedBeforeWorkerDequeues(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	sendStarted := make(chan struct{}, 1)
	unblockSend := make(chan struct{})
	sendReleased := false

	batcher, err := newLogBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		logBatcherSettings{maxRecords: 1000, maxBytes: 1 << 20, flushInterval: 10 * time.Millisecond},
		func(_ context.Context, _ *wrappedExporter, _ plog.Logs, _ string) error {
			select {
			case sendStarted <- struct{}{}:
			default:
			}
			<-unblockSend
			return nil
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if !sendReleased {
			close(unblockSend)
		}
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317")
	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, simpleLogs()))

	select {
	case <-sendStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected backend send to start")
	}

	batcher.mu.RLock()
	backend := batcher.backends["endpoint-1:4317"]
	batcher.mu.RUnlock()
	require.NotNil(t, backend)
	backend.settings.flushInterval = time.Hour

	require.NoError(t, batcher.Enqueue(t.Context(), "endpoint-1:4317", exp, simpleLogs()))
	time.Sleep(60 * time.Millisecond)
	close(unblockSend)
	sendReleased = true

	var oldestAgeMillis int64
	require.Eventually(t, func() bool {
		for _, pending := range batcher.snapshotPending().pending {
			if pending.endpoint == "endpoint-1:4317" && pending.records == 1 {
				oldestAgeMillis = pending.oldestAgeMillis
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	require.GreaterOrEqual(t, oldestAgeMillis, int64(40), "oldest age should include time spent buffered in backend.requests")
}

func TestLogBatcherSnapshotPendingIgnoresOldestAgeWhenNoRecordsPending(t *testing.T) {
	batcher := &logBatcher{
		backends: map[string]*backendLogBatcher{
			"endpoint-1:4317": {},
		},
	}
	batcher.backends["endpoint-1:4317"].oldestEnqueue.Store(time.Now().Add(-time.Second).UnixNano())
	batcher.backends["endpoint-1:4317"].pendingBytes.Store(123)

	snapshot := batcher.snapshotPending()
	require.Len(t, snapshot.pending, 1)
	require.Zero(t, snapshot.pending[0].records)
	require.Zero(t, snapshot.pending[0].oldestAgeMillis)
	require.Zero(t, snapshot.maxOldestAgeMillis)
}

func findGaugePointValue(t *testing.T, dps []metricdata.DataPoint[int64], attrs attribute.Set) int64 {
	t.Helper()

	for _, dp := range dps {
		if dp.Attributes.Equals(&attrs) {
			return dp.Value
		}
	}

	t.Fatalf("gauge datapoint with attrs %v not found", attrs)
	return 0
}

func sumInt64Metric(metric metricdata.Metrics) (int64, bool) {
	sum, ok := metric.Data.(metricdata.Sum[int64])
	if !ok {
		return 0, false
	}
	var total int64
	for _, dp := range sum.DataPoints {
		total += dp.Value
	}
	return total, true
}

func TestLogBatcherRemoveRespectsContextWhileWaitingForInflight(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	batcher, err := newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    100,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, func(context.Context, *wrappedExporter, plog.Logs, string) error {
		return nil
	})
	require.NoError(t, err)
	defer batcher.telemetry.shutdown()

	backend, err := batcher.acquireBackend("endpoint-1:4317", newWrappedExporter(newNopMockLogsExporter(), "endpoint-1:4317"))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.ErrorIs(t, batcher.Remove(ctx, "endpoint-1:4317", backend.exporter()), context.Canceled)
	backend.inflight.Done()
	require.Eventually(t, func() bool {
		select {
		case <-backend.done:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
}

func TestLogBatcherFlushesRemovedBackendToOldExporter(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	endpoints := []string{"endpoint-1"}

	p, lb := newTestLogsExporter(t, ts, tb, &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}}),
		},
	}, func(_ context.Context, endpoint string) (component.Component, error) {
		switch endpoint {
		case "endpoint-1:4317":
			return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
				endpoint1Calls.Add(1)
				return nil
			}), nil
		case "endpoint-2:4317":
			return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
				endpoint2Calls.Add(1)
				return nil
			}), nil
		default:
			return newNopMockLogsExporter(), nil
		}
	})

	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb
	p.batcher, _ = newLogBatcher(ts.Logger, ts.TelemetrySettings, logBatcherSettings{
		maxRecords:    100,
		maxBytes:      1 << 20,
		flushInterval: time.Hour,
	}, p.consumeBatch)
	p.loadBalancer.onExporterRemove = func(ctx context.Context, endpoint string, exp *wrappedExporter) error {
		return p.batcher.Remove(ctx, endpoint, exp)
	}

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, p.Shutdown(t.Context())) }()

	require.NoError(t, p.ConsumeLogs(t.Context(), simpleLogs()))
	endpoints = []string{"endpoint-2"}
	_, err := lb.res.resolve(t.Context())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return endpoint1Calls.Load() == 1
	}, time.Second, 10*time.Millisecond)
	assert.Zero(t, endpoint2Calls.Load())
}

func newTestLogsExporter(
	tb testing.TB,
	ts exporter.Settings,
	telemetryBuilder *metadata.TelemetryBuilder,
	cfg *Config,
	componentFactory func(context.Context, string) (component.Component, error),
) (*logExporterImp, *loadBalancer) {
	tb.Helper()

	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, telemetryBuilder)
	require.NoError(tb, err)
	lb.addMissingExporters(tb.Context(), cfg.Resolver.Static.Get().Hostnames)
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return cfg.Resolver.Static.Get().Hostnames, nil
		},
	}

	p, err := newLogsExporter(ts, cfg)
	require.NoError(tb, err)
	p.loadBalancer = lb
	return p, lb
}

func logsWithTraceIDs(ids ...pcommon.TraceID) plog.Logs {
	logs := plog.NewLogs()
	for _, id := range ids {
		single := simpleLogWithID(id)
		mergeLogs(logs, single)
	}
	return logs
}

func sizedLogWithID(id pcommon.TraceID, size int) plog.Logs {
	logs := simpleLogWithID(id)
	logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr(string(make([]byte, size)))
	return logs
}

func compressibleLogs(records, bodySize int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("resource", "shared")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("shared-scope")
	body := string(make([]byte, bodySize))
	for i := range records {
		rec := sl.LogRecords().AppendEmpty()
		rec.Body().SetStr(body)
		rec.SetTraceID(pcommon.TraceID([16]byte{byte(i >> 8), byte(i)}))
	}
	return logs
}
