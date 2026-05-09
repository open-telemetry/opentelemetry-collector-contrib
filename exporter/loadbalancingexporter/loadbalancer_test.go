// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadatatest"
)

func TestNewLoadBalancerNoResolver(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{}

	// test
	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)

	// verify
	require.Nil(t, p)
	require.Equal(t, errNoResolver, err)
}

func TestNewLoadBalancerInvalidStaticResolver(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{}}),
		},
	}

	// test
	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)

	// verify
	require.Nil(t, p)
	require.Equal(t, errNoEndpoints, err)
}

func TestNewLoadBalancerInvalidDNSResolver(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{
				Hostname: "",
			}),
		},
	}

	// test
	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)

	// verify
	require.Nil(t, p)
	require.Equal(t, errNoHostname, err)
}

func TestNewLoadBalancerInvalidK8sResolver(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			K8sSvc: configoptional.Some(K8sSvcResolver{
				Service: "",
			}),
		},
	}

	// test
	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)

	// verify
	assert.Nil(t, p)
	assert.True(t, clientcmd.IsConfigurationInvalid(err) || errors.Is(err, errNoSvc))
}

func TestLoadBalancerStart(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()

	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)
	require.NotNil(t, p)
	require.NoError(t, err)
	p.res = &mockResolver{}

	// test
	res := p.Start(t.Context(), componenttest.NewNopHost())
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()
	// verify
	assert.NoError(t, res)
}

func TestWithDNSResolver(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{
				Hostname: "service-1",
			}),
		},
	}

	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res, ok := p.res.(*dnsResolver)

	// verify
	assert.NotNil(t, res)
	assert.True(t, ok)
}

func TestWithDNSResolverNoEndpoints(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{
				Hostname: "service-1",
			}),
		},
	}

	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)
	require.NotNil(t, p)
	require.NoError(t, err)
	res, ok := p.res.(*dnsResolver)
	require.True(t, ok)
	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			return nil, nil
		},
	}

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	// test
	_, e, _ := p.exporterAndEndpoint([]byte{128, 128, 0, 0})

	// verify
	assert.Empty(t, e)
}

func TestRandomExporterAndEndpointUsesEligibleRing(t *testing.T) {
	lb := &loadBalancer{
		ring:           newHashRing([]string{"eligible:4317"}),
		endpointHealth: newEndpointHealthManager(endpointHealthSettings{}),
		exporters: map[string]*wrappedExporter{
			"stale:4317": newWrappedExporter(mockComponent{}, "stale:4317"),
		},
	}

	_, endpoint, err := lb.randomExporterAndEndpoint()

	require.ErrorContains(t, err, "eligible:4317")
	require.Empty(t, endpoint)
}

func TestMultipleResolvers(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{
				Hostnames: []string{"endpoint-1", "endpoint-2"},
			}),
			DNS: configoptional.Some(DNSResolver{
				Hostname: "service-1",
			}),
		},
	}

	// test
	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)

	// verify
	assert.Nil(t, p)
	assert.Equal(t, errMultipleResolversProvided, err)
}

func TestStartFailureStaticResolver(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()

	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	expectedErr := errors.New("some expected err")
	p.res = &mockResolver{
		onStart: func(context.Context) error {
			return expectedErr
		},
	}

	// test
	res := p.Start(t.Context(), componenttest.NewNopHost())

	// verify
	assert.Equal(t, expectedErr, res)
}

