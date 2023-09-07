// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"k8s.io/client-go/tools/clientcmd"
)

func TestNewLoadBalancerNoResolver(t *testing.T) {
	// prepare
	cfg := &Config{}

	// test
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, nil)

	// verify
	require.Nil(t, p)
	require.Equal(t, errNoResolver, err)
}

func TestNewLoadBalancerInvalidStaticResolver(t *testing.T) {
	// prepare
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{}},
		},
	}

	// test
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, nil)

	// verify
	require.Nil(t, p)
	require.Equal(t, errNoEndpoints, err)
}

func TestNewLoadBalancerInvalidDNSResolver(t *testing.T) {
	// prepare
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: &DNSResolver{
				Hostname: "",
			},
		},
	}

	// test
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, nil)

	// verify
	require.Nil(t, p)
	require.Equal(t, errNoHostname, err)
}

func TestNewLoadBalancerInvalidK8sResolver(t *testing.T) {
	// prepare
	cfg := &Config{
		Resolver: ResolverSettings{
			K8sSvc: &K8sSvcResolver{
				Service: "",
			},
		},
	}

	// test
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, nil)

	// verify
	assert.Nil(t, p)
	assert.True(t, clientcmd.IsConfigurationInvalid(err) || errors.Is(err, errNoSvc))
}

func TestLoadBalancerStart(t *testing.T) {
	// prepare
	cfg := simpleConfig()
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, nil)
	require.NotNil(t, p)
	require.NoError(t, err)
	p.res = &mockResolver{}

	// test
	res := p.Start(context.Background(), componenttest.NewNopHost())
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()
	// verify
	assert.Nil(t, res)
}

func TestWithDNSResolver(t *testing.T) {
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: &DNSResolver{
				Hostname: "service-1",
			},
		},
	}
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, nil)
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
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: &DNSResolver{
				Hostname: "service-1",
			},
		},
	}
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, nil)
	require.NotNil(t, p)
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	e := p.Endpoint([]byte{128, 128, 0, 0})

	// verify
	assert.Equal(t, "", e)
}

func TestMultipleResolvers(t *testing.T) {
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{
				Hostnames: []string{"endpoint-1", "endpoint-2"},
			},
			DNS: &DNSResolver{
				Hostname: "service-1",
			},
		},
	}

	// test
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, nil)

	// verify
	assert.Nil(t, p)
	assert.Equal(t, errMultipleResolversProvided, err)
}

func TestStartFailureStaticResolver(t *testing.T) {
	// prepare
	cfg := simpleConfig()
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, nil)
	require.NotNil(t, p)
	require.NoError(t, err)

	expectedErr := errors.New("some expected err")
	p.res = &mockResolver{
		onStart: func(context.Context) error {
			return expectedErr
		},
	}

	// test
	res := p.Start(context.Background(), componenttest.NewNopHost())

	// verify
	assert.Equal(t, expectedErr, res)
}

func TestLoadBalancerShutdown(t *testing.T) {
	// prepare
	cfg := simpleConfig()
	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(context.Background())

	// verify
	assert.Nil(t, res)
}

func TestOnBackendChanges(t *testing.T) {
	// prepare
	cfg := simpleConfig()
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, componentFactory)
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
	cfg := simpleConfig()
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, componentFactory)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.addMissingExporters(context.Background(), []string{"endpoint-1", "endpoint-2"})
	resolved := []string{"endpoint-1"}

	// test
	p.removeExtraExporters(context.Background(), resolved)

	// verify
	assert.Len(t, p.exporters, 1)
	assert.NotContains(t, p.exporters, endpointWithPort("endpoint-2"))
}

func TestAddMissingExporters(t *testing.T) {
	// prepare
	cfg := simpleConfig()
	exporterFactory := exporter.NewFactory("otlp", func() component.Config {
		return &otlpexporter.Config{}
	}, exporter.WithTraces(func(
		_ context.Context,
		_ exporter.CreateSettings,
		_ component.Config,
	) (exporter.Traces, error) {
		return newNopMockTracesExporter(), nil
	}, component.StabilityLevelDevelopment))
	fn := func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := cfg.Protocol.OTLP
		oCfg.Endpoint = endpoint
		return exporterFactory.CreateTracesExporter(ctx, exportertest.NewNopCreateSettings(), &oCfg)
	}

	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, fn)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.exporters["endpoint-1:4317"] = newNopMockExporter()
	resolved := []string{"endpoint-1", "endpoint-2"}

	// test
	p.addMissingExporters(context.Background(), resolved)

	// verify
	assert.Len(t, p.exporters, 2)
	assert.Contains(t, p.exporters, "endpoint-2:4317")
}

func TestFailedToAddMissingExporters(t *testing.T) {
	// prepare
	cfg := simpleConfig()
	expectedErr := errors.New("some expected error")
	exporterFactory := exporter.NewFactory("otlp", func() component.Config {
		return &otlpexporter.Config{}
	}, exporter.WithTraces(func(
		_ context.Context,
		_ exporter.CreateSettings,
		_ component.Config,
	) (exporter.Traces, error) {
		return nil, expectedErr
	}, component.StabilityLevelDevelopment))
	fn := func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := cfg.Protocol.OTLP
		oCfg.Endpoint = endpoint
		return exporterFactory.CreateTracesExporter(ctx, exportertest.NewNopCreateSettings(), &oCfg)
	}

	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, fn)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.exporters["endpoint-1:4317"] = newNopMockExporter()
	resolved := []string{"endpoint-1", "endpoint-2"}

	// test
	p.addMissingExporters(context.Background(), resolved)

	// verify
	assert.Len(t, p.exporters, 1)
	assert.Contains(t, p.exporters, "endpoint-1:4317")
}

func TestEndpointFound(t *testing.T) {
	for _, tt := range []struct {
		endpoint  string
		endpoints []string
		expected  bool
	}{
		{
			"endpoint-1",
			[]string{"endpoint-1", "endpoint-2"},
			true,
		},
		{
			"endpoint-3",
			[]string{"endpoint-1", "endpoint-2"},
			false,
		},
	} {
		assert.Equal(t, tt.expected, endpointFound(tt.endpoint, tt.endpoints))
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
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}},
		},
	}
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	p, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, componentFactory)
	require.NotNil(t, p)
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
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
	_, err = p.Exporter(p.Endpoint([]byte{128, 128, 0, 0}))

	// verify
	assert.Error(t, err)

	// test
	// this service name will reach the endpoint-2 -- see the consistent hashing tests for more info
	_, err = p.Exporter(p.Endpoint([]byte("get-recommendations-1")))

	// verify
	assert.Error(t, err)
}

func newNopMockExporter() component.Component {
	return mockComponent{}
}
