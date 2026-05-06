// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadatatest"
)

func TestMetricBatcherReroutesEndpointLocalFlushFailure(t *testing.T) {
	ts, tb, telemetry := getTelemetryAssetsWithReader(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)
	cfg.MetricBatcher = MetricBatcherConfig{Enabled: true, MaxDataPoints: 1, MaxBytes: 1 << 20, FlushInterval: time.Hour}
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}})

	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
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

	service := findRoutingIDForEndpoint(t, p.loadBalancer.ring, "endpoint-1:4317")
	md := singleDataPointMetric("batcher-reroute")
	md.ResourceMetrics().At(0).Resource().Attributes().PutStr(serviceNameKey, service)
	require.NoError(t, p.ConsumeMetrics(t.Context(), md))

	require.Eventuallyf(t, func() bool {
		return endpoint1Calls.Load() == 1 && endpoint2Calls.Load() == 1
	}, 2*time.Second, 20*time.Millisecond, "endpoint-1 calls=%d endpoint-2 calls=%d", endpoint1Calls.Load(), endpoint2Calls.Load())
	metadatatest.AssertEqualLoadbalancerBackendRerouteTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("signal", "metrics"), attribute.String("result", "success"), attribute.String("reason", "unavailable")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestMetricBatcherFlushExporterStoppingIsRerouteable(t *testing.T) {
	ts, _, telemetry := getTelemetryAssetsWithReader(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	stoppingExp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	stoppingExp.markStopping()

	err = p.consumeBatch(t.Context(), stoppingExp, singleDataPointMetric("stopping"), metricFlushReasonSize)

	var rerouteable metricBatcherRerouteableError
	require.ErrorAs(t, err, &rerouteable)
	require.Equal(t, 1, rerouteable.Data().DataPointCount())
	rerouteable.RecordReroute(t.Context(), nil)
	metadatatest.AssertEqualLoadbalancerBackendRerouteTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("signal", "metrics"), attribute.String("result", "success"), attribute.String("reason", "exporter_stopping")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestMetricBatcherRerouteTargetEndpointLocalFailureMarksTargetUnhealthy(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)
	cfg.MetricBatcher = MetricBatcherConfig{Enabled: true, MaxDataPoints: 1, MaxBytes: 1 << 20, FlushInterval: time.Hour}
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2", "endpoint-3"}})

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			if endpoint == "endpoint-1:4317" || endpoint == "endpoint-2:4317" {
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

	service := findRoutingIDForEndpointsBeforeAndAfterReroute(t, p.loadBalancer.ring, "endpoint-1:4317", "endpoint-2:4317", "endpoint-3:4317")
	md := singleDataPointMetric("batcher-reroute-target-failure")
	md.ResourceMetrics().At(0).Resource().Attributes().PutStr(serviceNameKey, service)
	require.NoError(t, p.ConsumeMetrics(t.Context(), md))

	require.Eventually(t, func() bool {
		eligible := p.loadBalancer.endpointHealth.eligibleEndpoints()
		return !slices.Contains(eligible, "endpoint-1:4317") && !slices.Contains(eligible, "endpoint-2:4317")
	}, 2*time.Second, 20*time.Millisecond)
}

func findRoutingIDForEndpointsBeforeAndAfterReroute(t *testing.T, ring *hashRing, firstEndpoint, rerouteEndpoint string, additionalRerouteEndpoints ...string) string {
	t.Helper()

	rerouteEndpoints := append([]string{rerouteEndpoint}, additionalRerouteEndpoints...)
	rerouteRing := newHashRing(rerouteEndpoints)
	for i := range 4096 {
		routingID := fmt.Sprintf("routing-id-%d", i)
		if ring.endpointFor([]byte(routingID)) == firstEndpoint && rerouteRing.endpointFor([]byte(routingID)) == rerouteEndpoint {
			return routingID
		}
	}

	require.FailNow(t, "failed to find routing id for endpoints", "first=%s reroute=%s", firstEndpoint, rerouteEndpoint)
	return ""
}

func TestMetricBatcherDoesNotReroutePlainConsumerErrorMetrics(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var flushCalls atomic.Int64
	var rerouteCalls atomic.Int64
	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1, maxBytes: 1 << 20, flushInterval: 20 * time.Millisecond},
		func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, _ string) error {
			flushCalls.Add(1)
			return consumererror.NewMetrics(errors.New("partial non-endpoint failure"), md)
		},
		func(context.Context, pmetric.Metrics, string) error {
			rerouteCalls.Add(1)
			return nil
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	require.Eventually(t, func() bool {
		return flushCalls.Load() >= 2
	}, 2*time.Second, 20*time.Millisecond)
	require.Zero(t, rerouteCalls.Load())
}

func TestMetricBatcherMaxRerouteAttemptsZeroDoesNotPoisonBackend(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.MaxRerouteAttempts = 0
	cfg.MetricBatcher = MetricBatcherConfig{Enabled: true, MaxDataPoints: 1, MaxBytes: 1 << 20, FlushInterval: 20 * time.Millisecond}

	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
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
	defer func() { require.NoError(t, p.Shutdown(context.WithoutCancel(t.Context()))) }()

	service := findRoutingIDForEndpoint(t, p.loadBalancer.ring, "endpoint-1:4317")
	md := singleDataPointMetric("batcher-no-reroute")
	md.ResourceMetrics().At(0).Resource().Attributes().PutStr(serviceNameKey, service)
	require.NoError(t, p.ConsumeMetrics(t.Context(), md))

	require.Eventually(t, func() bool {
		return endpoint1Calls.Load() >= 2
	}, 2*time.Second, 20*time.Millisecond)
	require.Zero(t, endpoint2Calls.Load())

	p.batcher.mu.RLock()
	backend := p.batcher.backends["endpoint-1:4317"]
	p.batcher.mu.RUnlock()
	require.NotNil(t, backend)
	require.False(t, backend.exporter().isStopping())
}

func TestMetricBatcherEndpointHealthDisabledDoesNotRerouteOrPoisonBackend(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	cfg.MetricBatcher = MetricBatcherConfig{Enabled: true, MaxDataPoints: 1, MaxBytes: 1 << 20, FlushInterval: 20 * time.Millisecond}

	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
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

	service := findRoutingIDForEndpoint(t, p.loadBalancer.ring, "endpoint-1:4317")
	md := singleDataPointMetric("batcher-health-disabled")
	md.ResourceMetrics().At(0).Resource().Attributes().PutStr(serviceNameKey, service)
	require.NoError(t, p.ConsumeMetrics(t.Context(), md))

	require.Eventually(t, func() bool {
		return endpoint1Calls.Load() >= 2
	}, 2*time.Second, 20*time.Millisecond)
	require.Zero(t, endpoint2Calls.Load())

	p.batcher.mu.RLock()
	backend := p.batcher.backends["endpoint-1:4317"]
	p.batcher.mu.RUnlock()
	require.NotNil(t, backend)
	require.False(t, backend.exporter().isStopping())
}

func TestMetricBatcherFailOpenDoesNotRerouteOrPoisonBackend(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)
	cfg.MetricBatcher = MetricBatcherConfig{Enabled: true, MaxDataPoints: 1, MaxBytes: 1 << 20, FlushInterval: 20 * time.Millisecond}
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}})

	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
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
	p.loadBalancer.onBackendChanges([]string{"endpoint-1"})
	p.started.Store(true)
	defer func() { _ = p.Shutdown(context.WithoutCancel(t.Context())) }()

	md := singleDataPointMetric("batcher-fail-open")
	md.ResourceMetrics().At(0).Resource().Attributes().PutStr(serviceNameKey, "endpoint-1-route")
	require.NoError(t, p.ConsumeMetrics(t.Context(), md))

	require.Eventually(t, func() bool {
		return endpoint1Calls.Load() >= 2
	}, 2*time.Second, 20*time.Millisecond)
	require.Zero(t, endpoint2Calls.Load())

	p.batcher.mu.RLock()
	backend := p.batcher.backends["endpoint-1:4317"]
	p.batcher.mu.RUnlock()
	if backend != nil {
		require.False(t, backend.exporter().isStopping())
	}
	p.loadBalancer.updateLock.RLock()
	activeExp := p.loadBalancer.exporters["endpoint-1:4317"]
	p.loadBalancer.updateLock.RUnlock()
	require.NotNil(t, activeExp)
	require.False(t, activeExp.isStopping())
}