func TestLoadBalancerShutdown(t *testing.T) {
	// prepare
	cfg := simpleConfig()
	p, err := newTracesExporter(exportertest.NewNopSettings(metadata.Type), cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(t.Context())

	// verify
	assert.NoError(t, res)
}

func TestOnBackendChanges(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	p.onBackendChanges([]string{"endpoint-1"})
	require.Len(t, p.ring.items, defaultWeight)

	// this should resolve to two endpoints
	endpoints := []string{"endpoint-1", "endpoint-2"}
	p.onBackendChanges(endpoints)

	// verify
	assert.Len(t, p.ring.items, 2*defaultWeight)
}

func TestRemoveExtraExporters(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.addMissingExporters(t.Context(), []string{"endpoint-1", "endpoint-2"})
	resolved := []string{"endpoint-1"}

	// test
	removed := p.removeExtraExportersLocked(resolved)
	p.drainRemovedExporters(t.Context(), removed)

	// verify
	assert.Len(t, p.exporters, 1)
	assert.NotContains(t, p.exporters, endpointWithPort("endpoint-2"))
}

func TestLoadBalancerShutdownWaitsForAsyncRemovedExporterCleanup(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	cleanupStarted := make(chan struct{})
	releaseCleanup := make(chan struct{})
	exporterShutdownStarted := make(chan struct{})
	releaseExporterShutdown := make(chan struct{})
	var releaseCleanupOnce sync.Once
	var releaseExporterShutdownOnce sync.Once
	defer releaseCleanupOnce.Do(func() { close(releaseCleanup) })
	defer releaseExporterShutdownOnce.Do(func() { close(releaseExporterShutdown) })

	p.onExporterRemove = func(context.Context, string, *wrappedExporter) error {
		close(cleanupStarted)
		<-releaseCleanup
		return nil
	}
	p.exporters["endpoint-1:4317"] = newWrappedExporter(&blockingShutdownComponent{
		started: exporterShutdownStarted,
		release: releaseExporterShutdown,
	}, "endpoint-1:4317")

	p.cleanupBackendWithoutDrain(t.Context(), "endpoint-1:4317")
	require.Eventually(t, func() bool {
		select {
		case <-cleanupStarted:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	shutdownDone := make(chan error, 1)
	var shutdownReturned atomic.Bool
	go func() {
		err := p.Shutdown(t.Context())
		shutdownReturned.Store(true)
		shutdownDone <- err
	}()

	assert.Never(t, shutdownReturned.Load, 50*time.Millisecond, 10*time.Millisecond)

	releaseCleanupOnce.Do(func() { close(releaseCleanup) })
	require.Eventually(t, func() bool {
		select {
		case <-exporterShutdownStarted:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
	assert.Never(t, shutdownReturned.Load, 50*time.Millisecond, 10*time.Millisecond)

	releaseExporterShutdownOnce.Do(func() { close(releaseExporterShutdown) })
	require.NoError(t, <-shutdownDone)
}

func TestAddMissingExporters(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	exporterFactory := exporter.NewFactory(component.MustNewType("otlp"), func() component.Config {
		return &otlpexporter.Config{}
	}, exporter.WithTraces(func(
		_ context.Context,
		_ exporter.Settings,
		_ component.Config,
	) (exporter.Traces, error) {
		return newNopMockTracesExporter(), nil
	}, component.StabilityLevelDevelopment))
	fn := func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := cfg.Protocol.OTLP
		oCfg.ClientConfig.Endpoint = endpoint
		return exporterFactory.CreateTraces(ctx, exportertest.NewNopSettings(exporterFactory.Type()), &oCfg)
	}

	p, err := newLoadBalancer(ts.Logger, cfg, fn, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.exporters["endpoint-1:4317"] = newNopMockExporter()
	resolved := []string{"endpoint-1", "endpoint-2"}

	// test
	p.addMissingExporters(t.Context(), resolved)

	// verify
	assert.Len(t, p.exporters, 2)
	assert.Contains(t, p.exporters, "endpoint-2:4317")
}

func TestFailedToAddMissingExporters(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	expectedErr := errors.New("some expected error")
	exporterFactory := exporter.NewFactory(component.MustNewType("otlp"), func() component.Config {
		return &otlpexporter.Config{}
	}, exporter.WithTraces(func(
		_ context.Context,
		_ exporter.Settings,
		_ component.Config,
	) (exporter.Traces, error) {
		return nil, expectedErr
	}, component.StabilityLevelDevelopment))
	fn := func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := cfg.Protocol.OTLP
		oCfg.ClientConfig.Endpoint = endpoint
		return exporterFactory.CreateTraces(ctx, exportertest.NewNopSettings(metadata.Type), &oCfg)
	}

	p, err := newLoadBalancer(ts.Logger, cfg, fn, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.exporters["endpoint-1:4317"] = newNopMockExporter()
	resolved := []string{"endpoint-1", "endpoint-2"}

	// test
	p.addMissingExporters(t.Context(), resolved)

	// verify
	assert.Len(t, p.exporters, 1)
	assert.Contains(t, p.exporters, "endpoint-1:4317")
}

func TestLoadBalancerEndpointHealthExcludesQuarantinedEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	require.Contains(t, p.exporters, "endpoint-1:4317")
	require.Contains(t, p.exporters, "endpoint-2:4317")

	decision := p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.endpointLocal)
	require.True(t, decision.quarantined)
	require.NotContains(t, p.exporters, "endpoint-1:4317")
	require.Contains(t, p.exporters, "endpoint-2:4317")

	for i := range 100 {
		_, endpoint, err := p.exporterAndEndpoint([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)})
		require.NoError(t, err)
		assert.NotEqual(t, "endpoint-1:4317", endpoint)
	}
}

func TestLoadBalancerEndpointHealthActiveProbeExcludesAndRecoversEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.ActiveProbe = EndpointHealthActiveProbeConfig{
		Enabled:        true,
		Type:           EndpointHealthActiveProbeTypeTCPConnect,
		Interval:       time.Second,
		Timeout:        100 * time.Millisecond,
		Jitter:         "0%",
		MaxConcurrency: 2,
		Fall:           2,
		Rise:           2,
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	routingID := findRoutingIDForEndpoint(t, p.ring, "endpoint-1:4317")

	decision := p.handleBackendProbeFailure(t.Context(), "endpoint-1:4317", errors.New("probe failed"))
	require.False(t, decision.quarantined)
	require.Contains(t, p.exporters, "endpoint-1:4317")

	decision = p.handleBackendProbeFailure(t.Context(), "endpoint-1:4317", errors.New("probe failed"))
	require.True(t, decision.quarantined)
	require.NotContains(t, p.exporters, "endpoint-1:4317")

	_, endpoint, err := p.exporterAndEndpoint([]byte(routingID))
	require.NoError(t, err)
	require.Equal(t, "endpoint-2:4317", endpoint)

	success := p.handleBackendProbeSuccess(t.Context(), "endpoint-1:4317")
	require.False(t, success.recovered)
	require.NotContains(t, p.exporters, "endpoint-1:4317")

	success = p.handleBackendProbeSuccess(t.Context(), "endpoint-1:4317")
	require.True(t, success.recovered)
	require.Contains(t, p.exporters, "endpoint-1:4317")

	_, endpoint, err = p.exporterAndEndpoint([]byte(routingID))
	require.NoError(t, err)
	require.Equal(t, "endpoint-1:4317", endpoint)
}

func TestLoadBalancerEndpointHealthActiveProbeFailsOpenWhenEveryEndpointIsUnhealthy(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.ActiveProbe = EndpointHealthActiveProbeConfig{
		Enabled:        true,
		Type:           EndpointHealthActiveProbeTypeTCPConnect,
		Interval:       time.Second,
		Timeout:        100 * time.Millisecond,
		Jitter:         "0%",
		MaxConcurrency: 2,
		Fall:           1,
		Rise:           1,
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	endpoint1Original := p.exporters["endpoint-1:4317"]
	endpoint2Original := p.exporters["endpoint-2:4317"]

	decision := p.handleBackendProbeFailure(t.Context(), "endpoint-1:4317", errors.New("probe failed"))
	require.True(t, decision.quarantined)
	require.False(t, decision.failOpen)

	decision = p.handleBackendProbeFailure(t.Context(), "endpoint-2:4317", errors.New("probe failed"))
	require.True(t, decision.quarantined)
	require.True(t, decision.failOpen)
	require.Contains(t, decision.eligible, "endpoint-1:4317")
	require.Contains(t, decision.eligible, "endpoint-2:4317")
	require.NotSame(t, endpoint1Original, p.exporters["endpoint-1:4317"])
	require.NotSame(t, endpoint2Original, p.exporters["endpoint-2:4317"])
}

func TestLoadBalancerEndpointHealthActiveProbeUpdatesExistingHealthMetrics(t *testing.T) {
	ts, tb, telemetry := getTelemetryAssetsWithReader(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.ActiveProbe = EndpointHealthActiveProbeConfig{
		Enabled:        true,
		Type:           EndpointHealthActiveProbeTypeTCPConnect,
		Interval:       time.Second,
		Timeout:        100 * time.Millisecond,
		Jitter:         "0%",
		MaxConcurrency: 2,
		Fall:           1,
		Rise:           1,
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	p.handleBackendProbeFailure(t.Context(), "endpoint-1:4317", errors.New("probe failed"))
	p.handleBackendProbeSuccess(t.Context(), "endpoint-1:4317")

	metadatatest.AssertEqualLoadbalancerBackendQuarantineTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("reason", "active_probe")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualLoadbalancerBackendUnquarantineTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("reason", "active_probe")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualLoadbalancerBackendState(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("state", "eligible")),
			Value:      1,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("state", "quarantined")),
			Value:      0,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("state", "stale")),
			Value:      0,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-2:4317"), attribute.String("state", "eligible")),
			Value:      1,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-2:4317"), attribute.String("state", "quarantined")),
			Value:      0,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-2:4317"), attribute.String("state", "stale")),
			Value:      0,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestLoadBalancerEndpointHealthProbeRecoveryKeepsActiveTransportQuarantineMetrics(t *testing.T) {
	ts, tb, telemetry := getTelemetryAssetsWithReader(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.ActiveProbe = EndpointHealthActiveProbeConfig{
		Enabled:        true,
		Type:           EndpointHealthActiveProbeTypeTCPConnect,
		Interval:       time.Second,
		Timeout:        100 * time.Millisecond,
		Jitter:         "0%",
		MaxConcurrency: 2,
		Fall:           1,
		Rise:           1,
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}
	now := time.Unix(100, 0)

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	p.endpointHealth.settings.now = func() time.Time { return now }
	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})

	transportFailure := p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, transportFailure.quarantined)
	probeFailure := p.handleBackendProbeFailure(t.Context(), "endpoint-1:4317", errors.New("probe failed"))
	require.True(t, probeFailure.quarantined)

	probeSuccess := p.handleBackendProbeSuccess(t.Context(), "endpoint-1:4317")
	require.False(t, probeSuccess.recovered)
	require.NotContains(t, p.exporters, "endpoint-1:4317")
	_, err = telemetry.GetMetric("otelcol_loadbalancer_backend_unquarantine_total")
	require.Error(t, err)
	metadatatest.AssertEqualLoadbalancerBackendState(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("state", "eligible")),
			Value:      0,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("state", "quarantined")),
			Value:      1,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("state", "stale")),
			Value:      0,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-2:4317"), attribute.String("state", "eligible")),
			Value:      1,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-2:4317"), attribute.String("state", "quarantined")),
			Value:      0,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-2:4317"), attribute.String("state", "stale")),
			Value:      0,
		},
	}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())

	p.endpointHealth.mu.RLock()
	state := p.endpointHealth.endpoints["endpoint-1:4317"]
	quarantinedUntil := state.quarantinedUntil
	failureReason := state.failureReason
	p.endpointHealth.mu.RUnlock()
	require.Equal(t, endpointFailureUnavailable, failureReason)

	now = quarantinedUntil.Add(time.Nanosecond)
	require.True(t, p.endpointHealth.quarantineRefreshDue())
	p.refreshExpiredEndpointHealth(t.Context())
	require.Contains(t, p.exporters, "endpoint-1:4317")
	metadatatest.AssertEqualLoadbalancerBackendUnquarantineTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("reason", string(endpointFailureUnavailable))),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestLoadBalancerEndpointHealthActiveProbeCycleUsesTCPConnectAndRecoversEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.ActiveProbe = EndpointHealthActiveProbeConfig{
		Enabled:        true,
		Type:           EndpointHealthActiveProbeTypeTCPConnect,
		Interval:       time.Second,
		Timeout:        100 * time.Millisecond,
		Jitter:         "0%",
		MaxConcurrency: 2,
		Fall:           1,
		Rise:           1,
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	reachable := listenTCPForProbe(t, "127.0.0.1:0")
	unreachableAddr := reserveTCPAddr(t)

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{reachable.Addr().String(), unreachableAddr})
	routingID := findRoutingIDForEndpoint(t, p.ring, unreachableAddr)

	p.runEndpointHealthActiveProbeCycle(t.Context())

	require.NotContains(t, p.exporters, unreachableAddr)
	_, endpoint, err := p.exporterAndEndpoint([]byte(routingID))
	require.NoError(t, err)
	require.NotEqual(t, unreachableAddr, endpoint)

	recovered := listenTCPForProbe(t, unreachableAddr)
	defer recovered.Close()

	p.runEndpointHealthActiveProbeCycle(t.Context())

	require.Contains(t, p.exporters, unreachableAddr)
	_, endpoint, err = p.exporterAndEndpoint([]byte(routingID))
	require.NoError(t, err)
	require.Equal(t, unreachableAddr, endpoint)
}

