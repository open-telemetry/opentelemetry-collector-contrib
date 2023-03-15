// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"path/filepath"
	"sync"
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
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.9.0"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

func TestNewTracesExporter(t *testing.T) {
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
			_, err := newTracesExporter(exportertest.NewNopCreateSettings(), tt.config, zap.NewNop())

			// verify
			require.Equal(t, tt.err, err)
		})
	}
}

func TestTracesExporterStart(t *testing.T) {
	for _, tt := range []struct {
		desc string
		te   *traceExporterImp
		err  error
	}{
		{
			"ok",
			func() *traceExporterImp {
				p, _ := newTracesExporter(exportertest.NewNopCreateSettings(), simpleConfig(), zap.NewNop())
				return p
			}(),
			nil,
		},
		{
			"error",
			func() *traceExporterImp {
				lb, _ := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), nil)
				p, _ := newTracesExporter(exportertest.NewNopCreateSettings(), simpleConfig(), zap.NewNop())

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

func TestTracesExporterShutdown(t *testing.T) {
	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), simpleConfig(), zap.NewNop())
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(context.Background())

	// verify
	assert.Nil(t, res)
}

func TestConsumeTraces(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockTracesExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), simpleConfig(), zap.NewNop())
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
	res := p.ConsumeTraces(context.Background(), simpleTraces())

	// verify
	assert.Nil(t, res)
}

func TestConsumeTracesServiceBased(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockTracesExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), serviceBasedRoutingConfig(), zap.NewNop())
	require.NotNil(t, p)
	require.NoError(t, err)
	assert.Equal(t, p.resourceKeys, []string{"service.name"})

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
	res := p.ConsumeTraces(context.Background(), simpleTracesWithServiceName())

	// verify
	assert.Nil(t, res)
}

func TestServiceBasedRoutingForSameTraceId(t *testing.T) {
	for _, tt := range []struct {
		desc  string
		batch ptrace.Traces
		res   []routingEntry
		err   error
	}{
		{
			"same trace id and different services - service based routing",
			twoServicesWithSameTraceID(),
			[]routingEntry{
				{
					routingKey: resourceAttrRouting,
					keyValue:   "ad-service-1",
				},
				{
					routingKey: resourceAttrRouting,
					keyValue:   "get-recommendations-7",
				},
			},
			nil,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res, err := splitTracesByResourceAttr(tt.batch, []string{"service.name"})
			for i, r := range res {
				assert.Equal(t, tt.res[i].routingKey, r.routingKey)
				assert.Equal(t, tt.res[i].keyValue, r.keyValue)

				var sn string
				if v, ok := r.trace.ResourceSpans().At(0).Resource().Attributes().Get("service.name"); ok {
					sn = v.Str()
				} else {
					sn = ""
				}
				assert.Equal(t, tt.res[i].keyValue, sn)
			}
			assert.Equal(t, tt.err, err)
		})
	}
}

func TestIdBasedRoutingForSameTraceId(t *testing.T) {
	for _, tt := range []struct {
		desc  string
		batch ptrace.Traces
		res   string
		err   error
	}{
		{
			"same trace id and different services - trace id routing",
			twoServicesWithSameTraceID(),
			"",
			errors.New("routeByTraceId must receive a ptrace.Traces with a single ResourceSpan"),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res, err := routeByTraceId(tt.batch)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.res, res)
		})
	}
}

