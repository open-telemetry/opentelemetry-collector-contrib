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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
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
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.9.0"
	"go.uber.org/zap"
)

const (
	signalName1  = "metric-name-1"
	signalName2  = "metric-name-2"
	serviceName1 = "service-name-1"
	serviceName2 = "service-name-2"
)

func TestNewMetricsExporter(t *testing.T) {
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
				p, _ := newMetricsExporter(exportertest.NewNopCreateSettings(), simpleConfig())
				return p
			}(),
			nil,
		},
		{
			"error",
			func() *metricExporterImp {
				lb, _ := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), nil)
				p, _ := newMetricsExporter(exportertest.NewNopCreateSettings(), simpleConfig())

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
	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), simpleConfig())
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
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)
	assert.Equal(t, p.routingKey, traceIDRouting)

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
	res := p.ConsumeMetrics(context.Background(), simpleMetrics())

	// verify
	assert.Nil(t, res)
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

func TestServiceBasedRoutingForSameMetricName(t *testing.T) {

	for _, tt := range []struct {
		desc       string
		batch      pmetric.Metrics
		routingKey routingKey
		res        map[string]bool
	}{
		{
			"same trace id and different services - service based routing",
			twoServicesWithSameMetricName(),
			svcRouting,
			map[string]bool{serviceName1: true, serviceName2: true},
		},
		{
			"same trace id and different services - trace id routing",
			twoServicesWithSameMetricName(),
			traceIDRouting,
			map[string]bool{string(signalName1[:]): true},
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
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), simpleConfig())
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
	res := p.ConsumeMetrics(context.Background(), simpleMetrics())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("couldn't find the exporter for the endpoint %q", ""))
}

func TestConsumeMetricsUnexpectedExporterType(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

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
	res := p.ConsumeMetrics(context.Background(), simpleMetrics())

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
	grpcSettings := defaultCfg.GRPCClientSettings
	grpcSettings.Endpoint = "the-endpoint"
	assert.Equal(t, grpcSettings, exporterCfg.GRPCClientSettings)

	assert.Equal(t, defaultCfg.TimeoutSettings, exporterCfg.TimeoutSettings)
	assert.Equal(t, defaultCfg.QueueSettings, exporterCfg.QueueSettings)
	assert.Equal(t, defaultCfg.RetrySettings, exporterCfg.RetrySettings)
}

func TestBatchWithTwoMetrics(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(sink.ConsumeMetrics), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(exportertest.NewNopCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})

	td := simpleMetrics()
	appendSimpleMetricWithID(td.ResourceMetrics().AppendEmpty(), "metric-name")

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
			traceIDRouting,
			errors.New("empty resource metrics"),
		},
		{
			"no instrumentation library metrics",
			func() pmetric.Metrics {
				batch := pmetric.NewMetrics()
				batch.ResourceMetrics().AppendEmpty()
				return batch
			}(),
			traceIDRouting,
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
	defaultExporters := map[string]component.Component{
		"127.0.0.1:4317": newMockMetricsExporter(func(ctx context.Context, td pmetric.Metrics) error {
			counter1.Add(1)
			// simulate an unreachable backend
			time.Sleep(10 * time.Second)
			return nil
		},
		),
		"127.0.0.2:4317": newMockMetricsExporter(func(ctx context.Context, td pmetric.Metrics) error {
			counter2.Add(1)
			return nil
		},
		),
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

func randomMetrics() pmetric.Metrics {
	v1 := uint64(rand.Intn(256))
	name := strconv.FormatUint(v1, 10)
	metrics := pmetric.NewMetrics()
	appendSimpleMetricWithID(metrics.ResourceMetrics().AppendEmpty(), name)
	return metrics
}

func simpleMetrics() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	appendSimpleMetricWithID(metrics.ResourceMetrics().AppendEmpty(), "simple-metric-name")
	return metrics
}

func simpleMetricsWithServiceName() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(1)
	rmetrics := metrics.ResourceMetrics().AppendEmpty()
	rmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-1")
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(string("name"))
	return metrics
}

func twoServicesWithSameMetricName() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(2)
	rs1 := metrics.ResourceMetrics().AppendEmpty()
	rs1.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName1)
	appendSimpleMetricWithID(rs1, signalName1)
	rs2 := metrics.ResourceMetrics().AppendEmpty()
	rs2.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName2)
	appendSimpleMetricWithID(rs2, signalName1)
	return metrics
}

func appendSimpleMetricWithID(dest pmetric.ResourceMetrics, id string) {
	dest.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(id)
}

// func simpleConfig() *Config {
// 	return &Config{
// 		Resolver: ResolverSettings{
// 			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
// 		},
// 	}
// }

// func serviceBasedRoutingConfig() *Config {
// 	return &Config{
// 		Resolver: ResolverSettings{
// 			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
// 		},
// 		RoutingKey: "service",
// 	}
// }

type mockMetricsExporter struct {
	component.Component
	ConsumeMetricsFn func(ctx context.Context, td pmetric.Metrics) error
}

func newMockMetricsExporter(consumeMetricsFn func(ctx context.Context, td pmetric.Metrics) error) exporter.Metrics {
	return &mockMetricsExporter{
		Component:        mockComponent{},
		ConsumeMetricsFn: consumeMetricsFn,
	}
}

func newNopMockMetricsExporter() exporter.Metrics {
	return &mockMetricsExporter{
		Component: mockComponent{},
		ConsumeMetricsFn: func(ctx context.Context, md pmetric.Metrics) error {
			return nil
		},
	}
}

func (e *mockMetricsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *mockMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if e.ConsumeMetricsFn == nil {
		return nil
	}
	return e.ConsumeMetricsFn(ctx, md)
}
