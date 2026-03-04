// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
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
			_, err := newTracesExporter(exportertest.NewNopSettings(metadata.Type), tt.config)

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
				p, _ := newTracesExporter(exportertest.NewNopSettings(metadata.Type), simpleConfig())
				return p
			}(),
			nil,
		},
		{
			"error",
			func() *traceExporterImp {
				ts, tb := getTelemetryAssets(t)
				lb, _ := newLoadBalancer(ts.Logger, simpleConfig(), nil, tb)
				p, _ := newTracesExporter(ts, simpleConfig())

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
			res := p.Start(t.Context(), componenttest.NewNopHost())
			defer func() {
				require.NoError(t, p.Shutdown(t.Context()))
			}()

			// verify
			require.Equal(t, tt.err, res)
		})
	}
}

func TestTracesExporterShutdown(t *testing.T) {
	p, err := newTracesExporter(exportertest.NewNopSettings(metadata.Type), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(t.Context())

	// verify
	assert.NoError(t, res)
}

func TestConsumeTraces(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockTracesExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)
	assert.Equal(t, traceIDRouting, p.routingKey)

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
	res := p.ConsumeTraces(t.Context(), simpleTraces())

	// verify
	assert.NoError(t, res)
}

// This test validates that exporter is can concurrently change the endpoints while consuming traces.
func TestConsumeTraces_ConcurrentResolverChange(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	consumeStarted := make(chan struct{})
	consumeDone := make(chan struct{})

	// imitate a slow exporter
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		te := &mockTracesExporter{Component: mockComponent{}}
		te.ConsumeTracesFn = func(_ context.Context, _ ptrace.Traces) error {
			close(consumeStarted)
			time.Sleep(50 * time.Millisecond)
			return te.consumeErr
		}
		return te, nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)
	assert.Equal(t, traceIDRouting, p.routingKey)

	endpoints := []string{"endpoint-1"}
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	go func() {
		assert.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))
		close(consumeDone)
	}()

	// update endpoint while consuming traces
	<-consumeStarted
	endpoints = []string{"endpoint-2"}
	endpoint, err := lb.res.resolve(t.Context())
	require.NoError(t, err)
	require.Equal(t, endpoints, endpoint)
	<-consumeDone
}

func TestConsumeTracesServiceBased(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockTracesExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, serviceBasedRoutingConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(ts, serviceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)
	assert.Equal(t, svcRouting, p.routingKey)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-2"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1", "endpoint-2"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// test
	res := p.ConsumeTraces(t.Context(), simpleTracesWithServiceName())

	// verify
	assert.NoError(t, res)
}

func TestAttributeBasedRouting(t *testing.T) {
	for _, tc := range []struct {
		name       string
		attributes []string
		batch      ptrace.Traces
		res        map[string]bool
	}{
		{
			name: "service name",
			attributes: []string{
				"service.name",
			},
			batch: simpleTracesWithServiceName(),

			res: map[string]bool{
				"service-name-1": true,
				"service-name-2": true,
				"service-name-3": true,
			},
		},
		{
			name: "span name",
			attributes: []string{
				"span.name",
			},
			batch: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				traces.ResourceSpans().EnsureCapacity(1)

				span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetName("/foo/bar/baz")

				return traces
			}(),
			res: map[string]bool{
				"/foo/bar/baz": true,
			},
		},
		{
			name: "span kind",
			attributes: []string{
				"span.kind",
			},
			batch: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				traces.ResourceSpans().EnsureCapacity(1)

				span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetKind(ptrace.SpanKindClient)

				return traces
			}(),
			res: map[string]bool{
				"Client": true,
			},
		},
		{
			name: "composite; name & span kind",
			attributes: []string{
				"service.name",
				"span.kind",
			},
			batch: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				traces.ResourceSpans().EnsureCapacity(1)

				res := traces.ResourceSpans().AppendEmpty()
				res.Resource().Attributes().PutStr("service.name", "service-name-1")

				span := res.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetKind(ptrace.SpanKindClient)

				return traces
			}(),
			res: map[string]bool{
				"service-name-1Client": true,
			},
		},
		{
			name: "composite, but missing attr",
			attributes: []string{
				"missing.attribute",
				"span.kind",
			},
			batch: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				traces.ResourceSpans().EnsureCapacity(1)

				span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetKind(ptrace.SpanKindServer)

				return traces
			}(),
			res: map[string]bool{
				"Server": true,
			},
		},
		{
			name: "span attribute",
			attributes: []string{
				"http.path",
			},
			batch: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				traces.ResourceSpans().EnsureCapacity(1)

				span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.Attributes().PutStr("http.path", "/foo/bar/baz")

				return traces
			}(),
			res: map[string]bool{
				"/foo/bar/baz": true,
			},
		},
		{
			name: "composite pseudo, resource and span attributes",
			attributes: []string{
				"service.name",
				"span.kind",
				"http.path",
			},
			batch: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				traces.ResourceSpans().EnsureCapacity(1)

				res := traces.ResourceSpans().AppendEmpty()
				res.Resource().Attributes().PutStr("service.name", "service-name-1")

				span := res.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetKind(ptrace.SpanKindClient)
				span.Attributes().PutStr("http.path", "/foo/bar/baz")

				return traces
			}(),
			res: map[string]bool{
				"service-name-1Client/foo/bar/baz": true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := routingIdentifiersFromTraces(tc.batch, attrRouting, tc.attributes)
			assert.NoError(t, err)
			assert.Equal(t, res, tc.res)
		})
	}
}