func TestMetricBatcherQueuedDataDuringCleanupReroutesAwayFromStoppedExporter(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)
	cfg.MetricBatcher = MetricBatcherConfig{Enabled: true, MaxDataPoints: 1, MaxBytes: 1 << 20, FlushInterval: time.Hour}

	firstFlushStarted := make(chan struct{})
	releaseFirstFlush := make(chan struct{})
	var endpoint1Calls atomic.Int64
	var endpoint2Calls atomic.Int64
	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer, err = newLoadBalancer(ts.Logger, cfg, func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
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

	service := findRoutingIDForEndpoint(t, p.loadBalancer.ring, "endpoint-1:4317")
	firstMetric := singleDataPointMetric("batcher-cleanup-first")
	firstMetric.ResourceMetrics().At(0).Resource().Attributes().PutStr(serviceNameKey, service)
	require.NoError(t, p.ConsumeMetrics(t.Context(), firstMetric))
	<-firstFlushStarted

	secondMetric := singleDataPointMetric("batcher-cleanup-second")
	secondMetric.ResourceMetrics().At(0).Resource().Attributes().PutStr(serviceNameKey, service)
	require.NoError(t, p.ConsumeMetrics(t.Context(), secondMetric))
	close(releaseFirstFlush)

	require.Eventually(t, func() bool {
		return endpoint1Calls.Load() == 1 && endpoint2Calls.Load() == 2
	}, 2*time.Second, 20*time.Millisecond)
}

func TestMetricBatcherRerouteFailureRestoresFailedMetricsForRetry(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var sendCalls atomic.Int64
	var rerouteCalls atomic.Int64
	retriedFailedData := make(chan struct{}, 1)

	failedRerouteData := singleDataPointMetric("failed-reroute")
	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1, maxBytes: 1 << 20, flushInterval: 20 * time.Millisecond},
		func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, _ string) error {
			if sendCalls.Add(1) == 1 {
				return metricBatcherRerouteableError{err: status.Error(codes.Unavailable, "backend down"), data: md}
			}
			if metricNameExists(md, "failed-reroute") {
				retriedFailedData <- struct{}{}
			}
			return nil
		},
		func(context.Context, pmetric.Metrics, string) error {
			rerouteCalls.Add(1)
			return consumererror.NewMetrics(errors.New("reroute failed"), failedRerouteData)
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("original"))

	select {
	case <-retriedFailedData:
	case <-time.After(2 * time.Second):
		t.Fatal("expected failed reroute metrics to be restored for retry")
	}
	require.Equal(t, int64(1), rerouteCalls.Load())
}

func TestMetricBatcherFlushesOnMaxDataPoints(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	flushed := make(chan int, 1)

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 2, maxBytes: 1 << 20, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, _ string) error {
			flushed <- md.DataPointCount()
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = batcher.Shutdown(context.WithoutCancel(t.Context()))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	select {
	case got := <-flushed:
		require.Equal(t, 2, got)
	case <-time.After(2 * time.Second):
		t.Fatal("expected metric batch flush on max_datapoints")
	}
}

func TestMetricBatcherFlushesOnInterval(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	flushed := make(chan int, 1)

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: 50 * time.Millisecond},
		func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, _ string) error {
			flushed <- md.DataPointCount()
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = batcher.Shutdown(context.WithoutCancel(t.Context()))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	select {
	case got := <-flushed:
		require.Equal(t, 1, got)
	case <-time.After(2 * time.Second):
		t.Fatal("expected metric batch flush on interval")
	}
}

func TestMetricBatcherFlushesOnMaxBytes(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	flushed := make(chan int, 1)

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, _ string) error {
			flushed <- md.DataPointCount()
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	select {
	case got := <-flushed:
		require.Equal(t, 1, got)
	case <-time.After(2 * time.Second):
		t.Fatal("expected metric batch flush on max_bytes")
	}
}

