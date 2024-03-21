// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.9.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

const (
	serviceRouteKey  = "service"
	resourceRouteKey = "resource"
	metricRouteKey   = "metric"

	ilsName1          = "library-1"
	ilsName2          = "library-2"
	keyAttr1          = "resattr-1"
	keyAttr2          = "resattr-2"
	valueAttr1        = "resvaluek1"
	valueAttr2        = 10
	signal1Name       = "sig-1"
	signal2Name       = "sig-2"
	signal1Attr1Key   = "sigattr1k"
	signal1Attr1Value = "sigattr1v"
	signal1Attr2Key   = "sigattr2k"
	signal1Attr2Value = 20
	signal1Attr3Key   = "sigattr3k"
	signal1Attr3Value = true
	signal1Attr4Key   = "sigattr4k"
	signal1Attr4Value = 3.3
	serviceName1      = "service-name-1"
	serviceName2      = "service-name-2"
)

func TestNewMetricsExporter(t *testing.T) {
	for _, tt := range []struct {
		desc   string
		config *Config
		err    error
	}{
		{
			"empty routing key",
			&Config{},
			errNoResolver,
		},
		{
			"service",
			serviceBasedRoutingConfig(),
			nil,
		},
		{
			"metric",
			metricNameBasedRoutingConfig(),
			nil,
		},
		{
			"resource",
			resourceBasedRoutingConfig(),
			nil,
		},
		{
			"traceID",
			&Config{
				RoutingKey: "service",
			},
			errNoResolver,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			// test
			_, err := newMetricsExporter(exportertest.NewNopCreateSettings(), tt.config)

			// verify
			require.Equal(t, tt.err, err)
		})
	}
}

func TestMetricsExporterStart(t *testing.T) {
	for _, tt := range []struct {
		desc string
		te   *metricExporterImp
		err  error
	}{
		{
			"ok",
			func() *metricExporterImp {
				p, _ := newMetricsExporter(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig())
				return p
			}(),
			nil,
		},
		{
			"error",
			func() *metricExporterImp {
				lb, _ := newLoadBalancer(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig(), nil)
				p, _ := newMetricsExporter(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig())

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
			p := tt.te

			// test
			res := p.Start(context.Background(), componenttest.NewNopHost())
			defer func() {
				require.NoError(t, p.Shutdown(context.Background()))
			}()

			// verify
			require.Equal(t, tt.err, res)
		})
	}
}

func TestMetricsExporterShutdown(t *testing.T) {
	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(context.Background())

	// verify
	assert.Nil(t, res)
}

func TestConsumeMetrics(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)
	assert.Equal(t, p.routingKey, svcRouting)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1", "endpoint-2"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1", "endpoint-2"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeMetrics(context.Background(), simpleMetricsWithNoService())

	// verify
	assert.Error(t, res)

}

// this test validates that exporter is can concurrently change the endpoints while consuming metrics.
func TestConsumeMetrics_ConcurrentResolverChange(t *testing.T) {
	consumeStarted := make(chan struct{})
	consumeDone := make(chan struct{})

	// imitate a slow exporter
	te := &mockMetricsExporter{Component: mockComponent{}}
	te.ConsumeMetricsFn = func(ctx context.Context, td pmetric.Metrics) error {
		close(consumeStarted)
		time.Sleep(50 * time.Millisecond)
		return te.consumeErr
	}
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return te, nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	endpoints := []string{"endpoint-1"}
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	go func() {
		assert.NoError(t, p.ConsumeMetrics(context.Background(), simpleMetricsWithResource()))
		close(consumeDone)
	}()

	// update endpoint while consuming logs
	<-consumeStarted
	endpoints = []string{"endpoint-2"}
	endpoint, err := lb.res.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, endpoints, endpoint)
	<-consumeDone
}

func TestConsumeMetricsServiceBased(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)
	assert.Equal(t, p.routingKey, svcRouting)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeMetrics(context.Background(), simpleMetricsWithServiceName())

	// verify
	assert.Nil(t, res)
}

func TestConsumeMetricsResourceBased(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), resourceBasedRoutingConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), resourceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)
	assert.Equal(t, p.routingKey, resourceRouting)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeMetrics(context.Background(), simpleMetricsWithResource())

	// verify
	assert.Nil(t, res)
}

func TestConsumeMetricsMetricNameBased(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), metricNameBasedRoutingConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), metricNameBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)
	assert.Equal(t, p.routingKey, metricNameRouting)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeMetrics(context.Background(), simpleMetricsWithResource())

	// verify
	assert.Nil(t, res)
}