func TestIdBasedRouting(t *testing.T) {
	sink := new(consumertest.TracesSink)
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newMockTracesExporter(sink.ConsumeTraces), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), simpleConfig(), zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, p)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	lb.addMissingExporters(context.Background(), []string{"endpoint-2"})
	lb.addMissingExporters(context.Background(), []string{"endpoint-3"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1", "endpoint-2", "endpoint-3"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	trace := twoServicesWithSameTraceID()
	appendSimpleTraceWithID(trace.ResourceSpans().At(0), [16]byte{1, 2, 3, 4})
	trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})
	trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty().SetTraceID([16]byte{2, 3, 4, 5})
	res := p.ConsumeTraces(context.Background(), trace)

	// Resulting Trace
	// {
	// 	ptrace.Traces: [
	// 		{
	// 			ResourceSpans: [{
	// 				service.name: "ad-service-1"
	// 				ScopeSpans: [{
	// 					Spans: [{
	// 						TraceID: "1234"
	// 					}]
	// 					Spans: [{
	// 						TraceID: "1234"
	// 					}]
	// 					Spans: [{
	// 						TraceID: "2345"
	// 					}]
	// 				}]
	// 				ScopeSpans: [{
	// 					Spans: [{
	// 						TraceID: "1234"
	// 					}]
	// 				}]
	// 			}]
	// 		},
	// 		{
	// 			ResourceSpans: [{
	// 				service.name: "get-recommendation-7"
	// 				ScopeSpans: [{
	// 					Spans: [{
	// 						TraceID: "1234"
	// 					}]
	// 				}]
	// 			}]
	// 		}
	// 	]
	// }

	// verify
	assert.Nil(t, res)

	// This will be split into four because of the behavior of batchpersignal.SplitTraces
	// The ad-service-1 trace is split into 3
	// - 1 trace containing the two "1234" spans
	// - 1 trace containing the "2345" span
	// - 1 trace containing the span in a different ILS (despite having the same trace ID as the two "1234" spans)
	// - 1 trace containing the span in a different RS (despite having the same trace ID as the thre "1234" spans)
	assert.Len(t, sink.AllTraces(), 4)
}

func TestAttrBasedRouting(t *testing.T) {
	sink := new(consumertest.TracesSink)
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newMockTracesExporter(sink.ConsumeTraces), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), attrBasedRoutingConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), attrBasedRoutingConfig(), zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, p.resourceKeys, []string{"service.name"})

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	lb.addMissingExporters(context.Background(), []string{"endpoint-2"})
	lb.addMissingExporters(context.Background(), []string{"endpoint-3"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1", "endpoint-2", "endpoint-3"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	trace := twoServicesWithSameTraceID()
	rs := trace.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr(conventions.AttributeServiceName, "ad-service-1")
	appendSimpleTraceWithID(rs, [16]byte{1, 2, 3, 4})
	res := p.ConsumeTraces(context.Background(), trace)

	// verify
	assert.Nil(t, res)

	// Verify that the single Trace was split into two based on service name
	// With the two `ad-service-1` RS being grouped into a single trace
	assert.Len(t, sink.AllTraces(), 2)
}

func TestConsumeTracesExporterNoEndpoint(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockTracesExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), simpleConfig(), zap.NewNop())
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
	res := p.ConsumeTraces(context.Background(), simpleTraces())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("couldn't find the exporter for the endpoint %q", ""))
}

func TestConsumeTracesUnexpectedExporterType(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), simpleConfig(), zap.NewNop())
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
	res := p.ConsumeTraces(context.Background(), simpleTraces())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("unable to export traces, unexpected exporter type: expected exporter.Traces but got %T", newNopMockExporter()))
}

func TestBuildExporterConfig(t *testing.T) {
	// prepare
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factories.Exporters[typeStr] = NewFactory()

	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "test-build-exporter-config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	c := cfg.Exporters[component.NewID(typeStr)]
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

func TestBatchWithTwoTraces(t *testing.T) {
	sink := new(consumertest.TracesSink)
	componentFactory := func(ctx context.Context, endpoint string) (component.Component, error) {
		return newMockTracesExporter(sink.ConsumeTraces), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), simpleConfig(), zap.NewNop())
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})

	td := simpleTraces()
	appendSimpleTraceWithID(td.ResourceSpans().AppendEmpty(), [16]byte{2, 3, 4, 5})

	// test
	err = p.ConsumeTraces(context.Background(), td)

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.AllTraces(), 2)
}

func TestNoTracesInBatchTraceIdRouting(t *testing.T) {
	for _, tt := range []struct {
		desc  string
		batch ptrace.Traces
		err   error
	}{
		// Trace ID routing
		{
			"no resource spans",
			ptrace.NewTraces(),
			errors.New("empty resource spans"),
		},
		{
			"no instrumentation library spans",
			func() ptrace.Traces {
				batch := ptrace.NewTraces()
				batch.ResourceSpans().AppendEmpty()
				return batch
			}(),
			errors.New("empty scope spans"),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res, err := routeByTraceId(tt.batch)
			assert.Equal(t, err, tt.err)
			assert.Equal(t, res, "")
		})
	}
}