func TestMetricBatcherRemoveFlushesPending(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var flushes atomic.Int64

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			flushes.Add(1)
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))
	require.NoError(t, batcher.Remove(t.Context(), "endpoint-1:4317", exp))

	require.Equal(t, int64(1), flushes.Load())
}

func TestMetricBatcherShutdownFlushesPending(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	flushed := make(chan int, 1)

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, reason string) error {
			if reason == metricFlushReasonShutdown {
				flushed <- md.DataPointCount()
			}
			return nil
		},
		nil,
	)
	require.NoError(t, err)

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))
	require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))

	select {
	case got := <-flushed:
		require.Equal(t, 1, got)
	case <-time.After(2 * time.Second):
		t.Fatal("expected metric batch flush on shutdown")
	}
}

func TestMetricBatcherRemoveTimeoutSchedulesCleanup(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error { return nil },
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	batcher.mu.RLock()
	backend := batcher.backends["endpoint-1:4317"]
	batcher.mu.RUnlock()
	require.NotNil(t, backend)

	backend.inflight.Add(1)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	err = batcher.Remove(ctx, "endpoint-1:4317", exp)
	require.Error(t, err)

	go backend.inflight.Done()

	require.Eventually(t, func() bool {
		select {
		case <-backend.done:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)
}

func TestMetricBatcherRestoresPendingOnTimeoutFlushFailure(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var calls atomic.Int64

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: 30 * time.Millisecond},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			if calls.Add(1) == 1 {
				return errors.New("boom")
			}
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	require.Eventually(t, func() bool {
		return calls.Load() >= 2
	}, 2*time.Second, 20*time.Millisecond)
}

func TestMetricBatcherRestoresPendingBytesFromMergedPayloadOnFailure(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	firstFailed := make(chan struct{}, 1)

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: 200 * time.Millisecond},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			select {
			case firstFailed <- struct{}{}:
			default:
			}
			return errors.New("boom")
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = batcher.Shutdown(context.WithoutCancel(t.Context()))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	md1 := singleDataPointMetric("m1")
	md2 := singleDataPointMetric("m1")
	requireMetricBatcherEnqueued(t, batcher, exp, md1)
	requireMetricBatcherEnqueued(t, batcher, exp, md2)

	select {
	case <-firstFailed:
	case <-time.After(2 * time.Second):
		t.Fatal("expected first timeout flush failure")
	}

	expected := mergePendingMetricChunks([]pmetric.Metrics{singleDataPointMetric("m1"), singleDataPointMetric("m1")})
	expectedBytes := (&pmetric.ProtoMarshaler{}).MetricsSize(expected)

	require.Eventually(t, func() bool {
		for _, p := range batcher.snapshotPending().pending {
			if p.endpoint == "endpoint-1:4317" {
				return p.datapoints == 2 && int(p.bytes) == expectedBytes
			}
		}
		return false
	}, time.Second, 20*time.Millisecond)
}

func TestMetricBatcherRecordsPendingOldestAgeAndFlushAge(t *testing.T) {
	telemetry := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, telemetry.Shutdown(context.WithoutCancel(t.Context())))
	})

	batcher, err := newMetricBatcher(
		componenttest.NewNopTelemetrySettings().Logger,
		telemetry.NewTelemetrySettings(),
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			return nil
		},
		nil,
	)
	require.NoError(t, err)

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))
	time.Sleep(25 * time.Millisecond)

	pendingMetric, err := telemetry.GetMetric("otelcol_loadbalancer_metric_batch_pending_oldest_datapoint_age")
	require.NoError(t, err)
	pendingGauge, ok := pendingMetric.Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Positive(t, findGaugePointValue(t, pendingGauge.DataPoints, attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"))))

	maxMetric, err := telemetry.GetMetric("otelcol_loadbalancer_metric_batch_pending_oldest_datapoint_age_max")
	require.NoError(t, err)
	maxGauge, ok := maxMetric.Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Positive(t, maxGauge.DataPoints[0].Value)

	require.NoError(t, batcher.Shutdown(t.Context()))

	flushMetric, err := telemetry.GetMetric("otelcol_loadbalancer_metric_batch_flush_oldest_datapoint_age")
	require.NoError(t, err)
	flushHistogram, ok := flushMetric.Data.(metricdata.Histogram[int64])
	require.True(t, ok)
	require.Len(t, flushHistogram.DataPoints, 1)
	require.Equal(t, uint64(1), flushHistogram.DataPoints[0].Count)
	require.Positive(t, flushHistogram.DataPoints[0].Sum)
}

func TestMetricBatcherHandleRequestUsesAcceptanceTimeForOldestAge(t *testing.T) {
	backend := &backendMetricBatcher{
		endpoint:  "endpoint-1:4317",
		settings:  metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour},
		requests:  make(chan metricBatcherRequest, 1),
		done:      make(chan struct{}),
		telemetry: &metricBatcherTelemetry{},
	}

	pending := make([]pmetric.Metrics, 0, 1)
	pendingDataPoints := 0
	pendingBytes := 0
	sizer := &pmetric.ProtoMarshaler{}
	var nextReq *metricBatcherRequest
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	var timerC <-chan time.Time
	retryingSince := time.Time{}

	start := time.Now()
	stopped := backend.handleRequest(
		metricBatcherRequest{
			kind:               metricBatcherRequestEnqueue,
			md:                 singleDataPointMetric("m1"),
			enqueuedAtUnixNano: start.UnixNano(),
		},
		sizer,
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&nextReq,
		timer,
		&timerC,
		&retryingSince,
	)
	require.False(t, stopped)

	stored := time.Unix(0, backend.oldestEnqueue.Load())
	require.False(t, stored.Before(start), "oldest timestamp should reflect request enqueue time")
}