func TestLoadBalancerEndpointHealthActiveProbeCycleHonorsMaxConcurrency(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.ActiveProbe = EndpointHealthActiveProbeConfig{
		Enabled:        true,
		Type:           EndpointHealthActiveProbeTypeTCPConnect,
		Interval:       time.Second,
		Timeout:        100 * time.Millisecond,
		Jitter:         "0%",
		MaxConcurrency: 2,
		Fall:           1,
		Rise:           1,
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	p.onBackendChanges([]string{"endpoint-1", "endpoint-2", "endpoint-3", "endpoint-4"})

	var active atomic.Int64
	var maxActive atomic.Int64
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	p.activeProbeFunc = func(ctx context.Context, _ string) error {
		current := active.Add(1)
		for {
			observedMax := maxActive.Load()
			if current <= observedMax || maxActive.CompareAndSwap(observedMax, current) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		active.Add(-1)
		return nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.runEndpointHealthActiveProbeCycle(t.Context())
	}()

	requireProbeStarts(t, started, 2)
	select {
	case <-started:
		t.Fatal("active_probe exceeded max_concurrency before a probe completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("active probe cycle did not finish")
	}
	require.LessOrEqual(t, maxActive.Load(), int64(2))
}

func TestLoadBalancerEndpointHealthActiveProbeLoopStopsOnShutdown(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.ActiveProbe = EndpointHealthActiveProbeConfig{
		Enabled:        true,
		Type:           EndpointHealthActiveProbeTypeTCPConnect,
		Interval:       10 * time.Millisecond,
		Timeout:        time.Minute,
		Jitter:         "0%",
		MaxConcurrency: 1,
		Fall:           1,
		Rise:           1,
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	p.onBackendChanges([]string{"endpoint-1"})

	probeStarted := make(chan struct{})
	var startedOnce sync.Once
	p.activeProbeFunc = func(ctx context.Context, _ string) error {
		startedOnce.Do(func() { close(probeStarted) })
		<-ctx.Done()
		return ctx.Err()
	}

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	select {
	case <-probeStarted:
	case <-time.After(time.Second):
		t.Fatal("expected active probe loop to start")
	}

	shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, p.Shutdown(shutdownCtx))
}

func TestLoadBalancerEndpointHealthActiveProbeLoopWaitsBeforeFirstCycle(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.ActiveProbe = EndpointHealthActiveProbeConfig{
		Enabled:        true,
		Type:           EndpointHealthActiveProbeTypeTCPConnect,
		Interval:       200 * time.Millisecond,
		Timeout:        50 * time.Millisecond,
		Jitter:         "0%",
		MaxConcurrency: 1,
		Fall:           1,
		Rise:           1,
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	p.onBackendChanges([]string{"endpoint-1"})

	probeStarted := make(chan struct{}, 1)
	p.activeProbeFunc = func(context.Context, string) error {
		probeStarted <- struct{}{}
		return nil
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.runEndpointHealthActiveProbeLoop(ctx)
	}()

	select {
	case <-probeStarted:
		cancel()
		<-done
		t.Fatal("active probe loop started before the first interval elapsed")
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("active probe loop did not stop after cancellation")
	}
}

func TestLoadBalancerEndpointHealthRemovesDNSStaleEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	var shutdowns sync.Map
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return &countingComponent{endpoint: endpoint, shutdowns: &shutdowns}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	p.onBackendChanges([]string{"endpoint-2"})

	require.NotContains(t, p.exporters, "endpoint-1:4317")
	require.Contains(t, p.exporters, "endpoint-2:4317")
	require.Eventually(t, func() bool {
		count, ok := shutdowns.Load("endpoint-1:4317")
		return ok && count.(*atomic.Int64).Load() > 0
	}, time.Second, 10*time.Millisecond)
}

func TestLoadBalancerEndpointHealthFailOpenRefreshesFailedExporter(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	endpoint1Original := p.exporters["endpoint-1:4317"]
	endpoint2Original := p.exporters["endpoint-2:4317"]

	decision := p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	require.False(t, decision.failOpen)

	decision = p.handleBackendFailure(t.Context(), "endpoint-2:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	require.True(t, decision.failOpen)
	require.Contains(t, decision.eligible, "endpoint-1:4317")
	require.Contains(t, decision.eligible, "endpoint-2:4317")
	require.NotSame(t, endpoint1Original, p.exporters["endpoint-1:4317"])
	require.NotSame(t, endpoint2Original, p.exporters["endpoint-2:4317"])
}

func TestLoadBalancerEndpointHealthFailOpenKeepsExporterWhenRefreshFails(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)

	var endpoint2Creations atomic.Int64
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		if endpoint == "endpoint-2:4317" && endpoint2Creations.Add(1) > 1 {
			return nil, errors.New("endpoint-2 refresh failed")
		}
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	endpoint2Original := p.exporters["endpoint-2:4317"]

	decision := p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	require.False(t, decision.failOpen)

	decision = p.handleBackendFailure(t.Context(), "endpoint-2:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.failOpen)
	require.Same(t, endpoint2Original, p.exporters["endpoint-2:4317"])

	routingID := findRoutingIDForEndpoint(t, p.ring, "endpoint-2:4317")
	exporter, endpoint, err := p.exporterAndEndpoint([]byte(routingID))
	require.NoError(t, err)
	require.Equal(t, "endpoint-2:4317", endpoint)
	require.Same(t, endpoint2Original, exporter)
}

func TestShouldCommitEndpointHealthFailureNormalizesEligibleEndpoints(t *testing.T) {
	decision := endpointHealthFailureDecision{
		endpointLocal: true,
		eligible:      []string{"endpoint-1"},
	}

	require.False(t, shouldCommitEndpointHealthFailure("endpoint-1:4317", decision))
	require.True(t, shouldCommitEndpointHealthFailure("endpoint-2:4317", decision))
}

func TestLoadBalancerEndpointHealthHealthOnlyFailOpenRefreshesFailedExporter(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	endpoint1Original := p.exporters["endpoint-1:4317"]
	endpoint2Original := p.exporters["endpoint-2:4317"]

	decision := p.handleBackendFailureHealthOnly(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	require.False(t, decision.failOpen)
	if shouldRerouteDirectFailure(p, "endpoint-1:4317", decision, 0) {
		p.cleanupBackendWithoutDrain(t.Context(), "endpoint-1:4317")
	}

	decision = p.handleBackendFailureHealthOnly(t.Context(), "endpoint-2:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	require.True(t, decision.failOpen)
	require.Contains(t, decision.eligible, "endpoint-1:4317")
	require.Contains(t, decision.eligible, "endpoint-2:4317")
	require.NotSame(t, endpoint1Original, p.exporters["endpoint-1:4317"])
	require.NotSame(t, endpoint2Original, p.exporters["endpoint-2:4317"])
}

func TestLoadBalancerEndpointHealthHealthOnlyFailOpenKeepsExporterWhenRefreshFails(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)

	var endpoint2Creations atomic.Int64
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		if endpoint == "endpoint-2:4317" && endpoint2Creations.Add(1) > 1 {
			return nil, errors.New("endpoint-2 refresh failed")
		}
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	endpoint2Original := p.exporters["endpoint-2:4317"]

	decision := p.handleBackendFailureHealthOnly(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	if shouldRerouteDirectFailure(p, "endpoint-1:4317", decision, 0) {
		p.cleanupBackendWithoutDrain(t.Context(), "endpoint-1:4317")
	}

	decision = p.handleBackendFailureHealthOnly(t.Context(), "endpoint-2:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.failOpen)
	require.Same(t, endpoint2Original, p.exporters["endpoint-2:4317"])

	routingID := findRoutingIDForEndpoint(t, p.ring, "endpoint-2:4317")
	exporter, endpoint, err := p.exporterAndEndpoint([]byte(routingID))
	require.NoError(t, err)
	require.Equal(t, "endpoint-2:4317", endpoint)
	require.Same(t, endpoint2Original, exporter)
}

func TestLoadBalancerEndpointHealthSuccessRecoversFailOpenRing(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	decision := p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	decision = p.handleBackendFailure(t.Context(), "endpoint-2:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.failOpen)

	p.handleBackendSuccess("endpoint-1:4317")

	require.False(t, p.endpointHealth.failOpen())
	for i := range 100 {
		_, endpoint, err := p.exporterAndEndpoint([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)})
		require.NoError(t, err)
		assert.Equal(t, "endpoint-1:4317", endpoint)
	}
}

func TestLoadBalancerEndpointHealthHealthySuccessDoesNotRebuildRing(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	var created atomic.Int64
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		created.Add(1)
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	require.Equal(t, int64(2), created.Load())
	created.Store(0)
	originalRing := p.ring

	p.handleBackendSuccess("endpoint-1:4317")

	require.Zero(t, created.Load())
	require.Same(t, originalRing, p.ring)
}

func TestLoadBalancerEndpointHealthRebuildsRingAfterQuarantineExpires(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	cfg.EndpointHealth.QuarantineDuration = 30 * time.Second
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}
	now := time.Unix(100, 0)

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	p.endpointHealth.settings.now = func() time.Time { return now }

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	routingID := findRoutingIDForEndpoint(t, p.ring, "endpoint-1:4317")

	decision := p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	require.NotContains(t, p.exporters, "endpoint-1:4317")

	now = now.Add(31 * time.Second)

	_, endpoint, err := p.exporterAndEndpoint([]byte(routingID))
	require.NoError(t, err)
	require.Equal(t, "endpoint-1:4317", endpoint)
	require.Contains(t, p.exporters, "endpoint-1:4317")
}

func TestLoadBalancerEndpointHealthReconcileClearsStaleGaugeOnReadmit(t *testing.T) {
	_, tb, telemetry := getTelemetryAssetsWithReader(t)
	lb := &loadBalancer{telemetry: tb}

	lb.recordEndpointStale(t.Context(), "endpoint-1:4317")
	lb.recordEndpointHealthReconcile(t.Context(), endpointHealthReconcileResult{
		eligible: []string{"endpoint-1:4317"},
	})

	metadatatest.AssertEqualLoadbalancerBackendState(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("state", "eligible")),
			Value:      1,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("state", "quarantined")),
			Value:      0,
		},
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("state", "stale")),
			Value:      0,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestLoadBalancerEndpointHealthAlreadyQuarantinedFailureRefreshesStaleRing(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)

	endpoint2CreateStarted := make(chan struct{})
	releaseEndpoint2Create := make(chan struct{})
	var blockedEndpoint2Create atomic.Bool
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		if endpoint == "endpoint-2:4317" && blockedEndpoint2Create.CompareAndSwap(false, true) {
			close(endpoint2CreateStarted)
			select {
			case <-releaseEndpoint2Create:
			case <-time.After(5 * time.Second):
				return nil, errors.New("timed out waiting to release endpoint-2 creation")
			}
		}
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	p.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	p.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	p.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": newWrappedExporter(mockComponent{}, "endpoint-1:4317"),
	}
	routingID := findRoutingIDForEndpoint(t, p.ring, "endpoint-1:4317")

	firstFailureDone := make(chan endpointHealthFailureDecision, 1)
	go func() {
		firstFailureDone <- p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	}()

	require.Eventually(t, func() bool {
		select {
		case <-endpoint2CreateStarted:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	secondDecision := p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, secondDecision.endpointLocal)
	require.False(t, secondDecision.quarantined)
	require.Equal(t, []string{"endpoint-2:4317"}, secondDecision.eligible)

	_, endpoint, err := p.exporterAndEndpoint([]byte(routingID))
	require.NoError(t, err)
	require.Equal(t, "endpoint-2:4317", endpoint)
	require.Contains(t, p.exporters, "endpoint-2:4317")
	require.NotContains(t, p.exporters, "endpoint-1:4317")

	close(releaseEndpoint2Create)
	firstDecision := <-firstFailureDone
	require.True(t, firstDecision.quarantined)
}

func TestLoadBalancerEndpointHealthCreatesExportersOutsideUpdateLock(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)

	var p *loadBalancer
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		if p != nil {
			acquired := make(chan struct{})
			go func() {
				p.updateLock.RLock()
				defer p.updateLock.RUnlock()
				close(acquired)
			}()

			select {
			case <-acquired:
			case <-time.After(100 * time.Millisecond):
				return nil, errors.New("componentFactory called while updateLock is held")
			}
		}
		return mockComponent{}, nil
	}

	var err error
	p, err = newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	decision := p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	decision = p.handleBackendFailure(t.Context(), "endpoint-2:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.failOpen)

	p.updateLock.Lock()
	delete(p.exporters, "endpoint-1:4317")
	p.updateLock.Unlock()

	p.handleBackendSuccess("endpoint-1:4317")
	require.Contains(t, p.exporters, "endpoint-1:4317")
}

func TestLoadBalancerEndpointHealthResolverRecomputesEligibilityBeforeRingInstall(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)

	var shutdowns sync.Map
	var starts sync.Map
	var blockedEndpoint1 atomic.Bool
	endpoint1CreateBlocked := make(chan struct{})
	releaseEndpoint1Create := make(chan struct{})
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		if endpoint == "endpoint-1:4317" && blockedEndpoint1.CompareAndSwap(false, true) {
			close(endpoint1CreateBlocked)
			select {
			case <-releaseEndpoint1Create:
			case <-time.After(5 * time.Second):
				return nil, errors.New("timed out waiting to release endpoint-1 creation")
			}
		}
		return &countingComponent{endpoint: endpoint, starts: &starts, shutdowns: &shutdowns}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	resolverDone := make(chan struct{})
	go func() {
		p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
		close(resolverDone)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-endpoint1CreateBlocked:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	decision := p.handleBackendFailure(t.Context(), "endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	require.Equal(t, []string{"endpoint-2:4317"}, decision.eligible)

	close(releaseEndpoint1Create)
	require.Eventually(t, func() bool {
		select {
		case <-resolverDone:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	p.updateLock.RLock()
	for _, item := range p.ring.items {
		require.NotEqual(t, "endpoint-1:4317", item.endpoint)
	}
	p.updateLock.RUnlock()

	require.NotContains(t, p.exporters, "endpoint-1:4317")
	require.Contains(t, p.exporters, "endpoint-2:4317")
	_, endpoint1Started := starts.Load("endpoint-1:4317")
	require.False(t, endpoint1Started)
	require.Eventually(t, func() bool {
		count, ok := shutdowns.Load("endpoint-1:4317")
		return ok && count.(*atomic.Int64).Load() > 0
	}, time.Second, 10*time.Millisecond)
	endpoint2Starts, endpoint2Started := starts.Load("endpoint-2:4317")
	require.True(t, endpoint2Started)
	require.Equal(t, int64(1), endpoint2Starts.(*atomic.Int64).Load())

	for i := range 100 {
		_, endpoint, err := p.exporterAndEndpoint([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)})
		require.NoError(t, err)
		assert.Equal(t, "endpoint-2:4317", endpoint)
	}
}

func TestLoadBalancerEndpointHealthResolverCommitUsesLockedEligibility(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	var shutdowns sync.Map
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return &countingComponent{endpoint: endpoint, shutdowns: &shutdowns}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	resolved := normalizeEndpoints([]string{"endpoint-1", "endpoint-2"})
	reconcile := p.endpointHealth.reconcile(resolved)
	require.Equal(t, []string{"endpoint-1:4317", "endpoint-2:4317"}, reconcile.eligible)
	staleEligible := append([]string(nil), reconcile.eligible...)
	require.Contains(t, staleEligible, "endpoint-1:4317")

	staleEndpoint1 := newWrappedExporter(&countingComponent{endpoint: "endpoint-1:4317", shutdowns: &shutdowns}, "endpoint-1:4317")
	endpoint2 := newWrappedExporter(&countingComponent{endpoint: "endpoint-2:4317", shutdowns: &shutdowns}, "endpoint-2:4317")
	created := []createdExporter{
		{endpoint: "endpoint-1:4317", exporter: staleEndpoint1},
		{endpoint: "endpoint-2:4317", exporter: endpoint2},
	}

	decision := p.endpointHealth.markFailure("endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	require.Equal(t, []string{"endpoint-2:4317"}, decision.eligible)

	p.updateLock.Lock()
	duplicates, removed := p.commitEndpointHealthResolverUpdateLocked(resolved, created)
	p.updateLock.Unlock()
	p.shutdownCreatedExporters(t.Context(), duplicates)
	p.drainRemovedExporters(t.Context(), removed)

	require.Contains(t, p.exporters, "endpoint-2:4317")
	require.Same(t, endpoint2, p.exporters["endpoint-2:4317"])
	require.NotContains(t, p.exporters, "endpoint-1:4317")
	require.Eventually(t, func() bool {
		count, ok := shutdowns.Load("endpoint-1:4317")
		return ok && count.(*atomic.Int64).Load() > 0
	}, time.Second, 10*time.Millisecond)

	for i := range 100 {
		_, endpoint, err := p.exporterAndEndpoint([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)})
		require.NoError(t, err)
		assert.Equal(t, "endpoint-2:4317", endpoint)
	}
}

func TestLoadBalancerEndpointHealthResolverCommitDoesNotReadmitExpiredEndpointWithoutExporter(t *testing.T) {
	now := time.Unix(100, 0)
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()
	enableEndpointHealth(cfg)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return mockComponent{}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	p.endpointHealth.settings.now = func() time.Time { return now }

	resolved := normalizeEndpoints([]string{"endpoint-1", "endpoint-2"})
	p.endpointHealth.reconcile(resolved)
	p.exporters["endpoint-2:4317"] = newWrappedExporter(mockComponent{}, "endpoint-2:4317")

	decision := p.endpointHealth.markFailure("endpoint-1:4317", status.Error(codes.Unavailable, "unavailable"))
	require.True(t, decision.quarantined)
	require.Equal(t, []string{"endpoint-2:4317"}, decision.eligible)

	now = now.Add(time.Minute)
	p.updateLock.Lock()
	duplicates, removed := p.commitEndpointHealthResolverUpdateLocked(resolved, nil)
	p.updateLock.Unlock()
	p.shutdownCreatedExporters(t.Context(), duplicates)
	p.drainRemovedExporters(t.Context(), removed)

	require.Contains(t, p.exporters, "endpoint-2:4317")
	require.NotContains(t, p.exporters, "endpoint-1:4317")
	p.updateLock.RLock()
	for _, item := range p.ring.items {
		require.NotEqual(t, "endpoint-1:4317", item.endpoint)
	}
	p.updateLock.RUnlock()

	for i := range 100 {
		_, _, err := p.exporterAndEndpoint([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)})
		require.NoError(t, err)
	}
}

func TestEndpointWithPort(t *testing.T) {
	for _, tt := range []struct {
		input, expected string
	}{
		{
			"endpoint-1",
			"endpoint-1:4317",
		},
		{
			"endpoint-1:55690",
			"endpoint-1:55690",
		},
	} {
		assert.Equal(t, tt.expected, endpointWithPort(tt.input))
	}
}

func TestFailedExporterInRing(t *testing.T) {
	// this test is based on the discussion in the original PR for this exporter:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/1542#discussion_r521268180
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	// simulate the case where one of the exporters failed to be created and do not exist in the internal map
	// this is a case that we are not even sure that might happen, so, this test case is here to document
	// this behavior. As the solution would require more locks/syncs/checks, we should probably wait to see
	// if this is really a problem in the real world
	resEndpoint := "endpoint-2"
	delete(p.exporters, endpointWithPort(resEndpoint))

	// sanity check
	require.Contains(t, p.res.(*staticResolver).endpoints, resEndpoint)

	// test
	// this trace ID will reach the endpoint-2 -- see the consistent hashing tests for more info
	_, _, err = p.exporterAndEndpoint([]byte{128, 128, 1, 0})

	// verify
	assert.Error(t, err)

	// test
	// this service name will reach the endpoint-2 -- see the consistent hashing tests for more info
	_, _, err = p.exporterAndEndpoint([]byte("get-recommendations-2"))

	// verify
	assert.Error(t, err)
}

func TestNewLoadBalancerInvalidNamespaceAwsResolver(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			AWSCloudMap: configoptional.Some(AWSCloudMapResolver{
				NamespaceName: "",
			}),
		},
	}

	// test
	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)

	// verify
	assert.Nil(t, p)
	assert.True(t, clientcmd.IsConfigurationInvalid(err) || errors.Is(err, errNoNamespace))
}

func TestNewLoadBalancerInvalidServiceAwsResolver(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			AWSCloudMap: configoptional.Some(AWSCloudMapResolver{
				NamespaceName: "cloudmap",
				ServiceName:   "",
			}),
		},
	}

	// test
	p, err := newLoadBalancer(ts.Logger, cfg, nil, tb)

	// verify
	assert.Nil(t, p)
	assert.True(t, clientcmd.IsConfigurationInvalid(err) || errors.Is(err, errNoServiceName))
}