func TestServiceBasedRoutingForSameMetricName(t *testing.T) {

	for _, tt := range []struct {
		desc       string
		batch      pmetric.Metrics
		routingKey routingKey
		res        map[string]bool
	}{
		{
			"different services - service based routing",
			twoServicesWithSameMetricName(),
			svcRouting,
			map[string]bool{serviceName1: true, serviceName2: true},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res, err := routingIdentifiersFromMetrics(tt.batch, tt.routingKey)
			assert.Equal(t, err, nil)
			assert.Equal(t, res, tt.res)
		})
	}
}

func TestConsumeMetricsExporterNoEndpoint(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), endpoint2Config())
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return nil, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeMetrics(context.Background(), simpleMetricsWithServiceName())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("couldn't find the exporter for the endpoint %q", ""))
}

func TestConsumeMetricsUnexpectedExporterType(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	lb.addMissingExporters(context.Background(), []string{"endpoint-2"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1", "endpoint-2"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeMetrics(context.Background(), simpleMetricsWithServiceName())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("unable to export metrics, unexpected exporter type: expected exporter.Metrics but got %T", newNopMockExporter()))
}

func TestBuildExporterConfigUnknown(t *testing.T) {
	// prepare
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factories.Exporters[metadata.Type] = NewFactory()

	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "test-build-exporter-config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	c := cfg.Exporters[component.NewID(metadata.Type)]
	require.NotNil(t, c)

	// test
	defaultCfg := otlpexporter.NewFactory().CreateDefaultConfig().(*otlpexporter.Config)
	exporterCfg := buildExporterConfig(c.(*Config), "the-endpoint")

	// verify
	grpcSettings := defaultCfg.ClientConfig
	grpcSettings.Endpoint = "the-endpoint"
	assert.Equal(t, grpcSettings, exporterCfg.ClientConfig)

	assert.Equal(t, defaultCfg.TimeoutSettings, exporterCfg.TimeoutSettings)
	assert.Equal(t, defaultCfg.QueueConfig, exporterCfg.QueueConfig)
	assert.Equal(t, defaultCfg.RetryConfig, exporterCfg.RetryConfig)
}

func TestBatchWithTwoMetrics(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(sink.ConsumeMetrics), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})

	td := twoServicesWithSameMetricName()

	// test
	err = p.ConsumeMetrics(context.Background(), td)

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.AllMetrics(), 2)
}

func TestNoMetricsInBatch(t *testing.T) {
	for _, tt := range []struct {
		desc       string
		batch      pmetric.Metrics
		routingKey routingKey
		err        error
	}{
		{
			"no resource metrics",
			pmetric.NewMetrics(),
			svcRouting,
			errors.New("empty resource metrics"),
		},
		{
			"no instrumentation library metrics",
			func() pmetric.Metrics {
				batch := pmetric.NewMetrics()
				batch.ResourceMetrics().AppendEmpty()
				return batch
			}(),
			svcRouting,
			errors.New("empty scope metrics"),
		},
		{
			"no metrics",
			func() pmetric.Metrics {
				batch := pmetric.NewMetrics()
				batch.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				return batch
			}(),
			svcRouting,
			errors.New("empty metrics"),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res, err := routingIdentifiersFromMetrics(tt.batch, tt.routingKey)
			assert.Equal(t, err, tt.err)
			assert.Equal(t, res, map[string]bool(nil))
		})
	}
}

func TestResourceRoutingKey(t *testing.T) {

	md := pmetric.NewMetric()
	md.SetName("metric")
	attrs := pcommon.NewMap()
	if got := resourceRoutingKey(md, attrs); got != "metric" {
		t.Errorf("metricRoutingKey() = %v, want %v", got, "metric")
	}

	attrs.PutStr("k1", "v1")
	if got := resourceRoutingKey(md, attrs); got != "k1v1metric" {
		t.Errorf("metricRoutingKey() = %v, want %v", got, "k1v1metric")
	}

	attrs.PutStr("k2", "v2")
	if got := resourceRoutingKey(md, attrs); got != "k1v1k2v2metric" {
		t.Errorf("metricRoutingKey() = %v, want %v", got, "k1v1k2v2metric")
	}
}