func TestMetricBatcherHandleRequestUsesOldestDrainedEnqueueTime(t *testing.T) {
	backend := &backendMetricBatcher{
		endpoint:  "endpoint-1:4317",
		settings:  metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour},
		requests:  make(chan metricBatcherRequest, 1),
		done:      make(chan struct{}),
		telemetry: &metricBatcherTelemetry{},
	}

	older := time.Now().Add(-80 * time.Millisecond).UnixNano()
	newer := time.Now().Add(-20 * time.Millisecond).UnixNano()
	backend.requests <- metricBatcherRequest{
		kind:               metricBatcherRequestEnqueue,
		md:                 singleDataPointMetric("queued"),
		enqueuedAtUnixNano: older,
	}

	pending := make([]pmetric.Metrics, 0, 1)
	pendingDataPoints := 0
	pendingBytes := 0
	sizer := &pmetric.ProtoMarshaler{}
	var nextReq *metricBatcherRequest
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	var timerC <-chan time.Time
	retryingSince := time.Time{}

	stopped := backend.handleRequest(
		metricBatcherRequest{
			kind:               metricBatcherRequestEnqueue,
			md:                 singleDataPointMetric("current"),
			enqueuedAtUnixNano: newer,
		},
		sizer,
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&nextReq,
		timer,
		&timerC,
		&retryingSince,
	)
	require.False(t, stopped)
	require.Equal(t, 2, pendingDataPoints)
	require.Equal(t, older, backend.oldestEnqueue.Load())
}

func TestMetricBatcherFlushAgeRecordedOnlyAfterTerminalFlush(t *testing.T) {
	telemetry := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, telemetry.Shutdown(context.WithoutCancel(t.Context())))
	})

	var calls atomic.Int64
	batcher, err := newMetricBatcher(
		componenttest.NewNopTelemetrySettings().Logger,
		telemetry.NewTelemetrySettings(),
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: 20 * time.Millisecond},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			if calls.Add(1) == 1 {
				return errors.New("boom")
			}
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	var flushHistogram metricdata.Histogram[int64]
	require.Eventually(t, func() bool {
		flushMetric, metricErr := telemetry.GetMetric("otelcol_loadbalancer_metric_batch_flush_oldest_datapoint_age")
		if metricErr != nil {
			return false
		}
		histogram, ok := flushMetric.Data.(metricdata.Histogram[int64])
		if !ok || len(histogram.DataPoints) != 1 || histogram.DataPoints[0].Count == 0 {
			return false
		}
		flushHistogram = histogram
		return true
	}, 2*time.Second, 20*time.Millisecond)

	require.Equal(t, uint64(1), flushHistogram.DataPoints[0].Count)
	require.GreaterOrEqual(t, calls.Load(), int64(2))
}

func TestMetricBatcherTryEnqueueReturnsFalseWhenBackendQueueIsFull(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	sendStarted := make(chan struct{}, 1)
	blockSend := make(chan struct{})

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1, maxBytes: 1 << 20, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			select {
			case sendStarted <- struct{}{}:
			default:
			}
			<-blockSend
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		close(blockSend)
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	select {
	case <-sendStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected backend send to start")
	}

	for range 16 {
		enqueued, enqueueErr := batcher.TryEnqueue("endpoint-1:4317", exp, singleDataPointMetric("m1"))
		require.NoError(t, enqueueErr)
		require.True(t, enqueued)
	}

	enqueued, enqueueErr := batcher.TryEnqueue("endpoint-1:4317", exp, singleDataPointMetric("m1"))
	require.NoError(t, enqueueErr)
	require.False(t, enqueued)
}

func TestMetricBatcherPendingAgeIncludesTimeBufferedBeforeWorkerDequeues(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	sendStarted := make(chan struct{}, 1)
	unblockSend := make(chan struct{})
	sendReleased := false

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: 10 * time.Millisecond},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			select {
			case sendStarted <- struct{}{}:
			default:
			}
			<-unblockSend
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if !sendReleased {
			close(unblockSend)
		}
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

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

	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m2"))
	time.Sleep(60 * time.Millisecond)
	close(unblockSend)
	sendReleased = true

	var oldestAgeMillis int64
	require.Eventually(t, func() bool {
		for _, pending := range batcher.snapshotPending().pending {
			if pending.endpoint == "endpoint-1:4317" && pending.datapoints == 1 {
				oldestAgeMillis = pending.oldestAgeMillis
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	require.GreaterOrEqual(t, oldestAgeMillis, int64(40), "oldest age should include time spent buffered in backend.requests")
}

func TestMetricBatcherSnapshotPendingIgnoresOldestAgeWhenNoDatapointsPending(t *testing.T) {
	batcher := &metricBatcher{
		backends: map[string]*backendMetricBatcher{
			"endpoint-1:4317": {},
		},
	}
	batcher.backends["endpoint-1:4317"].oldestEnqueue.Store(time.Now().Add(-time.Second).UnixNano())
	batcher.backends["endpoint-1:4317"].pendingBytes.Store(123)

	snapshot := batcher.snapshotPending()
	require.Len(t, snapshot.pending, 1)
	require.Zero(t, snapshot.pending[0].datapoints)
	require.Zero(t, snapshot.pending[0].oldestAgeMillis)
	require.Zero(t, snapshot.maxOldestAgeMillis)
}

func TestMetricBatcherRemoveReroutesOnResolverChangeFlushFailure(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	rerouted := make(chan int, 1)

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			return errors.New("flush failed")
		},
		func(_ context.Context, md pmetric.Metrics, reason string) error {
			require.Equal(t, metricFlushReasonResolverChange, reason)
			rerouted <- md.DataPointCount()
			return nil
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	require.NoError(t, batcher.Remove(t.Context(), "endpoint-1:4317", exp))

	select {
	case got := <-rerouted:
		require.Equal(t, 1, got)
	case <-time.After(2 * time.Second):
		t.Fatal("expected resolver-change drain reroute callback")
	}
}

func TestMetricBatcherDropsPendingWhenRetryBufferCapExceeded(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var calls atomic.Int64
	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1, maxBytes: 1 << 20, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			calls.Add(1)
			return errors.New("still failing")
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	for range defaultMetricBatchRetryBufferMultiplier + 1 {
		requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))
	}

	require.Eventually(t, func() bool {
		return calls.Load() > 0
	}, 2*time.Second, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		for _, p := range batcher.snapshotPending().pending {
			if p.endpoint == "endpoint-1:4317" {
				return p.datapoints == 0
			}
		}
		return true
	}, 2*time.Second, 20*time.Millisecond)
}