func newNopMockExporter() *wrappedExporter {
	return newWrappedExporter(mockComponent{}, "mock")
}

func enableEndpointHealth(cfg *Config) {
	cfg.EndpointHealth.Enabled = true
	cfg.EndpointHealth.QuarantineDuration = time.Minute
	cfg.EndpointHealth.RerouteOnFailure = true
	cfg.EndpointHealth.MaxRerouteAttempts = 1
}

func listenTCPForProbe(t *testing.T, address string) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", address)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = listener.Close()
	})
	return listener
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	listener := listenTCPForProbe(t, "127.0.0.1:0")
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func requireProbeStarts(t *testing.T, started <-chan struct{}, count int) {
	t.Helper()

	for range count {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("expected %d active probes to start", count)
		}
	}
}

type countingComponent struct {
	endpoint  string
	starts    *sync.Map
	shutdowns *sync.Map
}

func (c *countingComponent) Start(context.Context, component.Host) error {
	if c.starts != nil {
		count, _ := c.starts.LoadOrStore(c.endpoint, &atomic.Int64{})
		count.(*atomic.Int64).Add(1)
	}
	return nil
}

func (c *countingComponent) Shutdown(context.Context) error {
	count, _ := c.shutdowns.LoadOrStore(c.endpoint, &atomic.Int64{})
	count.(*atomic.Int64).Add(1)
	return nil
}

type blockingShutdownComponent struct {
	started     chan struct{}
	release     chan struct{}
	startedOnce sync.Once
}

func (*blockingShutdownComponent) Start(context.Context, component.Host) error {
	return nil
}

func (c *blockingShutdownComponent) Shutdown(ctx context.Context) error {
	c.startedOnce.Do(func() { close(c.started) })
	select {
	case <-c.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
