// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"net"
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

// hangingShutdownComponent blocks in Shutdown until its context is cancelled.
// Used to test that per-goroutine shutdown timeouts are enforced.
type hangingShutdownComponent struct {
	component.StartFunc
}

func (h *hangingShutdownComponent) Shutdown(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// TestOnBackendChanges_HungFactory_ReturnsAfterTimeout verifies that a factory
// that blocks during addMissingExporters does not hold the write lock indefinitely.
// The loadBalancer must impose a per-call timeout so that onBackendChanges returns
// and the ring update is not stalled.
func TestOnBackendChanges_HungFactory_ReturnsAfterTimeout(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	blocked := make(chan struct{})
	defer close(blocked)
	componentFactory := func(ctx context.Context, _ string) (component.Component, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-blocked:
			return newNopMockExporter(), nil
		}
	}

	p, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, p)
	require.NoError(t, err)
	p.exporterAddTimeout = 50 * time.Millisecond

	done := make(chan struct{})
	go func() {
		p.onBackendChanges([]string{"endpoint-1"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onBackendChanges did not return: hung factory is holding the write lock indefinitely")
	}

	p.updateLock.RLock()
	count := len(p.exporters)
	p.updateLock.RUnlock()
	assert.Zero(t, count, "no exporter should be added when the factory times out")
}

// TestRemoveExtraExporters_HungShutdown_GoRoutineTimesOut verifies that the
// goroutine spawned for an async exporter shutdown completes on its own timeout
// even when the exporter's Shutdown only returns after context cancellation.
func TestRemoveExtraExporters_HungShutdown_GoRoutineTimesOut(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	p, err := newLoadBalancer(ts.Logger, simpleConfig(), nil, tb)
	require.NotNil(t, p)
	require.NoError(t, err)
	p.exporterShutdownTimeout = 50 * time.Millisecond

	p.exporters["hanging:4317"] = newWrappedExporter(&hangingShutdownComponent{}, "hanging:4317")
	p.removeExtraExporters(t.Context(), []string{})

	done := make(chan struct{})
	go func() {
		p.exportersShutdownWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async shutdown goroutine did not return: per-goroutine shutdown timeout not enforced")
	}
}