func TestMetricBatcherDropsPendingWhenRetryAgeExceeded(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	var calls atomic.Int64

	oldRetryAge := metricBatcherMaxRetryAge
	metricBatcherMaxRetryAge = 30 * time.Millisecond
	t.Cleanup(func() {
		metricBatcherMaxRetryAge = oldRetryAge
	})

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: 10 * time.Millisecond},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			calls.Add(1)
			return errors.New("still failing")
		},
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	})

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	require.Eventually(t, func() bool {
		return calls.Load() >= 2
	}, 2*time.Second, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		for _, p := range batcher.snapshotPending().pending {
			if p.endpoint == "endpoint-1:4317" {
				return p.datapoints == 0
			}
		}
		return true
	}, 2*time.Second, 20*time.Millisecond)
}

func TestMetricBatcherShutdownReroutesOnShutdownFlushFailure(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	rerouted := make(chan int, 1)

	batcher, err := newMetricBatcher(
		ts.Logger,
		ts.TelemetrySettings,
		metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour},
		func(_ context.Context, _ *wrappedExporter, _ pmetric.Metrics, _ string) error {
			return errors.New("flush failed")
		},
		func(_ context.Context, md pmetric.Metrics, reason string) error {
			require.Equal(t, metricFlushReasonShutdown, reason)
			rerouted <- md.DataPointCount()
			return nil
		},
	)
	require.NoError(t, err)

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("m1"))

	require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))

	select {
	case got := <-rerouted:
		require.Equal(t, 1, got)
	case <-time.After(2 * time.Second):
		t.Fatal("expected shutdown drain reroute callback")
	}
}

func TestMergePendingMetricChunksMergesDistinctMetricNames(t *testing.T) {
	md1 := singleDataPointMetric("m1")
	md2 := singleDataPointMetric("m2")

	merged := mergePendingMetricChunks([]pmetric.Metrics{md1, md2})

	require.Equal(t, 2, merged.DataPointCount())
	require.Len(t, splitMetricsByMetricName(merged), 2)
}

func TestMergePendingMetricChunksMergesSameMetricNameDataPoints(t *testing.T) {
	md1 := singleDataPointMetric("m1")
	md2 := singleDataPointMetric("m1")

	merged := mergePendingMetricChunks([]pmetric.Metrics{md1, md2})

	require.Equal(t, 2, merged.DataPointCount())
	split := splitMetricsByMetricName(merged)
	require.Len(t, split, 1)
	require.Contains(t, split, "m1")
	require.Equal(t, 2, split["m1"].DataPointCount())
}

func TestMergePendingMetricChunksMergesByResourceScopeAndMetricIdentity(t *testing.T) {
	md1 := metricWithResourceScopeGauge("host-a", "scope-a", "m1", 1)
	md2 := metricWithResourceScopeGauge("host-a", "scope-a", "m1", 2)

	merged := mergePendingMetricChunks([]pmetric.Metrics{md1, md2})

	require.Equal(t, 1, merged.ResourceMetrics().Len())
	rm := merged.ResourceMetrics().At(0)
	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)
	require.Equal(t, 1, sm.Metrics().Len())
	g := sm.Metrics().At(0).Gauge()
	require.Equal(t, 2, g.DataPoints().Len())
}

func TestMergePendingMetricChunksKeepsDistinctResourcesSeparate(t *testing.T) {
	md1 := metricWithResourceScopeGauge("host-a", "scope-a", "m1", 1)
	md2 := metricWithResourceScopeGauge("host-b", "scope-a", "m1", 2)

	merged := mergePendingMetricChunks([]pmetric.Metrics{md1, md2})

	require.Equal(t, 2, merged.ResourceMetrics().Len())
}

func TestCompressedMetricBatcherEnqueueStoresCompressedRequests(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	sendStarted := make(chan struct{}, 1)
	unblockSend := make(chan struct{})

	batcher, err := newMetricBatcher(ts.Logger, ts.TelemetrySettings, metricBatcherSettings{
		maxDataPoints:      1,
		maxBytes:           1 << 20,
		flushInterval:      time.Hour,
		payloadCompression: QueuePayloadCompressionZstd,
	}, func(context.Context, *wrappedExporter, pmetric.Metrics, string) error {
		select {
		case sendStarted <- struct{}{}:
		default:
		}
		<-unblockSend
		return nil
	}, nil)
	require.NoError(t, err)
	defer func() {
		close(unblockSend)
		require.NoError(t, batcher.Shutdown(context.WithoutCancel(t.Context())))
	}()

	exp := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	requireMetricBatcherEnqueued(t, batcher, exp, singleDataPointMetric("first"))
	select {
	case <-sendStarted:
	case <-time.After(time.Second):
		t.Fatal("expected first batch to block in send")
	}

	enqueued, err := batcher.TryEnqueue("endpoint-1:4317", exp, compressibleMetrics(256))
	require.NoError(t, err)
	require.True(t, enqueued)

	batcher.mu.RLock()
	backend := batcher.backends["endpoint-1:4317"]
	batcher.mu.RUnlock()
	require.NotNil(t, backend)

	select {
	case req := <-backend.requests:
		require.Zero(t, req.md.DataPointCount())
		require.Equal(t, 256, req.compressedChunk.dataPoints)
		require.Positive(t, req.compressedChunk.compressedBytes)
		require.Less(t, req.compressedChunk.compressedBytes, req.compressedChunk.uncompressedBytes)
	case <-time.After(time.Second):
		t.Fatal("expected queued request to stay in compressed form while backend send is blocked")
	}
}