func TestUnsupportedRoutingKeyInRouting(t *testing.T) {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().EnsureCapacity(1)

	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetKind(ptrace.SpanKindServer)

	_, err := routingIdentifiersFromTraces(traces, 38, []string{})
	assert.Equal(t, "unsupported routing_key: 38", err.Error())
}

func TestServiceBasedRoutingForSameTraceId(t *testing.T) {
	b := pcommon.TraceID([16]byte{1, 2, 3, 4})
	for _, tt := range []struct {
		desc       string
		batch      ptrace.Traces
		routingKey routingKey
		res        map[string]bool
	}{
		{
			"same trace id and different services - service based routing",
			twoServicesWithSameTraceID(),
			svcRouting,
			map[string]bool{"ad-service-1": true, "get-recommendations-7": true},
		},
		{
			"same trace id and different services - trace id routing",
			twoServicesWithSameTraceID(),
			traceIDRouting,

			map[string]bool{string(b[:]): true},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res, err := routingIdentifiersFromTraces(tt.batch, tt.routingKey, []string{})
			assert.NoError(t, err)
			assert.Equal(t, res, tt.res)
		})
	}
}

func TestConsumeTracesExporterNoEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockTracesExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return nil, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// test
	res := p.ConsumeTraces(t.Context(), simpleTraces())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("couldn't find the exporter for the endpoint %q", ""))
}

func TestConsumeTracesUnexpectedExporterType(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(ts, simpleConfig())
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
	res := p.ConsumeTraces(t.Context(), simpleTraces())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("unable to export traces, unexpected exporter type: expected exporter.Traces but got %T", newNopMockExporter()))
}

func TestBatchWithTwoTraces(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.TracesSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockTracesExporter(sink.ConsumeTraces), nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb
	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})

	td := simpleTraces()
	appendSimpleTraceWithID(td.ResourceSpans().AppendEmpty(), [16]byte{2, 3, 4, 5})

	// test
	err = p.ConsumeTraces(t.Context(), td)

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.AllTraces(), 1)
	assert.Equal(t, 2, sink.AllTraces()[0].SpanCount())
}

func TestNoTracesInBatch(t *testing.T) {
	for _, tt := range []struct {
		desc         string
		batch        ptrace.Traces
		routingKey   routingKey
		routingAttrs []string
		err          error
	}{
		{
			"no resource spans",
			ptrace.NewTraces(),
			traceIDRouting,
			[]string{},
			errors.New("empty resource spans"),
		},
		{
			"no instrumentation library spans",
			func() ptrace.Traces {
				batch := ptrace.NewTraces()
				batch.ResourceSpans().AppendEmpty()
				return batch
			}(),
			traceIDRouting,
			[]string{},
			errors.New("empty scope spans"),
		},
		{
			"no spans",
			func() ptrace.Traces {
				batch := ptrace.NewTraces()
				batch.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
				return batch
			}(),
			svcRouting,
			[]string{},
			errors.New("empty spans"),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res, err := routingIdentifiersFromTraces(tt.batch, tt.routingKey, tt.routingAttrs)
			assert.Equal(t, err, tt.err)
			assert.Equal(t, res, map[string]bool(nil))
		})
	}
}