func TestMetricNameRoutingKey(t *testing.T) {

	md := pmetric.NewMetric()
	md.SetName(signal1Name)
	if got := metricRoutingKey(md); got != signal1Name {
		t.Errorf("metricRoutingKey() = %v, want %v", got, signal1Name)
	}

	md = pmetric.NewMetric()
	md.SetName(signal2Name)
	if got := metricRoutingKey(md); got != signal2Name {
		t.Errorf("metricRoutingKey() = %v, want %v", got, signal2Name)
	}

}

func TestRollingUpdatesWhenConsumeMetrics(t *testing.T) {
	t.Skip("Flaky Test - See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/13331")

	// this test is based on the discussion in the following issue for this exporter:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	// prepare

	// simulate rolling updates, the dns resolver should resolve in the following order
	// ["127.0.0.1"] -> ["127.0.0.1", "127.0.0.2"] -> ["127.0.0.2"]
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", 5*time.Second, 1*time.Second)
	require.NoError(t, err)

	mu := sync.Mutex{}
	var lastResolved []string
	res.onChange(func(s []string) {
		mu.Lock()
		lastResolved = s
		mu.Unlock()
	})

	resolverCh := make(chan struct{}, 1)
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
			defer func() {
				counter.Add(1)
			}()

			if counter.Load() <= 2 {
				return resolve[counter.Load()], nil
			}

			if counter.Load() == 3 {
				// stop as soon as rolling updates end
				resolverCh <- struct{}{}
			}

			return resolve[2], nil
		},
	}
	res.resInterval = 10 * time.Millisecond

	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: &DNSResolver{Hostname: "service-1", Port: ""},
		},
	}
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = res
	p.loadBalancer = lb

	counter1 := &atomic.Int64{}
	counter2 := &atomic.Int64{}
	defaultExporters := map[string]*wrappedExporter{
		"127.0.0.1:4317": newWrappedExporter(newMockMetricsExporter(func(ctx context.Context, td pmetric.Metrics) error {
			counter1.Add(1)
			// simulate an unreachable backend
			time.Sleep(10 * time.Second)
			return nil
		})),
		"127.0.0.2:4317": newWrappedExporter(newMockMetricsExporter(func(ctx context.Context, td pmetric.Metrics) error {
			counter2.Add(1)
			return nil
		})),
	}

	// test
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()
	// ensure using default exporters
	lb.updateLock.Lock()
	lb.exporters = defaultExporters
	lb.updateLock.Unlock()
	lb.res.onChange(func(endpoints []string) {
		lb.updateLock.Lock()
		lb.exporters = defaultExporters
		lb.updateLock.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	// keep consuming metrics every 2ms
	consumeCh := make(chan struct{})
	go func(ctx context.Context) {
		ticker := time.NewTicker(2 * time.Millisecond)
		for {
			select {
			case <-ctx.Done():
				consumeCh <- struct{}{}
				return
			case <-ticker.C:
				go func() {
					require.NoError(t, p.ConsumeMetrics(ctx, randomMetrics()))
				}()
			}
		}
	}(ctx)

	// give limited but enough time to rolling updates. otherwise this test
	// will still pass due to the 10 secs of sleep that is used to simulate
	// unreachable backends.
	go func() {
		time.Sleep(1 * time.Second)
		resolverCh <- struct{}{}
	}()

	<-resolverCh
	cancel()
	<-consumeCh

	// verify
	mu.Lock()
	require.Equal(t, []string{"127.0.0.2"}, lastResolved)
	mu.Unlock()
	require.Greater(t, counter1.Load(), int64(0))
	require.Greater(t, counter2.Load(), int64(0))
}

func appendSimpleMetricWithServiceName(metric pmetric.Metrics, serviceName string, sigName string) {
	metric.ResourceMetrics().EnsureCapacity(1)
	rmetrics := metric.ResourceMetrics().AppendEmpty()
	rmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName)
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(sigName)
}

func benchConsumeMetrics(b *testing.B, endpointsCount int, metricsCount int) {
	sink := new(consumertest.MetricsSink)
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(sink.ConsumeMetrics), nil
	}

	endpoints := []string{}
	for i := 0; i < endpointsCount; i++ {
		endpoints = append(endpoints, fmt.Sprintf("endpoint-%d", i))
	}

	config := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: endpoints},
		},
	}

	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), config, componentFactory)
	require.NotNil(b, lb)
	require.NoError(b, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), config)
	require.NotNil(b, p)
	require.NoError(b, err)

	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(b, err)

	metric1 := pmetric.NewMetrics()
	metric2 := pmetric.NewMetrics()
	for i := 0; i < endpointsCount; i++ {
		for j := 0; j < metricsCount/endpointsCount; j++ {
			appendSimpleMetricWithServiceName(metric2, fmt.Sprintf("service-%d", i), fmt.Sprintf("sig-%d", i))
		}
	}
	simpleMetricsWithServiceName()
	md := mergeMetrics(metric1, metric2)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = p.ConsumeMetrics(context.Background(), md)
		require.NoError(b, err)
	}

	b.StopTimer()
	err = p.Shutdown(context.Background())
	require.NoError(b, err)
}