func TestCompressedMetricBatcherRetriesRemainCompressed(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	telemetry, err := newMetricBatcherTelemetry(ts.TelemetrySettings)
	require.NoError(t, err)
	t.Cleanup(telemetry.shutdown)

	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	backend := &backendMetricBatcher{
		endpoint:     "endpoint-1:4317",
		logger:       ts.Logger,
		settings:     metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour, maxRetryBufferMultiplier: defaultMetricBatchRetryBufferMultiplier},
		telemetry:    telemetry,
		payloadCodec: codec,
		send: func(context.Context, *wrappedExporter, pmetric.Metrics, string) error {
			return errors.New("temporary failure")
		},
	}
	chunk, err := newCompressedMetricBatcherChunk(&pmetric.ProtoMarshaler{}, codec, metricBatcherRequest{
		md:                 compressibleMetrics(16),
		enqueuedAtUnixNano: time.Now().UnixNano(),
	})
	require.NoError(t, err)
	pending := []compressedMetricBatcherChunk{chunk}
	pendingDataPoints := chunk.dataPoints
	pendingBytes := chunk.uncompressedBytes
	pendingCompressedBytes := chunk.compressedBytes
	timer := time.NewTimer(time.Hour)
	require.True(t, timer.Stop())
	var timerC <-chan time.Time
	retryingSince := time.Time{}

	err = backend.flushCompressed(
		t.Context(),
		&pmetric.ProtoUnmarshaler{},
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&pendingCompressedBytes,
		metricFlushReasonSize,
		timer,
		&timerC,
		&retryingSince,
	)

	require.ErrorContains(t, err, "temporary failure")
	require.Len(t, pending, 1)
	require.Equal(t, chunk.dataPoints, pending[0].dataPoints)
	require.Equal(t, chunk.compressedBytes, pending[0].compressedBytes)
	require.NotEmpty(t, pending[0].payload)
}

func TestCompressedMetricBatcherRerouteFailureRetriesOnlyFailedMetrics(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	telemetry, err := newMetricBatcherTelemetry(ts.TelemetrySettings)
	require.NoError(t, err)
	t.Cleanup(telemetry.shutdown)

	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	failedRerouteData := singleDataPointMetric("failed-reroute")
	backend := &backendMetricBatcher{
		endpoint:     "endpoint-1:4317",
		logger:       ts.Logger,
		settings:     metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour, maxRetryBufferMultiplier: defaultMetricBatchRetryBufferMultiplier},
		telemetry:    telemetry,
		payloadCodec: codec,
		send: func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, _ string) error {
			return metricBatcherRerouteableError{err: status.Error(codes.Unavailable, "backend down"), data: md}
		},
		drainErr: func(context.Context, pmetric.Metrics, string) error {
			return consumererror.NewMetrics(errors.New("reroute failed"), failedRerouteData)
		},
	}
	chunk, err := newCompressedMetricBatcherChunk(&pmetric.ProtoMarshaler{}, codec, metricBatcherRequest{
		md:                 singleDataPointMetric("original"),
		enqueuedAtUnixNano: time.Now().UnixNano(),
	})
	require.NoError(t, err)
	pending := []compressedMetricBatcherChunk{chunk}
	pendingDataPoints := chunk.dataPoints
	pendingBytes := chunk.uncompressedBytes
	pendingCompressedBytes := chunk.compressedBytes
	timer := time.NewTimer(time.Hour)
	require.True(t, timer.Stop())
	var timerC <-chan time.Time
	retryingSince := time.Time{}

	err = backend.flushCompressed(
		t.Context(),
		&pmetric.ProtoUnmarshaler{},
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&pendingCompressedBytes,
		metricFlushReasonSize,
		timer,
		&timerC,
		&retryingSince,
	)

	require.ErrorContains(t, err, "reroute failed")
	require.Len(t, pending, 1)
	require.Positive(t, pending[0].compressedBytes)
	retryMetrics, err := decodeCompressedMetricBatcherChunk(&pmetric.ProtoUnmarshaler{}, codec, pending[0])
	require.NoError(t, err)
	require.True(t, metricNameExists(retryMetrics, "failed-reroute"))
	require.False(t, metricNameExists(retryMetrics, "original"))
	require.Equal(t, failedRerouteData.DataPointCount(), pendingDataPoints)
}

func TestCompressedMetricBatcherRerouteSuccessClearsRetryState(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	telemetry, err := newMetricBatcherTelemetry(ts.TelemetrySettings)
	require.NoError(t, err)
	t.Cleanup(telemetry.shutdown)

	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	var reroutes atomic.Int64
	backend := &backendMetricBatcher{
		endpoint:     "endpoint-1:4317",
		logger:       ts.Logger,
		settings:     metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour, maxRetryBufferMultiplier: defaultMetricBatchRetryBufferMultiplier},
		telemetry:    telemetry,
		payloadCodec: codec,
		send: func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, _ string) error {
			return metricBatcherRerouteableError{err: status.Error(codes.Unavailable, "backend down"), data: md}
		},
		drainErr: func(context.Context, pmetric.Metrics, string) error {
			reroutes.Add(1)
			return nil
		},
		flushFailures: 3,
	}
	chunk, err := newCompressedMetricBatcherChunk(&pmetric.ProtoMarshaler{}, codec, metricBatcherRequest{
		md:                 singleDataPointMetric("original"),
		enqueuedAtUnixNano: time.Now().UnixNano(),
	})
	require.NoError(t, err)
	pending := []compressedMetricBatcherChunk{chunk}
	pendingDataPoints := chunk.dataPoints
	pendingBytes := chunk.uncompressedBytes
	pendingCompressedBytes := chunk.compressedBytes
	timer := time.NewTimer(time.Hour)
	require.True(t, timer.Stop())
	var timerC <-chan time.Time
	retryingSince := time.Now().Add(-time.Second)

	err = backend.flushCompressed(
		t.Context(),
		&pmetric.ProtoUnmarshaler{},
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&pendingCompressedBytes,
		metricFlushReasonSize,
		timer,
		&timerC,
		&retryingSince,
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), reroutes.Load())
	require.True(t, retryingSince.IsZero())
	require.Zero(t, backend.flushFailures)
	require.Empty(t, pending)
}