func TestRollingUpdatesWhenConsumeTraces(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	// this test is based on the discussion in the following issue for this exporter:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	// prepare

	// simulate rolling updates, the dns resolver should resolve in the following order
	// ["127.0.0.1"] -> ["127.0.0.1", "127.0.0.2"] -> ["127.0.0.2"]
	res, err := newDNSResolver(ts.Logger, "service-1", "", 5*time.Second, 1*time.Second, tb)
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
	res.resInterval = 100 * time.Millisecond

	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{Hostname: "service-1", Port: ""}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockTracesExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newTracesExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = res
	p.loadBalancer = lb

	counter1 := &atomic.Int64{}
	counter2 := &atomic.Int64{}
	id1 := "127.0.0.1:4317"
	id2 := "127.0.0.2:4317"
	unreachableCh := make(chan struct{})
	defaultExporters := map[string]*wrappedExporter{
		id1: newWrappedExporter(newMockTracesExporter(func(_ context.Context, _ ptrace.Traces) error {
			counter1.Add(1)
			counter.Add(1)
			// simulate an unreachable backend
			<-unreachableCh
			return nil
		}), id1),
		id2: newWrappedExporter(newMockTracesExporter(func(_ context.Context, _ ptrace.Traces) error {
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
					assert.NoError(t, p.ConsumeTraces(ctx, randomTraces()))
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
	}, 1*time.Second, 100*time.Millisecond)
	cancel()
	<-consumeCh

	// verify
	mu.Lock()
	require.Equal(t, []string{"127.0.0.2"}, lastResolved)
	mu.Unlock()

	close(unreachableCh)
	waitWG.Wait()
}

func benchConsumeTraces(b *testing.B, endpointsCount, tracesCount int) {
	ts, tb := getTelemetryAssets(b)
	sink := new(consumertest.TracesSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockTracesExporter(sink.ConsumeTraces), nil
	}

	endpoints := []string{}
	for i := range endpointsCount {
		endpoints = append(endpoints, fmt.Sprintf("endpoint-%d", i))
	}

	config := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: endpoints}),
		},
	}

	lb, err := newLoadBalancer(ts.Logger, config, componentFactory, tb)
	require.NotNil(b, lb)
	require.NoError(b, err)

	p, err := newTracesExporter(exportertest.NewNopSettings(metadata.Type), config)
	require.NotNil(b, p)
	require.NoError(b, err)

	p.loadBalancer = lb

	err = p.Start(b.Context(), componenttest.NewNopHost())
	require.NoError(b, err)

	trace1 := ptrace.NewTraces()
	trace2 := ptrace.NewTraces()
	for i := range endpointsCount {
		for j := 0; j < tracesCount/endpointsCount; j++ {
			appendSimpleTraceWithID(trace2.ResourceSpans().AppendEmpty(), [16]byte{1, 2, 6, byte(i)})
		}
	}
	td := mergeTraces(trace1, trace2)

	for b.Loop() {
		err = p.ConsumeTraces(b.Context(), td)
		require.NoError(b, err)
	}

	b.StopTimer()
	err = p.Shutdown(b.Context())
	require.NoError(b, err)
}

func BenchmarkConsumeTraces_1E100T(b *testing.B) {
	benchConsumeTraces(b, 1, 100)
}

func BenchmarkConsumeTraces_1E1000T(b *testing.B) {
	benchConsumeTraces(b, 1, 1000)
}

func BenchmarkConsumeTraces_5E100T(b *testing.B) {
	benchConsumeTraces(b, 5, 100)
}

func BenchmarkConsumeTraces_5E500T(b *testing.B) {
	benchConsumeTraces(b, 5, 500)
}

func BenchmarkConsumeTraces_5E1000T(b *testing.B) {
	benchConsumeTraces(b, 5, 1000)
}

func BenchmarkConsumeTraces_10E100T(b *testing.B) {
	benchConsumeTraces(b, 10, 100)
}

func BenchmarkConsumeTraces_10E500T(b *testing.B) {
	benchConsumeTraces(b, 10, 500)
}

func BenchmarkConsumeTraces_10E1000T(b *testing.B) {
	benchConsumeTraces(b, 10, 1000)
}

func randomTraces() ptrace.Traces {
	v1 := uint8(rand.IntN(256))
	v2 := uint8(rand.IntN(256))
	v3 := uint8(rand.IntN(256))
	v4 := uint8(rand.IntN(256))
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
	rspans.Resource().Attributes().PutStr("service.name", "service-name-1")
	rspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})

	bspans := traces.ResourceSpans().AppendEmpty()
	bspans.Resource().Attributes().PutStr("service.name", "service-name-2")
	bspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})

	aspans := traces.ResourceSpans().AppendEmpty()
	aspans.Resource().Attributes().PutStr("service.name", "service-name-3")
	aspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 5})

	return traces
}

func twoServicesWithSameTraceID() ptrace.Traces {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().EnsureCapacity(2)
	rs1 := traces.ResourceSpans().AppendEmpty()
	rs1.Resource().Attributes().PutStr("service.name", "ad-service-1")
	appendSimpleTraceWithID(rs1, [16]byte{1, 2, 3, 4})
	rs2 := traces.ResourceSpans().AppendEmpty()
	rs2.Resource().Attributes().PutStr("service.name", "get-recommendations-7")
	appendSimpleTraceWithID(rs2, [16]byte{1, 2, 3, 4})
	return traces
}

func appendSimpleTraceWithID(dest ptrace.ResourceSpans, id pcommon.TraceID) {
	dest.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID(id)
}

func simpleConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}}),
		},
	}
}

func serviceBasedRoutingConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}}),
		},
		RoutingKey: "service",
	}
}

type mockTracesExporter struct {
	component.Component
	ConsumeTracesFn func(ctx context.Context, td ptrace.Traces) error
	consumeErr      error
}

func newMockTracesExporter(consumeTracesFn func(ctx context.Context, td ptrace.Traces) error) exporter.Traces {
	return &mockTracesExporter{
		Component:       mockComponent{},
		ConsumeTracesFn: consumeTracesFn,
	}
}

func newNopMockTracesExporter() exporter.Traces {
	return &mockTracesExporter{Component: mockComponent{}}
}

func (e *mockTracesExporter) Shutdown(context.Context) error {
	e.consumeErr = errors.New("exporter is shut down")
	return nil
}

func (*mockTracesExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *mockTracesExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if e.ConsumeTracesFn == nil {
		return e.consumeErr
	}
	return e.ConsumeTracesFn(ctx, td)
}