func TestNoTracesInBatchResourceRouting(t *testing.T) {
	for _, tt := range []struct {
		desc  string
		batch ptrace.Traces
		res   []routingEntry
		err   error
	}{
		// Service / Resource Attribute routing
		{
			"no spans",
			func() ptrace.Traces {
				batch := ptrace.NewTraces()
				batch.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
				return batch
			}(),
			[]routingEntry{
				{
					routingKey: traceIDRouting,
					keyValue:   "",
				},
			},
			nil,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res, err := splitTracesByResourceAttr(tt.batch, []string{"service.name"})
			assert.Equal(t, tt.err, err)
			for i, _ := range res {
				assert.Equal(t, tt.res[i].routingKey, res[i].routingKey)
				assert.Equal(t, tt.res[i].keyValue, res[i].keyValue)
			}
		})
	}
}

func TestRollingUpdatesWhenConsumeTraces(t *testing.T) {
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
	counter := atomic.NewInt64(0)
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
				counter.Inc()
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
		return newNopMockTracesExporter(), nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopCreateSettings(), cfg, componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(exportertest.NewNopCreateSettings(), cfg, zap.NewNop())
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = res
	p.loadBalancer = lb

	counter1 := atomic.NewInt64(0)
	counter2 := atomic.NewInt64(0)
	defaultExporters := map[string]component.Component{
		"127.0.0.1:4317": newMockTracesExporter(func(ctx context.Context, td ptrace.Traces) error {
			counter1.Inc()
			// simulate an unreachable backend
			time.Sleep(10 * time.Second)
			return nil
		},
		),
		"127.0.0.2:4317": newMockTracesExporter(func(ctx context.Context, td ptrace.Traces) error {
			counter2.Inc()
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
				go func() {
					require.NoError(t, p.ConsumeTraces(ctx, randomTraces()))
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

func randomTraces() ptrace.Traces {
	v1 := uint8(rand.Intn(256))
	v2 := uint8(rand.Intn(256))
	v3 := uint8(rand.Intn(256))
	v4 := uint8(rand.Intn(256))
	traces := ptrace.NewTraces()
	appendSimpleTraceWithID(traces.ResourceSpans().AppendEmpty(), [16]byte{v1, v2, v3, v4})
	return traces
}

func simpleTraces() ptrace.Traces {
	traces := ptrace.NewTraces()
	appendSimpleTraceWithID(traces.ResourceSpans().AppendEmpty(), [16]byte{1, 2, 3, 4})
	return traces
}

func simpleTracesWithServiceName() ptrace.Traces {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().EnsureCapacity(1)
	rspans := traces.ResourceSpans().AppendEmpty()
	rspans.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-1")
	rspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})
	return traces
}

func twoServicesWithSameTraceID() ptrace.Traces {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().EnsureCapacity(2)
	rs1 := traces.ResourceSpans().AppendEmpty()
	rs1.Resource().Attributes().PutStr(conventions.AttributeServiceName, "ad-service-1")
	appendSimpleTraceWithID(rs1, [16]byte{1, 2, 3, 4})
	rs2 := traces.ResourceSpans().AppendEmpty()
	rs2.Resource().Attributes().PutStr(conventions.AttributeServiceName, "get-recommendations-7")
	appendSimpleTraceWithID(rs2, [16]byte{1, 2, 3, 4})
	return traces
}

func appendSimpleTraceWithID(dest ptrace.ResourceSpans, id pcommon.TraceID) {
	dest.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID(id)
}

func simpleConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
	}
}

func serviceBasedRoutingConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
		RoutingKey: "service",
	}
}

func attrBasedRoutingConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2", "endpoint-3"}},
		},
		RoutingKey:   "resource",
		ResourceKeys: []string{"service.name"},
	}
}

type mockTracesExporter struct {
	component.Component
	ConsumeTracesFn func(ctx context.Context, td ptrace.Traces) error
}

func newMockTracesExporter(consumeTracesFn func(ctx context.Context, td ptrace.Traces) error) exporter.Traces {
	return &mockTracesExporter{
		Component:       mockComponent{},
		ConsumeTracesFn: consumeTracesFn,
	}
}

func newNopMockTracesExporter() exporter.Traces {
	return &mockTracesExporter{
		Component: mockComponent{},
		ConsumeTracesFn: func(ctx context.Context, td ptrace.Traces) error {
			return nil
		},
	}
}

func (e *mockTracesExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *mockTracesExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if e.ConsumeTracesFn == nil {
		return nil
	}
	return e.ConsumeTracesFn(ctx, td)
}