func TestCompressedMetricBatcherResolverChangeFallsBackToDrainErr(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	telemetry, err := newMetricBatcherTelemetry(ts.TelemetrySettings)
	require.NoError(t, err)
	t.Cleanup(telemetry.shutdown)

	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	var reroutedDataPoints atomic.Int64
	backend := &backendMetricBatcher{
		endpoint:     "endpoint-1:4317",
		logger:       ts.Logger,
		settings:     metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour, maxRetryBufferMultiplier: defaultMetricBatchRetryBufferMultiplier},
		telemetry:    telemetry,
		payloadCodec: codec,
		send: func(context.Context, *wrappedExporter, pmetric.Metrics, string) error {
			return errors.New("resolver changed")
		},
		drainErr: func(_ context.Context, md pmetric.Metrics, reason string) error {
			require.Equal(t, metricFlushReasonResolverChange, reason)
			reroutedDataPoints.Add(int64(md.DataPointCount()))
			return nil
		},
	}
	chunk, err := newCompressedMetricBatcherChunk(&pmetric.ProtoMarshaler{}, codec, metricBatcherRequest{
		md:                 singleDataPointMetric("original"),
		enqueuedAtUnixNano: time.Now().UnixNano(),
	})
	require.NoError(t, err)
	pending := []compressedMetricBatcherChunk{chunk}
	pendingDataPoints := chunk.dataPoints
	pendingBytes := chunk.uncompressedBytes
	pendingCompressedBytes := chunk.compressedBytes
	timer := time.NewTimer(time.Hour)
	require.True(t, timer.Stop())
	var timerC <-chan time.Time
	retryingSince := time.Time{}

	err = backend.flushCompressed(
		t.Context(),
		&pmetric.ProtoUnmarshaler{},
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&pendingCompressedBytes,
		metricFlushReasonResolverChange,
		timer,
		&timerC,
		&retryingSince,
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), reroutedDataPoints.Load())
	require.Empty(t, pending)
}

func TestCompressedMetricBatcherNonRetryDropRecordsOldestAge(t *testing.T) {
	telemetryReader := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, telemetryReader.Shutdown(context.WithoutCancel(t.Context())))
	})
	telemetry, err := newMetricBatcherTelemetry(telemetryReader.NewTelemetrySettings())
	require.NoError(t, err)
	t.Cleanup(telemetry.shutdown)

	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	backend := &backendMetricBatcher{
		endpoint:     "endpoint-1:4317",
		logger:       componenttest.NewNopTelemetrySettings().Logger,
		settings:     metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour, maxRetryBufferMultiplier: defaultMetricBatchRetryBufferMultiplier},
		telemetry:    telemetry,
		payloadCodec: codec,
		send: func(context.Context, *wrappedExporter, pmetric.Metrics, string) error {
			return errors.New("resolver changed")
		},
	}
	chunk, err := newCompressedMetricBatcherChunk(&pmetric.ProtoMarshaler{}, codec, metricBatcherRequest{
		md:                 singleDataPointMetric("original"),
		enqueuedAtUnixNano: time.Now().Add(-time.Second).UnixNano(),
	})
	require.NoError(t, err)
	pending := []compressedMetricBatcherChunk{chunk}
	pendingDataPoints := chunk.dataPoints
	pendingBytes := chunk.uncompressedBytes
	pendingCompressedBytes := chunk.compressedBytes
	timer := time.NewTimer(time.Hour)
	require.True(t, timer.Stop())
	var timerC <-chan time.Time
	retryingSince := time.Time{}

	err = backend.flushCompressed(
		t.Context(),
		&pmetric.ProtoUnmarshaler{},
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&pendingCompressedBytes,
		metricFlushReasonResolverChange,
		timer,
		&timerC,
		&retryingSince,
	)

	require.ErrorContains(t, err, "resolver changed")
	flushMetric, err := telemetryReader.GetMetric("otelcol_loadbalancer_metric_batch_flush_oldest_datapoint_age")
	require.NoError(t, err)
	flushHistogram, ok := flushMetric.Data.(metricdata.Histogram[int64])
	require.True(t, ok)
	require.Len(t, flushHistogram.DataPoints, 1)
	require.Equal(t, uint64(1), flushHistogram.DataPoints[0].Count)
}

func TestCompressedMetricBatcherNonRetryRerouteFailureDropsFailedSubset(t *testing.T) {
	telemetryReader := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, telemetryReader.Shutdown(context.WithoutCancel(t.Context())))
	})
	telemetry, err := newMetricBatcherTelemetry(telemetryReader.NewTelemetrySettings())
	require.NoError(t, err)
	t.Cleanup(telemetry.shutdown)

	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	backend := &backendMetricBatcher{
		endpoint:     "endpoint-1:4317",
		logger:       componenttest.NewNopTelemetrySettings().Logger,
		settings:     metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour, maxRetryBufferMultiplier: defaultMetricBatchRetryBufferMultiplier},
		telemetry:    telemetry,
		payloadCodec: codec,
		send: func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, _ string) error {
			return metricBatcherRerouteableError{err: status.Error(codes.Unavailable, "backend down"), data: md}
		},
		drainErr: func(context.Context, pmetric.Metrics, string) error {
			return consumererror.NewMetrics(errors.New("reroute failed"), singleDataPointMetric("failed-subset"))
		},
	}
	chunk, err := newCompressedMetricBatcherChunk(&pmetric.ProtoMarshaler{}, codec, metricBatcherRequest{
		md:                 compressibleMetrics(16),
		enqueuedAtUnixNano: time.Now().UnixNano(),
	})
	require.NoError(t, err)
	pending := []compressedMetricBatcherChunk{chunk}
	pendingDataPoints := chunk.dataPoints
	pendingBytes := chunk.uncompressedBytes
	pendingCompressedBytes := chunk.compressedBytes
	timer := time.NewTimer(time.Hour)
	require.True(t, timer.Stop())
	var timerC <-chan time.Time
	retryingSince := time.Time{}

	err = backend.flushCompressed(
		t.Context(),
		&pmetric.ProtoUnmarshaler{},
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&pendingCompressedBytes,
		metricFlushReasonResolverChange,
		timer,
		&timerC,
		&retryingSince,
	)

	require.ErrorContains(t, err, "reroute failed")
	droppedMetric, err := telemetryReader.GetMetric("otelcol_loadbalancer_metric_batch_dropped_datapoints")
	require.NoError(t, err)
	droppedDataPoints, ok := sumInt64Metric(droppedMetric)
	require.True(t, ok)
	require.Equal(t, int64(1), droppedDataPoints)
}