func BenchmarkConsumeMetrics_1E100T(b *testing.B) {
	benchConsumeMetrics(b, 1, 100)
}

func BenchmarkConsumeMetrics_1E1000T(b *testing.B) {
	benchConsumeMetrics(b, 1, 1000)
}

func BenchmarkConsumeMetrics_5E100T(b *testing.B) {
	benchConsumeMetrics(b, 5, 100)
}

func BenchmarkConsumeMetrics_5E500T(b *testing.B) {
	benchConsumeMetrics(b, 5, 500)
}

func BenchmarkConsumeMetrics_5E1000T(b *testing.B) {
	benchConsumeMetrics(b, 5, 1000)
}

func BenchmarkConsumeMetrics_10E100T(b *testing.B) {
	benchConsumeMetrics(b, 10, 100)
}

func BenchmarkConsumeMetrics_10E500T(b *testing.B) {
	benchConsumeMetrics(b, 10, 500)
}

func BenchmarkConsumeMetrics_10E1000T(b *testing.B) {
	benchConsumeMetrics(b, 10, 1000)
}

func endpoint2Config() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}},
		},
		RoutingKey: "service",
	}
}

func resourceBasedRoutingConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}},
		},
		RoutingKey: resourceRouteKey,
	}
}

func metricNameBasedRoutingConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}},
		},
		RoutingKey: metricRouteKey,
	}
}

func randomMetrics() pmetric.Metrics {
	v1 := uint64(rand.Intn(256))
	name := strconv.FormatUint(v1, 10)
	metrics := pmetric.NewMetrics()
	appendSimpleMetricWithID(metrics.ResourceMetrics().AppendEmpty(), name)
	return metrics
}

func simpleMetricsWithNoService() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	appendSimpleMetricWithID(metrics.ResourceMetrics().AppendEmpty(), "simple-metric-name")
	return metrics
}

func simpleMetricsWithServiceName() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(1)
	rmetrics := metrics.ResourceMetrics().AppendEmpty()
	rmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName1)
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(signal1Name)
	return metrics
}

func simpleMetricsWithResource() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(1)
	rmetrics := metrics.ResourceMetrics().AppendEmpty()
	rmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName1)
	rmetrics.Resource().Attributes().PutStr(keyAttr1, valueAttr1)
	rmetrics.Resource().Attributes().PutInt(keyAttr2, valueAttr2)
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(signal1Name)
	return metrics
}

func twoServicesWithSameMetricName() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(2)
	rs1 := metrics.ResourceMetrics().AppendEmpty()
	rs1.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName1)
	appendSimpleMetricWithID(rs1, signal1Name)
	rs2 := metrics.ResourceMetrics().AppendEmpty()
	rs2.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName2)
	appendSimpleMetricWithID(rs2, signal1Name)
	return metrics
}

func appendSimpleMetricWithID(dest pmetric.ResourceMetrics, id string) {
	dest.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(id)
}

type mockMetricsExporter struct {
	component.Component
	ConsumeMetricsFn func(ctx context.Context, td pmetric.Metrics) error
	consumeErr       error
}

func newMockMetricsExporter(consumeMetricsFn func(ctx context.Context, td pmetric.Metrics) error) exporter.Metrics {
	return &mockMetricsExporter{
		Component:        mockComponent{},
		ConsumeMetricsFn: consumeMetricsFn,
	}
}

func newNopMockMetricsExporter() exporter.Metrics {
	return &mockMetricsExporter{Component: mockComponent{}}
}

func (e *mockMetricsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *mockMetricsExporter) Shutdown(context.Context) error {
	e.consumeErr = errors.New("exporter is shut down")
	return nil
}

func (e *mockMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if e.ConsumeMetricsFn == nil {
		return e.consumeErr
	}
	return e.ConsumeMetricsFn(ctx, md)
}
