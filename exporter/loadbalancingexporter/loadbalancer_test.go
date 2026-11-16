// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"maps"
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
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
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

	dnsRes, ok := p.res.(*dnsResolver)
	require.True(t, ok)
	dnsRes.resolver = &mockDNSResolver{
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

func TestOnBackendChanges_SlowStartDoesNotBlockDataPath(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()

	reached := make(chan struct{})
	release := make(chan struct{})
	closeRelease := sync.OnceFunc(func() { close(release) })
	t.Cleanup(closeRelease) // Register in case of test failure to avoid a leak.

	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		if endpoint == endpointWithPort("endpoint-2") {
			close(reached)
			<-release
		}

		return newNopMockExporter(), nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1"})
	require.Len(t, p.exporters, 1)

	// find a key that moves to endpoint-2 once the new ring is installed
	var key []byte
	newRing := newHashRing([]string{"endpoint-1", "endpoint-2"})
	for i := range 256 {
		if candidate := []byte{byte(i), 0, 0, 0}; newRing.endpointFor(candidate) == "endpoint-2" {
			key = candidate
			break
		}
	}
	require.NotNil(t, key)

	// start a transition that blocks inside the component factory for endpoint-2
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	}()
	<-reached

	// test
	type lookup struct {
		endpoint string
		err      error
	}
	result := make(chan lookup, 1)
	go func() {
		_, endpoint, lookupErr := p.exporterAndEndpoint(key)
		result <- lookup{endpoint: endpoint, err: lookupErr}
	}()

	// verify the data path is still served by the existing ring
	select {
	case res := <-result:
		require.NoError(t, res.err)
		assert.Equal(t, "endpoint-1", res.endpoint)
	case <-time.After(5 * time.Second):
		require.Fail(t, "exporterAndEndpoint blocked while the new exporter was being started")
	}

	closeRelease()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "the backend transition didn't complete")
	}

	// verify
	assert.Len(t, p.exporters, 2)
	assert.Len(t, p.ring.items, 2*defaultWeight)
	_, endpoint, err := p.exporterAndEndpoint(key)
	require.NoError(t, err)
	assert.Equal(t, "endpoint-2", endpoint)
}

func TestOnBackendChanges_FailedEndpointIsLeftOutOfRing(t *testing.T) {
	// prepare
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()

	failing := true
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		_, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline, "exporter creation should be bounded by a timeout")

		if failing && endpoint == endpointWithPort("endpoint-2") {
			return nil, errors.New("failed to create exporter")
		}
		return newNopMockExporter(), nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	resolved := []string{"endpoint-1", "endpoint-2"}

	// test
	p.onBackendChanges(resolved)

	// verify
	assert.Len(t, p.exporters, 1)
	assert.Len(t, p.ring.items, defaultWeight)
	for i := range 256 {
		_, endpoint, lookupErr := p.exporterAndEndpoint([]byte{byte(i), 0, 0, 0})
		require.NoError(t, lookupErr)
		assert.Equal(t, "endpoint-1", endpoint)
	}

	// resolvers only report changes, so the failed endpoint is retried on the next topology change
	failing = false
	p.onBackendChanges([]string{"endpoint-1", "endpoint-2", "endpoint-3"})
	assert.Len(t, p.exporters, 3)
	assert.Len(t, p.ring.items, 3*defaultWeight)
}

// addMissingExporters synchronously creates and installs an exporter for every missing endpoint.
func (lb *loadBalancer) addMissingExporters(_ context.Context, endpoints []string) {
	maps.Copy(lb.exporters, lb.startMissingExporters(endpoints, lb.exporters))
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
	p.removeExtraExporters(t.Context(), resolved)

	// verify
	assert.Len(t, p.exporters, 1)
	assert.NotContains(t, p.exporters, endpointWithPort("endpoint-2"))
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

func TestShutdown_DuringBackendChanges_NoMapPanicOrLeak(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()

	reached := make(chan struct{})
	release := make(chan struct{})
	closeRelease := sync.OnceFunc(func() { close(release) })
	t.Cleanup(closeRelease)

	var ep1Shutdown, ep2Shutdown atomic.Bool

	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		if endpoint == endpointWithPort("endpoint-2") {
			close(reached)
			<-release
			return mockComponent{
				ShutdownFunc: func(context.Context) error {
					ep2Shutdown.Store(true)
					return nil
				},
			}, nil
		}

		return mockComponent{
			ShutdownFunc: func(context.Context) error {
				ep1Shutdown.Store(true)
				return nil
			},
		}, nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	// Initial backend: endpoint-1
	p.onBackendChanges([]string{"endpoint-1"})
	require.Len(t, p.exporters, 1)

	// Trigger onBackendChanges to add endpoint-2 in background
	backendChangeDone := make(chan struct{})
	go func() {
		defer close(backendChangeDone)
		p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
	}()

	// Wait until endpoint-2 is in startMissingExporters
	<-reached

	// While endpoint-2 is starting, invoke Shutdown in parallel
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		assert.NoError(t, p.Shutdown(t.Context()))
	}()

	// Release endpoint-2 so startMissingExporters can complete
	closeRelease()

	// Wait for both onBackendChanges and Shutdown to finish
	select {
	case <-backendChangeDone:
	case <-time.After(5 * time.Second):
		require.Fail(t, "onBackendChanges timed out")
	}

	select {
	case <-shutdownDone:
	case <-time.After(5 * time.Second):
		require.Fail(t, "Shutdown timed out")
	}

	// Verify no resource leak: both endpoint-1 and endpoint-2 had Shutdown called
	assert.True(t, ep1Shutdown.Load(), "endpoint-1 should have been shut down")
	assert.True(t, ep2Shutdown.Load(), "endpoint-2 should have been shut down when stopped")

	// Verify loadbalancer state
	assert.True(t, p.stopped)
	assert.Empty(t, p.exporters)
	assert.Equal(t, 0, p.NumBackends())

	// Verify routing after shutdown returns error
	_, _, err = p.exporterAndEndpoint([]byte("any"))
	assert.Error(t, err)

	// Subsequent onBackendChanges after shutdown should be a no-op
	p.onBackendChanges([]string{"endpoint-3"})
	assert.Empty(t, p.exporters)
}

func TestShutdown_ConcurrentWithBackendChanges_Race(t *testing.T) {
	for range 10 {
		ts, tb := getTelemetryAssets(t)
		cfg := simpleConfig()

		componentFactory := func(_ context.Context, _ string) (component.Component, error) {
			return newNopMockExporter(), nil
		}

		p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
		require.NotNil(t, p)
		require.NoError(t, err)

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for range 20 {
				p.onBackendChanges([]string{"endpoint-1", "endpoint-2"})
				p.onBackendChanges([]string{"endpoint-1"})
			}
		}()

		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond)
			_ = p.Shutdown(t.Context())
		}()

		wg.Wait()
		assert.True(t, p.stopped)
	}
}

func TestShutdown_Idempotent(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := simpleConfig()

	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}

	p, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.onBackendChanges([]string{"endpoint-1"})
	require.Len(t, p.exporters, 1)

	require.NoError(t, p.Shutdown(t.Context()))
	assert.True(t, p.stopped)
	assert.Empty(t, p.exporters)

	// Calling Shutdown again should succeed and be a no-op
	require.NoError(t, p.Shutdown(t.Context()))
}