func TestCompressedMetricBatcherRetryRecompressFailureDropsFailedSubset(t *testing.T) {
	telemetryReader := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, telemetryReader.Shutdown(context.WithoutCancel(t.Context())))
	})
	telemetry, err := newMetricBatcherTelemetry(telemetryReader.NewTelemetrySettings())
	require.NoError(t, err)
	t.Cleanup(telemetry.shutdown)

	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	backend := &backendMetricBatcher{
		endpoint:     "endpoint-1:4317",
		logger:       componenttest.NewNopTelemetrySettings().Logger,
		settings:     metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour, maxRetryBufferMultiplier: defaultMetricBatchRetryBufferMultiplier},
		telemetry:    telemetry,
		payloadCodec: codec,
		send: func(_ context.Context, _ *wrappedExporter, md pmetric.Metrics, _ string) error {
			return metricBatcherRerouteableError{err: status.Error(codes.Unavailable, "backend down"), data: md}
		},
	}
	backend.drainErr = func(context.Context, pmetric.Metrics, string) error {
		backend.payloadCodec = newQueuePayloadCodec(QueuePayloadCompression("invalid"))
		return consumererror.NewMetrics(errors.New("reroute failed"), singleDataPointMetric("failed-subset"))
	}
	chunk, err := newCompressedMetricBatcherChunk(&pmetric.ProtoMarshaler{}, codec, metricBatcherRequest{
		md:                 compressibleMetrics(16),
		enqueuedAtUnixNano: time.Now().UnixNano(),
	})
	require.NoError(t, err)
	pending := []compressedMetricBatcherChunk{chunk}
	pendingDataPoints := chunk.dataPoints
	pendingBytes := chunk.uncompressedBytes
	pendingCompressedBytes := chunk.compressedBytes
	timer := time.NewTimer(time.Hour)
	require.True(t, timer.Stop())
	var timerC <-chan time.Time
	retryingSince := time.Time{}

	err = backend.flushCompressed(
		t.Context(),
		&pmetric.ProtoUnmarshaler{},
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&pendingCompressedBytes,
		metricFlushReasonSize,
		timer,
		&timerC,
		&retryingSince,
	)

	require.ErrorContains(t, err, "reroute failed")
	droppedMetric, err := telemetryReader.GetMetric("otelcol_loadbalancer_metric_batch_dropped_datapoints")
	require.NoError(t, err)
	droppedDataPoints, ok := sumInt64Metric(droppedMetric)
	require.True(t, ok)
	require.Equal(t, int64(1), droppedDataPoints)
}

func TestCompressedMetricBatcherDropsAfterRetryAge(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	telemetry, err := newMetricBatcherTelemetry(ts.TelemetrySettings)
	require.NoError(t, err)
	t.Cleanup(telemetry.shutdown)

	oldMaxRetryAge := metricBatcherMaxRetryAge
	metricBatcherMaxRetryAge = 0
	t.Cleanup(func() { metricBatcherMaxRetryAge = oldMaxRetryAge })

	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	backend := &backendMetricBatcher{
		endpoint:     "endpoint-1:4317",
		logger:       ts.Logger,
		settings:     metricBatcherSettings{maxDataPoints: 1000, maxBytes: 1 << 20, flushInterval: time.Hour, maxRetryBufferMultiplier: defaultMetricBatchRetryBufferMultiplier},
		telemetry:    telemetry,
		payloadCodec: codec,
		send: func(context.Context, *wrappedExporter, pmetric.Metrics, string) error {
			return errors.New("temporary failure")
		},
	}
	chunk, err := newCompressedMetricBatcherChunk(&pmetric.ProtoMarshaler{}, codec, metricBatcherRequest{
		md:                 compressibleMetrics(16),
		enqueuedAtUnixNano: time.Now().UnixNano(),
	})
	require.NoError(t, err)
	pending := []compressedMetricBatcherChunk{chunk}
	pendingDataPoints := chunk.dataPoints
	pendingBytes := chunk.uncompressedBytes
	pendingCompressedBytes := chunk.compressedBytes
	timer := time.NewTimer(time.Hour)
	require.True(t, timer.Stop())
	var timerC <-chan time.Time
	retryingSince := time.Now().Add(-time.Second)

	err = backend.flushCompressed(
		t.Context(),
		&pmetric.ProtoUnmarshaler{},
		&pending,
		&pendingDataPoints,
		&pendingBytes,
		&pendingCompressedBytes,
		metricFlushReasonSize,
		timer,
		&timerC,
		&retryingSince,
	)

	require.ErrorContains(t, err, "temporary failure")
	require.Empty(t, pending)
	require.Zero(t, pendingDataPoints)
	require.Zero(t, pendingBytes)
	require.Zero(t, pendingCompressedBytes)
	require.True(t, retryingSince.IsZero())
}

func singleDataPointMetric(name string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	g := m.SetEmptyGauge()
	g.DataPoints().AppendEmpty().SetIntValue(1)
	return md
}

func metricNameExists(md pmetric.Metrics, name string) bool {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				if sm.Metrics().At(k).Name() == name {
					return true
				}
			}
		}
	}
	return false
}

func metricWithResourceScopeGauge(resourceValue, scopeName, metricName string, value int64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("host.name", resourceValue)
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName(scopeName)
	m := sm.Metrics().AppendEmpty()
	m.SetName(metricName)
	g := m.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetIntValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(value, 0)))
	return md
}

func requireMetricBatcherEnqueued(t *testing.T, batcher *metricBatcher, exp *wrappedExporter, md pmetric.Metrics) {
	t.Helper()
	enqueued, err := batcher.TryEnqueue("endpoint-1:4317", exp, md)
	require.NoError(t, err)
	require.True(t, enqueued)
}
